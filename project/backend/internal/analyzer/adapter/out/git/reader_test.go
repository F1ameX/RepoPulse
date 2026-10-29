package git

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type rewriteTransport struct {
	base      http.RoundTripper
	target    *url.URL
	interrupt bool
}

func (t rewriteTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	clone := r.Clone(r.Context())
	u := *r.URL
	u.Scheme, u.Host = t.target.Scheme, t.target.Host
	clone.URL = &u
	clone.Host = u.Host
	response, err := t.base.RoundTrip(clone)
	if err == nil && t.interrupt && strings.HasSuffix(u.Path, "/git-upload-pack") {
		response.Body = &shortBody{ReadCloser: response.Body, remaining: 64}
		response.ContentLength = -1
	}
	return response, err
}

type shortBody struct {
	io.ReadCloser
	remaining int
}

func (s *shortBody) Read(p []byte) (int, error) {
	if s.remaining <= 0 {
		return 0, io.ErrUnexpectedEOF
	}
	if len(p) > s.remaining {
		p = p[:s.remaining]
	}
	n, e := s.ReadCloser.Read(p)
	s.remaining -= n
	return n, e
}
func command(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.test", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.test")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture git %s: %v", args[0], err)
	}
	return strings.TrimSpace(string(output))
}
func fixture(t *testing.T) (domain.RepositoryMetadata, *httptest.Server, string) {
	t.Helper()
	root := t.TempDir()
	work := filepath.Join(root, "work")
	if err := os.Mkdir(work, 0700); err != nil {
		t.Fatal(err)
	}
	command(t, work, "init", "-b", "main")
	for path, body := range map[string]string{"README.md": "# real fixture", "run.sh": "#!/bin/sh\ntouch NEVER_EXECUTE_REPOSITORY_CODE\n", ".gitattributes": "*.md filter=evil diff=evil"} {
		if err := os.WriteFile(filepath.Join(work, path), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("/must-not-be-followed", filepath.Join(work, "external-link")); err != nil {
		t.Fatal(err)
	}
	command(t, work, "add", ".")
	command(t, work, "commit", "-m", "first")
	first := command(t, work, "rev-parse", "HEAD")
	command(t, work, "branch", "other")
	command(t, work, "tag", "-a", "v1", "-m", "tag")
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("# second version"), 0600); err != nil {
		t.Fatal(err)
	}
	command(t, work, "add", "README.md")
	command(t, work, "commit", "-m", "second")
	sha := command(t, work, "rev-parse", "HEAD")
	command(t, work, "checkout", "other")
	if err := os.WriteFile(filepath.Join(work, "only-other-branch"), []byte("unmerged"), 0600); err != nil {
		t.Fatal(err)
	}
	command(t, work, "add", ".")
	command(t, work, "commit", "-m", "unmerged branch")
	command(t, work, "checkout", "main")
	// An inherited user configuration would execute a filter/fsmonitor. The reader
	// starts from a new bare repository and ignores these settings and templates.
	poison := filepath.Join(root, "poison-config")
	if err := os.WriteFile(poison, []byte("[core]\n fsmonitor = touch "+filepath.Join(root, "EXECUTED")+"\n[filter \"evil\"]\n smudge = touch "+filepath.Join(root, "EXECUTED")+"\n required = true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "owner"), 0700); err != nil {
		t.Fatal(err)
	}
	command(t, root, "clone", "--bare", work, filepath.Join(root, "owner", "repo.git"))
	binary, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	handler := &cgi.Handler{Path: binary, Args: []string{"http-backend"}, Dir: root, Env: []string{"GIT_PROJECT_ROOT=" + root, "GIT_HTTP_EXPORT_ALL=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull}}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("GIT_CONFIG_GLOBAL", poison)
	branch := "main"
	return domain.RepositoryMetadata{Platform: "github", GitHubID: "42", CanonicalURL: "https://github.com/owner/repo", Owner: "owner", Name: "repo", DefaultBranch: &branch, CommitSHA: &sha}, server, first
}
func readerFor(t *testing.T, server *httptest.Server, limits Limits, interrupt bool) *Reader {
	t.Helper()
	reader, err := NewReader(t.TempDir(), limits)
	if err != nil {
		t.Fatal(err)
	}
	target, _ := url.Parse(server.URL)
	reader.client = &http.Client{Transport: rewriteTransport{base: http.DefaultTransport, target: target, interrupt: interrupt}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return reader
}
func hasReason(data domain.GitData, reason string) bool {
	for _, source := range data.Sources {
		if source.Reason != nil && source.Reason.Code == reason {
			return true
		}
	}
	return false
}
func TestRealGitIsReadWithoutCheckout(t *testing.T) {
	metadata, server, old := fixture(t)
	reader := readerFor(t, server, DefaultLimits(), false)

	session, err := reader.Open(t.Context(), metadata)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	data := session.Data()
	if _, err := os.Stat(filepath.Join(server.Config.Handler.(*cgi.Handler).Dir, "EXECUTED")); !os.IsNotExist(err) {
		t.Fatal("user Git config was executed")
	}
	if len(data.Refs) != 3 || len(data.Commits) != 3 || len(data.Files) != 4 {
		t.Fatalf("unexpected structural counts refs=%d commits=%d files=%d sources=%+v", len(data.Refs), len(data.Commits), len(data.Files), data.Sources)
	}
	foundOld, foundSymlink, foundUnmerged := false, false, false
	for _, commit := range data.Commits {
		if commit.Main == nil {
			t.Fatal("main membership is unknown after a complete traversal")
		}
		foundOld = foundOld || commit.SHA == old
		foundUnmerged = foundUnmerged || !*commit.Main
	}
	for _, file := range data.Files {
		if file.Path == "README.md" {
			body, err := session.ReadBlob(t.Context(), file.ObjectSHA)
			if err != nil || string(body) != "# second version" {
				t.Fatal("pinned blob was not read")
			}
		}
		if file.Path == "external-link" {
			foundSymlink = file.Mode == "120000"
			body, err := session.ReadBlob(t.Context(), file.ObjectSHA)
			if err != nil || string(body) != "/must-not-be-followed" {
				t.Fatal("symlink should be read as a blob, never followed")
			}
		}
	}
	if !foundOld || !foundSymlink || !foundUnmerged {
		t.Fatal("old tagged/branch history or symlink missing")
	}
	for _, source := range data.Sources {
		if source.Status != "complete" {
			t.Fatalf("unexpected incomplete source: %+v", source)
		}
	}
	encoded, _ := json.Marshal(data)
	if bytes.Contains(encoded, []byte("touch NEVER_EXECUTE")) {
		t.Fatal("raw source leaked into structural facts")
	}
	// Processor mutations must not upgrade collector completeness or alter the
	// pinned revision supplied later to the publisher.
	*data.Repository.CommitSHA = strings.Repeat("b", 40)
	*data.Sources[0].Sample.Collected = 0
	previousMain := *session.Data().Commits[0].Main
	*data.Commits[0].Main = !previousMain
	if *session.Data().Commits[0].Main != previousMain {
		t.Fatal("main membership is mutable through the public view")
	}
	if *session.Data().Repository.CommitSHA != *metadata.CommitSHA || *session.Data().Sources[0].Sample.Collected == 0 {
		t.Fatal("collector data is mutable through its public view")
	}
	local := session.(*Session)
	if _, err := os.Stat(filepath.Join(local.root, "README.md")); !os.IsNotExist(err) {
		t.Fatal("checkout was created")
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(local.root); !os.IsNotExist(err) {
		t.Fatal("temporary Git was not removed")
	}
}

func TestBlobBudgetAndClosedSession(t *testing.T) {
	metadata, server, _ := fixture(t)
	session, err := readerFor(t, server, DefaultLimits(), false).Open(t.Context(), metadata)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	var blob string
	for _, file := range session.Data().Files {
		if file.Path == "README.md" {
			blob = file.ObjectSHA
		}
	}
	session.(*Session).blobBytes = 200 << 20
	if _, err := session.ReadBlob(t.Context(), blob); ReasonCode(err) != "GIT_BLOB_LIMIT" {
		t.Fatal("blob budget ignored")
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := session.ReadBlob(t.Context(), blob); ReasonCode(err) != "GIT_BLOB_UNAVAILABLE" {
		t.Fatal("closed session still reads Git objects")
	}
}

// This optional network test is separate from the deterministic control-repo
// tests. Run with REPOPULSE_GITHUB_SMOKE=1 to check GitHub's real smart HTTP.
func TestPublicGitHubSmoke(t *testing.T) {
	if os.Getenv("REPOPULSE_GITHUB_SMOKE") != "1" {
		t.Skip("set REPOPULSE_GITHUB_SMOKE=1 for a real GitHub request")
	}
	reader, err := NewReader(t.TempDir(), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	metadata := domain.RepositoryMetadata{Platform: "github", GitHubID: "1296269", CanonicalURL: "https://github.com/octocat/hello-world", Owner: "octocat", Name: "hello-world"}
	session, err := reader.Open(t.Context(), metadata)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	data := session.Data()
	if data.Repository.CommitSHA == nil || len(data.Files) == 0 || len(data.Commits) == 0 {
		t.Fatal("public GitHub objects not collected")
	}
	for _, source := range data.Sources {
		if source.Status != "complete" {
			t.Fatalf("public GitHub collection incomplete: scope=%s reason=%+v", source.Scope, source.Reason)
		}
	}
	t.Logf("GitHub: refs=%d paths=%d commits=%d received=%d", len(data.Refs), len(data.Files), len(data.Commits), data.ReceivedBytes)
}
func TestGitLimitsAndInterruptedTransferAreIncomplete(t *testing.T) {
	metadata, server, _ := fixture(t)
	for _, tc := range []struct {
		name        string
		limits      Limits
		interrupted bool
		reason      string
	}{
		{"paths", Limits{Timeout: time.Minute, Bytes: 100 << 20, Paths: 1, Commits: 10000}, false, "GIT_PATH_LIMIT"},
		{"commits", Limits{Timeout: time.Minute, Bytes: 100 << 20, Paths: 50000, Commits: 1}, false, "GIT_COMMIT_LIMIT"},
		{"bytes", Limits{Timeout: time.Minute, Bytes: 128, Paths: 50000, Commits: 10000}, false, "GIT_SIZE_LIMIT"},
		{"interrupted", DefaultLimits(), true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := readerFor(t, server, tc.limits, tc.interrupted)
			session, err := reader.Open(t.Context(), metadata)
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			data := session.Data()
			if data.ReceivedBytes > tc.limits.Bytes {
				t.Fatal("received-byte budget exceeded")
			}
			incomplete := false
			for _, source := range data.Sources {
				if source.Status != "complete" {
					incomplete = true
					if source.Reason == nil || source.Sample.Total != nil {
						t.Fatal("missing limitation or fabricated total")
					}
				}
			}
			if !incomplete {
				t.Fatal("limited transfer claimed full coverage")
			}
			if tc.reason != "" && !hasReason(data, tc.reason) {
				t.Fatalf("missing reason %s; sources=%+v", tc.reason, data.Sources)
			}
		})
	}
}
func TestGitTimeoutAndCancellationCleanTemporaryData(t *testing.T) {
	metadata, _, _ := fixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	limits := DefaultLimits()
	limits.Timeout = 100 * time.Millisecond
	reader := readerFor(t, server, limits, false)
	session, err := reader.Open(t.Context(), metadata)
	if err == nil {
		if !hasReason(session.Data(), "GIT_TIME_LIMIT") {
			t.Fatal("missing time-limit reason")
		}
		if err = session.Close(); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(reader.tempRoot)
	if err != nil || len(entries) != 0 {
		t.Fatal("temporary Git data survived timeout")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if session, err := reader.Open(ctx, metadata); err == nil {
		session.Close()
		t.Fatal("cancelled acquisition succeeded")
	}
	entries, _ = os.ReadDir(reader.tempRoot)
	if len(entries) != 0 {
		t.Fatal("temporary Git data survived cancellation")
	}
}
