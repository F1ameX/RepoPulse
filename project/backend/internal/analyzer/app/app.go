// Package app wires the Analyzer Service process.
package app

import (
	"context"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/snapshot"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/worker"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/logger"
)

const Name = "analyzer-service"

// Run starts the persistent Analyzer queue and Git collector. Real processors
// are connected through RunWithProcessor; migrations are applied externally.
func Run(ctx context.Context) error { return RunWithProcessor(ctx, nil) }

// RunWithInputs assembles collected inputs before invoking a real calculator.
// The worker publishes snapshot/report/completed together through storage.
func RunWithInputs(ctx context.Context, collectors []snapshot.Collector, calculator snapshot.Calculator) error {
	return RunWithProcessor(ctx, snapshot.NewProcessor(collectors, calculator))
}

// RunWithProcessor is wired to real normalization/calculation adapters as those
// neighboring tasks land. No fixture or fabricated result is used in production.
func RunWithProcessor(ctx context.Context, processor worker.Processor) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	runner, closeStorage, err := OpenWorker(ctx, cfg, processor)
	if err != nil {
		return err
	}
	defer closeStorage()
	logger.From(ctx).InfoContext(ctx, "Analyzer queue ready")
	if err := runner.Run(ctx, cfg.PollInterval); err != nil {
		return err
	}
	logger.From(ctx).InfoContext(ctx, "Service stopped")
	return nil
}
