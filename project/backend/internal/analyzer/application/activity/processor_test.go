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

func TestPartialHistoryPreservesIndependentLastCommitProof(t *testing.T) {
	body, err := os.ReadFile("../../../../../../contracts/examples/snapshot.json")
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := domain.DecodeSnapshot(body)
	if err != nil {
		t.Fatal(err)
	}
	a := domain.Analysis{ID: fixture.AnalysisID, SubmissionID: domain.NewID(), RequestID: domain.NewID(), Owner: fixture.Owner, Repository: fixture.Repository,
		AssessmentAt: fixture.AssessmentAt, CreatedAt: fixture.AssessmentAt, Window: fixture.Window, Versions: fixture.Versions, Status: "running"}
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

type manifestSession struct{ controlSession }

func (s manifestSession) ReadBlob(context.Context, string) ([]byte, error) {
	return []byte("module example.com/root\nrequire example.com/lib v1.2.3\n"), nil
}

// This is the processor constructed by app.Run, not the optional RunWithInputs path.
func TestRuntimeProcessorPreservesDependenciesInSnapshot(t *testing.T) {
	body, err := os.ReadFile("../../../../../../contracts/examples/snapshot.json")
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := domain.DecodeSnapshot(body)
	if err != nil {
		t.Fatal(err)
	}
	a := domain.Analysis{ID: fixture.AnalysisID, SubmissionID: domain.NewID(), RequestID: domain.NewID(), Owner: fixture.Owner, Repository: fixture.Repository,
		AssessmentAt: fixture.AssessmentAt, CreatedAt: fixture.AssessmentAt, Window: fixture.Window, Versions: fixture.Versions, Status: "running"}
	engine, err := scoring.Default()
	if err != nil {
		t.Fatal(err)
	}
	git := domain.GitData{Repository: a.Repository, Files: []domain.GitFile{{Path: "go.mod", ObjectSHA: "manifest", Mode: "100644"}},
		MainWalkComplete: true, MainCommits: []domain.ActivityCommit{{SHA: *a.Repository.CommitSHA, CommittedAt: a.AssessmentAt.Add(-time.Hour)}},
		Sources: []domain.SourceState{{Source: "git", Scope: "tree", Status: "complete"}, {Source: "git", Scope: domain.ActivityHistoryScope, Status: "complete"}}}
	output, err := NewProcessor(engine).Process(t.Context(), a, manifestSession{controlSession{git}}, func(string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(output.Snapshot.Dependencies) != 1 {
		t.Fatalf("%+v", output.Snapshot)
	}
	d := output.Snapshot.Dependencies[0]
	if d.Ecosystem != "Go" || d.Package != "example.com/lib" || d.Version != "v1.2.3" || d.ManifestPath != "go.mod" || d.VersionSource != "go_require_declaration" || len(d.Origins) != 1 {
		t.Fatalf("%+v", d)
	}
	found := false
	for _, s := range output.Snapshot.Sources {
		if s.Scope == "dependencies" && s.Status == "complete" {
			found = true
		}
	}
	if !found {
		t.Fatal("dependency collection state lost during snapshot.Build")
	}
}
