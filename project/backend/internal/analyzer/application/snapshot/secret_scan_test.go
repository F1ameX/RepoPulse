package snapshot

import (
	"context"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/worker"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

func TestSecretScanFailureIsIsolatedAndBounded(t *testing.T) {
	a, git := accepted()
	ctx, cancel := context.WithTimeout(t.Context(), domain.CoreTimeLimit)
	defer cancel()
	called := false
	scan := SecretScanCollector{Scanner: collectorFunc(func(scanCtx context.Context, _ domain.Analysis, _ worker.GitSession) (Collection, error) {
		called = true
		deadline, ok := scanCtx.Deadline()
		if !ok || time.Until(deadline) > 120*time.Second {
			t.Fatal("scanner did not get a separate deadline")
		}
		return Collection{}, context.DeadlineExceeded
	})}
	collected, err := scan.Collect(ctx, a, gitSession{git})
	if err != nil || !called || len(collected.Sources) != 1 || collected.Sources[0].Status != "unavailable" ||
		collected.Sources[0].Reason.Code != "SECRET_SCAN_TIME_LIMIT" || ctx.Err() != nil {
		t.Fatalf("scan failure affected another phase: %+v %v", collected, err)
	}
	if _, err := Build(a, git, collected); err != nil {
		t.Fatalf("scan refusal could not be saved beside Git: %v", err)
	}
}
