package snapshot

import (
	"context"
	"errors"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/worker"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

const (
	secretScanTimeLimit = 120 * time.Second
	calculationReserve  = 5 * time.Second
)

// SecretScanCollector bounds the scanner's phase. The Git session separately
// limits its blob reads. The scanner must return normalized partial facts on
// interruption if it has already confirmed matches.
type SecretScanCollector struct{ Scanner Collector }

func (c SecretScanCollector) Collect(ctx context.Context, a domain.Analysis, git worker.GitSession) (Collection, error) {
	if c.Scanner == nil {
		return unavailableScan("SECRET_SCAN_NOT_CONFIGURED"), nil
	}
	if err := ctx.Err(); err != nil {
		return Collection{}, err
	}
	limit := secretScanTimeLimit
	if deadline, ok := ctx.Deadline(); ok {
		limit = min(limit, max(0, time.Until(deadline)-calculationReserve))
	}
	scanCtx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	if scanCtx.Err() != nil {
		return unavailableScan("SECRET_SCAN_TIME_LIMIT"), nil
	}
	collected, err := c.Scanner.Collect(scanCtx, a, git)
	if ctx.Err() != nil {
		return Collection{}, ctx.Err()
	}
	code := "SECRET_SCAN_UNAVAILABLE"
	if errors.Is(scanCtx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		code = "SECRET_SCAN_TIME_LIMIT"
	}
	for _, source := range collected.Sources {
		if source.Source == "git" && source.Scope == "secret_scan" {
			if scanCtx.Err() == nil && err == nil || source.Status == "partial" && source.Reason != nil {
				return collected, nil
			}
		}
	}
	return unavailableScan(code), nil
}

func unavailableScan(code string) Collection {
	key := Source{Name: "git", Scope: "secret_scan"}
	reason := &domain.Reason{Code: code, Message: "Результат сканирования секретов недоступен",
		CannotConclude: "Нельзя подтвердить отсутствие потенциальных секретов"}
	return Collection{
		Sources: []domain.SourceState{{Source: key.Name, Scope: key.Scope, Status: "unavailable", Reason: reason}},
		Inputs:  []Input{{Value: domain.MetricInput{ID: "SEC_SECRETS", Unit: "secret_scan", Status: "unavailable", Reason: reason}, Sources: []Source{key}}},
	}
}
