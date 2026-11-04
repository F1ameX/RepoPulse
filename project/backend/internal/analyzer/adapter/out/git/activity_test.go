package git

import (
	"net/http/cgi"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

func TestHistoryDeduplicatesAndRejectsInvalidCommitterDate(t *testing.T) {
	a, b := strings.Repeat("a", 40), strings.Repeat("b", 40)
	output := []byte("946684800 " + a + "\n946684800 " + a + "\n253402300800 " + b + "\n")
	walk := parseHistory(t.Context(), output, 2)
	if len(walk.commits) != 1 || walk.commits[0].SHA != a || walk.code != "GIT_HISTORY_INVALID" {
		t.Fatalf("expected one dated commit and an incomplete history: %#v", walk)
	}
}

func TestActivityUsesPinnedHistoryAndCommitterWindow(t *testing.T) {
	metadata, server, _ := fixture(t)
	root := server.Config.Handler.(*cgi.Handler).Dir
	work := filepath.Join(root, "work")
	tree := command(t, work, "rev-parse", "HEAD^{tree}")
	until := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	from := until.Add(-140 * 24 * time.Hour)
	window := domain.Window{From: from, Until: until}
	commit := func(at time.Time, parents ...string) string {
		t.Helper()
		t.Setenv("GIT_AUTHOR_DATE", "2001-01-01T00:00:00Z")
		t.Setenv("GIT_COMMITTER_DATE", at.In(time.FixedZone("UTC+07", 7*60*60)).Format(time.RFC3339))
		args := []string{"commit-tree", tree, "-m", "activity fixture"}
		for _, parent := range parents {
			args = append(args, "-p", parent)
		}
		return command(t, work, args...)
	}
	old := commit(from.Add(-time.Second))
	firstInWindow := commit(from, old)
	merged := commit(from.Add(time.Hour), old)
	merge := commit(until.Add(-time.Second), firstInWindow, merged)
	pinned := commit(until, merge)
	unmerged := commit(from.Add(2*time.Hour), old)
	command(t, work, "update-ref", "refs/heads/main", pinned)
	command(t, work, "update-ref", "refs/heads/other", unmerged)
	bare := "--git-dir=" + filepath.Join(root, "owner", "repo.git")
	command(t, root, bare, "fetch", "--no-tags", work, "+refs/heads/*:refs/heads/*")
	command(t, root, bare, "update-ref", "refs/tags/v1", firstInWindow)
	metadata.CommitSHA = &pinned

	limits := DefaultLimits()
	limits.Commits = 5 // Fits pinned history, but not all advertised branches.
	session, err := readerFor(t, server, limits, false).Open(t.Context(), metadata)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	data := session.Data()
	activity := data.Activity(window)
	if activity.Source.Status != "complete" || activity.Source.Fingerprint == nil || !hasReason(data, "GIT_COMMIT_LIMIT") {
		t.Fatalf("activity should be complete independently of all-refs history: %#v", activity.Source)
	}
	want := map[string]time.Time{firstInWindow: from, merged: from.Add(time.Hour), merge: until.Add(-time.Second)}
	if len(activity.Commits) != len(want) {
		t.Fatalf("wrong activity window: %#v", activity.Commits)
	}
	for _, observed := range activity.Commits {
		at, ok := want[observed.SHA]
		if !ok || observed.CommittedAt != at {
			t.Fatalf("wrong commit or committer time: %#v", observed)
		}
		delete(want, observed.SHA)
	}
	if activity.LastCommit == nil || activity.LastCommit.SHA != pinned || activity.FirstCommit == nil || activity.FirstCommit.SHA != old {
		t.Fatalf("wrong extrema through the assessment: %#v", activity)
	}

	limits.Commits = 3
	limited, err := readerFor(t, server, limits, false).Open(t.Context(), metadata)
	if err != nil {
		t.Fatal(err)
	}
	defer limited.Close()
	activity = limited.Data().Activity(window)
	if activity.Source.Status != "partial" || activity.Source.Reason == nil || activity.Source.Reason.Code != "GIT_COMMIT_LIMIT" ||
		activity.Source.Sample.Total != nil || activity.Source.Fingerprint != nil || activity.FirstCommit != nil ||
		activity.LastCommit != nil || activity.HistoryAgeDays != nil {
		t.Fatalf("truncated history claimed confirmed activity: %#v", activity)
	}
}
