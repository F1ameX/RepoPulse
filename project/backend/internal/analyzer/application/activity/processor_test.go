package activity

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/scoring"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type controlSession struct{ data domain.GitData }

func (s controlSession) Data() domain.GitData { return s.data }
func (s controlSession) ReadBlob(context.Context, string) ([]byte, error) {
	return nil, domain.ErrInvalidData
}
func (s controlSession) Close() error { return nil }

func runningAnalysis(t *testing.T) domain.Analysis {
	t.Helper()
	body, err := os.ReadFile("../../../../../../contracts/examples/snapshot.json")
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := domain.DecodeSnapshot(body)
	if err != nil {
		t.Fatal(err)
	}
	return domain.Analysis{ID: fixture.AnalysisID, SubmissionID: domain.NewID(), RequestID: domain.NewID(), Owner: fixture.Owner, Repository: fixture.Repository,
		AssessmentAt: fixture.AssessmentAt, CreatedAt: fixture.AssessmentAt, Window: fixture.Window, Versions: fixture.Versions, Status: "running", StartedAt: &fixture.AssessmentAt}
}

func TestPartialHistoryPreservesIndependentLastCommitProof(t *testing.T) {
	a := runningAnalysis(t)
	engine, err := scoring.Default()
	if err != nil {
		t.Fatal(err)
	}
	data := domain.GitData{Repository: a.Repository, MainCommits: []domain.ActivityCommit{{SHA: *a.Repository.CommitSHA, CommittedAt: a.AssessmentAt.Add(-9 * 24 * time.Hour)}},
		Sources: []domain.SourceState{{Source: "git", Scope: domain.ActivityHistoryScope, Status: "partial", Reason: &domain.Reason{
			Code: "GIT_FETCH_FAILED", Message: "Другие refs не получены", CannotConclude: "Полнота окна неизвестна"}}}}
	for _, confirmed := range []bool{false, true} {
		data.MainWalkComplete = confirmed
		output, err := NewProcessor(engine).Process(t.Context(), a, controlSession{data}, func(string) error { return nil })
		if err != nil {
			t.Fatal(err)
		}
		last := output.Report.Categories[0].Metrics[1]
		if confirmed {
			if last.Input.Status != "complete" || last.NormalizedScore == nil || last.Input.ObservedAt == nil {
				t.Fatal("independent proof was lost during snapshot assembly")
			}
		} else {
			raw, _ := last.Input.RawValue.MarshalJSON()
			if string(raw) != "null" || last.NormalizedScore != nil || last.Input.ObservedAt != nil {
				t.Fatal("one observed commit falsely proved the last date")
			}
		}
	}
}
