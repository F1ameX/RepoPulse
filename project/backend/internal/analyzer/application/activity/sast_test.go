package activity

import (
	"context"
	"testing"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/scoring"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type workflowSession struct {
	controlSession
	workflow        string
	externalStarted *bool
}

func (s workflowSession) ReadBlob(_ context.Context, object string) ([]byte, error) {
	if *s.externalStarted {
		return nil, domain.ErrInvalidData
	}
	if object == "workflow" && s.workflow != "" {
		return []byte(s.workflow), nil
	}
	return nil, domain.ErrInvalidData
}

func TestRuntimeSASTReport(t *testing.T) {
	a := runningAnalysis(t)
	engine, err := scoring.Default()
	if err != nil {
		t.Fatal(err)
	}
	const workflowPath = ".github/workflows/ci.yml"
	const workflow = "jobs:\n  verify:\n    steps:\n      - "
	for _, tc := range []struct {
		name, path, body, raw, status string
		score                         float64
		extra                         domain.GitFile
	}{
		{name: "CodeQL", path: workflowPath, body: workflow + "uses: github/codeql-action/analyze@v3", raw: "true", status: "complete", score: 100},
		{name: "Semgrep", path: workflowPath, body: workflow + "run: semgrep ci", raw: "true", status: "complete", score: 100},
		{name: "echo", path: workflowPath, body: workflow + "run: echo semgrep ci", raw: "false", status: "complete"},
		{name: "invalid YAML", path: workflowPath, body: "jobs: [", raw: "null", status: "partial"},
		{name: "unreadable", path: workflowPath, raw: "null", status: "partial"},
		{name: "unsupported CI", path: ".gitlab-ci.yml", raw: "null", status: "not_applicable"},
		{name: "reusable workflow", path: workflowPath, body: "jobs:\n  verify:\n    uses: org/repo/.github/workflows/other.yml@main", raw: "null", status: "not_applicable"},
		{name: "incomplete with evidence", path: workflowPath, body: workflow + "run: semgrep scan", raw: "null", status: "partial",
			extra: domain.GitFile{Path: ".github/workflows/missing.yml", Mode: "100644"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			git := domain.GitData{Repository: a.Repository,
				Files:   []domain.GitFile{{Path: tc.path, ObjectSHA: "workflow", Mode: "100644"}},
				Sources: []domain.SourceState{{Source: "git", Scope: domain.GitTreeScope, Status: "complete"}},
			}
			if tc.extra.Path != "" {
				git.Files = append(git.Files, tc.extra)
			}
			externalStarted := false
			osv := fixedOSV(func(_ context.Context, _ []domain.OSVQuery) []domain.OSVResult {
				t.Fatal("workflow-only fixture should not query OSV")
				return nil
			})
			session := workflowSession{controlSession{git}, tc.body, &externalStarted}
			out, err := NewProcessorWithSources(engine, nil, osv).Process(t.Context(), a, session, func(stage string) error {
				if stage == "collecting_external" {
					externalStarted = true
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := domain.NewResult(a, domain.NewID(), domain.NewID(), a.AssessmentAt, out.Snapshot, out.Report); err != nil {
				t.Fatal(err)
			}
			security := out.Report.Categories[3]
			metric := security.Metrics[2]
			raw, _ := metric.Input.RawValue.MarshalJSON()
			if metric.Input.ID != "SEC_SAST" || string(raw) != tc.raw || metric.Input.Status != tc.status {
				t.Fatalf("unexpected SAST result: %s %+v", raw, metric)
			}
			if tc.status == "complete" {
				if metric.NormalizedScore == nil || *metric.NormalizedScore != tc.score || security.Coverage.Calculated != 1 {
					t.Fatalf("unexpected score or coverage: %+v", security)
				}
			} else if metric.Input.Reason == nil || metric.NormalizedScore != nil {
				t.Fatal("N/A must have a reason and no score")
			}
			if tc.raw == "true" || tc.extra.Path != "" {
				if len(metric.Input.Evidence) != 1 || *metric.Input.Evidence[0].Path != workflowPath ||
					*metric.Input.Evidence[0].Revision != *a.Repository.CommitSHA {
					t.Fatalf("missing pinned workflow evidence: %+v", metric.Input.Evidence)
				}
			}
		})
	}
}
