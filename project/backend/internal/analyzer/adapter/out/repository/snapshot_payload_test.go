package repository

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

func TestSnapshotPayloadReadsOldRowsAndKeepsCIFacts(t *testing.T) {
	old, err := os.ReadFile("../../../../../../../contracts/examples/snapshot.json")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := decodeStoredSnapshot(old)
	if err != nil || snapshot.GitHubCI != nil {
		t.Fatalf("legacy payload: ci=%+v err=%v", snapshot.GitHubCI, err)
	}
	at := snapshot.AssessmentAt.Add(-time.Second)
	snapshot.Sources = append(snapshot.Sources,
		domain.SourceState{Source: "github", Scope: domain.ActionsScope, Status: "complete", ObtainedAt: &at},
		domain.SourceState{Source: "github", Scope: domain.BranchRulesScope, Status: "complete", ObtainedAt: &at})
	zero := 0
	ci := &domain.GitHubCI{ClassicRequiredChecks: &zero, ClassicObtainedAt: &at, Rules: []domain.BranchRule{
		{RulesetID: "7", Type: "required_status_checks", RequiredChecks: 1, ObtainedAt: at},
	}, Attempts: []domain.ActionsAttempt{
		{RunID: "9007199254740993", Attempt: 2, HeadSHA: *snapshot.Repository.CommitSHA, Status: "completed", Conclusion: "success",
			CreatedAt: at, UpdatedAt: at, ObtainedAt: at},
	}}
	body, err := json.Marshal(storedSnapshot{Snapshot: snapshot, GitHubCI: ci})
	if err != nil {
		t.Fatal(err)
	}
	restored, err := decodeStoredSnapshot(body)
	if err != nil || restored.GitHubCI == nil {
		t.Fatalf("CI payload: ci=%+v err=%v", restored.GitHubCI, err)
	}
	latest, ok := restored.GitHubCI.LatestCompleted(snapshot.Window)
	if !ok || latest.RunID != ci.Attempts[0].RunID || latest.Attempt != 2 ||
		len(restored.GitHubCI.Rules) != 1 || restored.GitHubCI.Rules[0] != ci.Rules[0] {
		t.Fatalf("changed CI facts after persistence: %+v", restored.GitHubCI)
	}
}
