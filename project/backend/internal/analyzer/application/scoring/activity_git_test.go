package scoring

import (
	"slices"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

func TestActivityFromGitCommitAtAssessmentHasZeroFrequencyAndAge(t *testing.T) {
	a := domain.Analysis{AssessmentAt: assessment, Window: window(), Versions: versions()}
	data := domain.GitData{
		Sources:     []domain.SourceState{{Source: "git", Scope: domain.ActivityHistoryScope, Status: "complete"}},
		MainCommits: []domain.ActivityCommit{{SHA: "at-t", CommittedAt: assessment}},
	}
	category, err := engine(t).ActivityFromGit(a, data)
	if err != nil {
		t.Fatal(err)
	}
	frequency, last := category.Metrics[0].Input, category.Metrics[1].Input
	if frequency.Status != "complete" || last.Status != "complete" || last.ObservedAt == nil || !last.ObservedAt.Equal(assessment) {
		t.Fatalf("commit at t must be known without entering the window: %#v", category.Metrics)
	}
	closeNumber(t, rawNumber(t, frequency), 0)
	closeNumber(t, rawNumber(t, last), 0)
}

func TestActivityFromGitUsesOnlyFullWeeksForTrend(t *testing.T) {
	a := domain.Analysis{AssessmentAt: assessment, Window: window(), Versions: versions()}
	const day = 24 * time.Hour
	data := domain.GitData{
		Sources: []domain.SourceState{{Source: "git", Scope: domain.ActivityHistoryScope, Status: "complete"}},
		MainCommits: []domain.ActivityCommit{
			{SHA: "before-first-full-week", CommittedAt: assessment.Add(-22 * day)},
			{SHA: "first-week", CommittedAt: assessment.Add(-21 * day)},
			{SHA: "second-week", CommittedAt: assessment.Add(-14 * day)},
			{SHA: "third-week", CommittedAt: assessment.Add(-7 * day)},
		},
	}
	category, err := engine(t).ActivityFromGit(a, data)
	if err != nil {
		t.Fatal(err)
	}
	closeNumber(t, rawNumber(t, category.Metrics[0].Input), 4.0/20)
	if trend := category.Metrics[2].Input; trend.Status != "complete" || !slices.Equal(trend.Series, []int{1, 1, 1}) {
		t.Fatalf("expected three complete weeks: %#v", trend)
	}
}

func TestActivityFromGitPartialHistoryNeedsIndependentLastCommitProof(t *testing.T) {
	a := domain.Analysis{AssessmentAt: assessment, Window: window(), Versions: versions()}
	data := domain.GitData{
		Sources: []domain.SourceState{{Source: "git", Scope: domain.ActivityHistoryScope, Status: "partial",
			Reason: reason("GIT_HISTORY_INTERRUPTED", "Обход истории прерван")}},
		MainCommits: []domain.ActivityCommit{{SHA: "known-001", CommittedAt: assessment.Add(-9 * 24 * time.Hour)}},
	}
	e := engine(t)
	category, err := e.ActivityFromGit(a, data)
	if err != nil {
		t.Fatal(err)
	}
	if last := category.Metrics[1].Input; last.Status == "complete" || last.ObservedAt != nil || rawNumber(t, last) != nil {
		t.Fatalf("one observed commit did not prove the latest date: %#v", last)
	}

	// A complete pinned walk can prove the latest commit separately when
	// collection of the advertised refs was interrupted.
	data.MainWalkComplete = true
	category, err = e.ActivityFromGit(a, data)
	if err != nil {
		t.Fatal(err)
	}
	frequency, last, trend := category.Metrics[0].Input, category.Metrics[1].Input, category.Metrics[2].Input
	if frequency.Status != "partial" || frequency.Sample.Total != nil || rawNumber(t, frequency) != nil ||
		last.Status != "complete" || trend.Status != "unavailable" || len(trend.Series) != 0 {
		t.Fatalf("wrong independent completeness: %#v", category.Metrics)
	}
	closeNumber(t, rawNumber(t, last), 9)
}
