package domain

import (
	"testing"
	"time"
)

func TestActivityDistinguishesEmptyInactiveAndMissingHistory(t *testing.T) {
	until := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	window := Window{From: until.Add(-140 * 24 * time.Hour), Until: until}
	source := SourceState{Source: "git", Scope: ActivityHistoryScope, Status: "complete"}
	empty := (GitData{Sources: []SourceState{source}}).Activity(window)
	if empty.Source.Status != "complete" || empty.Commits == nil || len(empty.Commits) != 0 || empty.LastCommit != nil || empty.HistoryAgeDays != nil {
		t.Fatalf("empty history must be confirmed without a last commit: %#v", empty)
	}
	old := ActivityCommit{SHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", CommittedAt: window.From.Add(-time.Second)}
	inactive := (GitData{Sources: []SourceState{source}, MainCommits: []ActivityCommit{old}}).Activity(window)
	if inactive.Source.Status != "complete" || len(inactive.Commits) != 0 || inactive.LastCommit == nil || *inactive.LastCommit != old {
		t.Fatalf("zero commits in the window must retain the last commit: %#v", inactive)
	}
	if inactive.HistoryAgeDays == nil || *inactive.HistoryAgeDays != 140 {
		t.Fatalf("expected 140 full days of history: %#v", inactive)
	}
	missing := (GitData{}).Activity(window)
	if missing.Source.Status != "unavailable" || missing.Source.Reason == nil || missing.Source.Sample.Total != nil || missing.LastCommit != nil || missing.HistoryAgeDays != nil {
		t.Fatalf("missing history must remain unknown: %#v", missing)
	}
}

func TestActivityYoungHistoryHasConfirmedZeroAgeDays(t *testing.T) {
	until := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	window := Window{From: until.Add(-140 * 24 * time.Hour), Until: until}
	data := GitData{
		Sources: []SourceState{{Source: "git", Scope: ActivityHistoryScope, Status: "complete"}},
		MainCommits: []ActivityCommit{{
			SHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", CommittedAt: until.Add(-24*time.Hour + time.Second),
		}},
	}
	activity := data.Activity(window)
	if activity.HistoryAgeDays == nil || *activity.HistoryAgeDays != 0 {
		t.Fatalf("expected a confirmed age of zero full days: %#v", activity)
	}
}

func TestActivityInvalidCommitDateDoesNotClaimCompleteHistory(t *testing.T) {
	until := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	window := Window{From: until.Add(-140 * 24 * time.Hour), Until: until}
	total, fingerprint := 2, "complete-raw-walk"
	data := GitData{
		Sources: []SourceState{{Source: "git", Scope: ActivityHistoryScope, Status: "complete",
			Sample: Sample{Collected: &total, Total: &total}, Fingerprint: &fingerprint}},
		MainCommits: []ActivityCommit{
			{SHA: "valid", CommittedAt: until.Add(-time.Second)},
			{SHA: "future", CommittedAt: until.Add(time.Second)},
		},
		MainWalkComplete: true,
	}
	activity := data.Activity(window)
	source := activity.Source
	if len(activity.Commits) != 1 || source.Status != "partial" || source.Reason == nil || source.Reason.Code != "GIT_HISTORY_INVALID" ||
		source.Sample.Total != nil || source.Fingerprint != nil || source.Sample.Excluded == nil || *source.Sample.Excluded != 1 ||
		activity.LastCommit != nil || activity.HistoryAgeDays != nil {
		t.Fatalf("invalid date must limit the result: %#v", activity)
	}
}
