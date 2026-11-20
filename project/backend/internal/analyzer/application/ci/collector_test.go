package ci

import (
	"context"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type recordedCI struct {
	facts   domain.GitHubCI
	sources []domain.SourceState
}

func (record recordedCI) CollectCI(context.Context, domain.RepositoryMetadata, domain.Window) (domain.GitHubCI, []domain.SourceState) {
	return record.facts, record.sources
}

func TestCollectorPreservesAttemptsAndRuleCoverage(t *testing.T) {
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	analysis := domain.Analysis{Window: domain.Window{From: start, Until: start.Add(time.Hour)}}
	tree := domain.SourceState{Source: "git", Scope: domain.GitTreeScope, Status: "complete"}
	git := domain.GitData{Files: []domain.GitFile{{Path: ".github/workflows/ci.yml", Mode: "100644"}}, Sources: []domain.SourceState{tree}}
	const runID = "9007199254740993"
	reader := recordedCI{
		facts: domain.GitHubCI{Attempts: []domain.ActionsAttempt{
			{RunID: runID, Attempt: 1, Status: "completed", Conclusion: "success", CreatedAt: start, UpdatedAt: start.Add(time.Minute)},
			{RunID: runID, Attempt: 2, Status: "in_progress", CreatedAt: start, UpdatedAt: start.Add(2 * time.Minute)},
		}},
		sources: []domain.SourceState{
			{Source: "github", Scope: domain.BranchRulesScope, Status: "unavailable", Reason: &domain.Reason{Code: "GITHUB_CI_ACCESS_UNAVAILABLE"}},
			{Source: "github", Scope: domain.ActionsScope, Status: "complete"},
		},
	}
	external, err := (Collector{Reader: reader}).CollectCI(t.Context(), analysis)
	if err != nil {
		t.Fatal(err)
	}
	result := Inputs(analysis, git, tree, external)
	rate, rules := result.Inputs[1].Value, result.Inputs[2].Value
	if rate.Status != "complete" || len(rate.Evidence) != 1 || *rate.Evidence[0].RunID != runID || *rate.Evidence[0].Attempt != 1 {
		t.Fatalf("latest completed attempt lost: %+v", rate)
	}
	raw, _ := rules.RawValue.MarshalJSON()
	if rules.Status != "unavailable" || string(raw) != "null" || rules.Reason.Code != "GITHUB_CI_ACCESS_UNAVAILABLE" {
		t.Fatalf("missing branch permissions reported incorrectly: %+v", rules)
	}
	if result.GitHubCI == nil || len(result.GitHubCI.Attempts) != 2 || !result.GitHubCI.ConfigTreeComplete || len(result.GitHubCI.ConfigPaths) != 1 {
		t.Fatal("normalized GitHub observations or pinned configuration missing from snapshot")
	}
}
