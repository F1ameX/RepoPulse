package git

import (
	"context"
	"net/http/cgi"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

func TestLimitedMergeHistoryDoesNotInventNonMainCommits(t *testing.T) {
	metadata, server, _ := fixture(t)
	root := server.Config.Handler.(*cgi.Handler).Dir
	work := filepath.Join(root, "work")
	tree := command(t, work, "rev-parse", "HEAD^{tree}")
	commit := func(at string, parents ...string) string {
		t.Setenv("GIT_AUTHOR_DATE", at)
		t.Setenv("GIT_COMMITTER_DATE", at)
		args := []string{"commit-tree", tree, "-m", "merge fixture"}
		for _, parent := range parents {
			args = append(args, "-p", parent)
		}
		return command(t, work, args...)
	}
	a := commit("2001-01-01T00:00:00Z")
	b := commit("2001-01-05T00:00:00Z", a)
	c := commit("2001-01-06T00:00:00Z", b)
	d := commit("2001-01-02T00:00:00Z", a)
	e := commit("2001-01-03T00:00:00Z", d)
	merge := commit("2001-01-07T00:00:00Z", c, e)
	command(t, work, "update-ref", "refs/heads/main", merge)
	command(t, work, "update-ref", "refs/heads/other", a)
	bare := "--git-dir=" + filepath.Join(root, "owner", "repo.git")
	command(t, root, bare, "fetch", "--no-tags", work, "+refs/heads/*:refs/heads/*")
	command(t, root, bare, "update-ref", "refs/tags/v1", a)
	metadata.CommitSHA = &merge

	limits := DefaultLimits()
	limits.Commits = 3
	session, err := readerFor(t, server, limits, false).Open(t.Context(), metadata)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	data := session.Data()
	if len(data.Commits) != limits.Commits || !hasReason(data, "GIT_COMMIT_LIMIT") {
		t.Fatal("fixture did not reach the history limit")
	}
	unknown := 0
	for _, item := range data.Commits {
		// Every advertised ref is an ancestor of the merge. Check reachability
		// using unrestricted Git only for this small, trusted test fixture.
		command(t, work, "merge-base", "--is-ancestor", item.SHA, merge)
		if item.Main != nil && !*item.Main {
			t.Fatalf("ancestor %s was classified outside main in a limited walk", item.SHA)
		}
		if item.Main == nil {
			unknown++
		}
	}
	if unknown == 0 {
		t.Fatal("fixture did not exercise unknown membership outside the main prefix")
	}
}

func TestGitTimeoutAlsoBoundsLocalObjectCommands(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("fixture uses the Unix process-group cancellation path")
	}
	sha, branch := strings.Repeat("a", 40), "main"
	metadata := domain.RepositoryMetadata{Platform: "github", GitHubID: "42", CanonicalURL: "https://github.com/owner/repo", Owner: "owner", Name: "repo", DefaultBranch: &branch, CommitSHA: &sha}
	for _, slowCommand := range []string{"cat-file", "ls-tree", "rev-list"} {
		t.Run(slowCommand, func(t *testing.T) {
			limits := DefaultLimits()
			limits.Timeout = 3 * time.Second
			reader, err := NewReader(t.TempDir(), limits)
			if err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(t.TempDir(), "local-command-started")
			quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
			// This trusted stub makes setup independent of network/disk speed.
			// Real smart HTTP/object traversal is covered by the other tests;
			// this test checks propagation of one deadline into each process.
			wrapper := `#!/bin/sh
operation=
timestamp=
for arg do
case "$arg" in
init|ls-remote|fetch|cat-file|ls-tree|rev-list) operation="$arg" ;;
--timestamp) timestamp=1 ;;
esac
done
if [ "$operation" = ` + quote(slowCommand) + ` ]; then
: > ` + quote(marker) + `
sleep 5
fi
sha=` + quote(sha) + `
case "$operation" in
ls-remote) printf 'ref: refs/heads/main\tHEAD\n%s\tHEAD\n%s\trefs/heads/main\n' "$sha" "$sha" ;;
cat-file) printf 'commit\n' ;;
ls-tree) printf '100644 blob %s 1\tREADME.md\000' "$sha" ;;
rev-list)
if [ -n "$timestamp" ]; then printf '946684800 %s\n' "$sha"; else printf '%s\n' "$sha"; fi ;;
esac
`
			reader.binary = filepath.Join(t.TempDir(), "slow-git")
			if err := os.WriteFile(reader.binary, []byte(wrapper), 0700); err != nil {
				t.Fatal(err)
			}
			// Warm up the test executable outside the measured collection phase.
			if err := exec.CommandContext(t.Context(), reader.binary, "--version").Run(); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			started := time.Now()
			session, err := reader.Open(ctx, metadata)
			elapsed := time.Since(started)
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			if _, err := os.Stat(marker); err != nil {
				t.Fatal("Git timeout expired before the local command under test")
			}
			if elapsed >= limits.Timeout+800*time.Millisecond {
				t.Fatalf("local command escaped the %s Git budget: %s", limits.Timeout, elapsed)
			}
			data := session.Data()
			for _, source := range data.Sources {
				if source.Scope == "history" || source.Scope == "tree" && slowCommand != "rev-list" {
					if source.Status == "complete" || source.Reason == nil || source.Reason.Code != "GIT_TIME_LIMIT" || source.Sample.Total != nil || source.Fingerprint != nil {
						t.Fatalf("expired Git phase claimed completeness: %+v", source)
					}
				}
			}
			if err := session.Close(); err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(reader.tempRoot)
			if err != nil || len(entries) != 0 {
				t.Fatal("temporary Git data survived local timeout")
			}
		})
	}
}
