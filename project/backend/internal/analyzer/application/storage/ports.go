package storage

import (
	"context"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

// Repository only supports immutable acceptance and guarded state changes.
// Complete must publish snapshot, report and completed in one DB transaction.
type Repository interface {
	Create(context.Context, domain.Analysis) (domain.Analysis, error)
	Get(context.Context, string) (domain.Analysis, error)
	Start(context.Context, string, time.Time, time.Time) error
	Complete(context.Context, domain.Result) error
	GetResult(context.Context, string) (domain.Result, error)
}
