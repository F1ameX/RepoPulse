// Package snapshot assembles safe, reproducible inputs before calculation and
// atomic publication by the worker. It does not implement metric/scoring rules.
package snapshot

import (
	"context"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/worker"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

// Source identifies the exact collected scope used by an input. A partial scope
// cannot silently become a complete metric during assembly.
type Source struct{ Name, Scope string }

type Input struct {
	Value   domain.MetricInput
	Sources []Source
}

// Collection contains only safe facts from trusted in-process collectors. It
// intentionally cannot assign IDs, owner, repository, assessment time or versions.
// Source failures are represented by completeness/reason, not an error containing
// an upstream response. An error aborts the job (invalid data or cancellation).
type Collection struct {
	Inputs          []Input
	Sources         []domain.SourceState
	Dependencies    []domain.Dependency
	Vulnerabilities []domain.Vulnerability
}

type Collector interface {
	Collect(context.Context, domain.Analysis, worker.GitSession) (Collection, error)
}

// Calculator receives the complete detached snapshot, including unknown inputs.
// It must return the report for precisely these inputs, without fetching sources.
type Calculator interface {
	Calculate(context.Context, domain.Analysis, domain.SnapshotData) (domain.ReportData, error)
}

type Processor struct {
	collectors []Collector
	calculator Calculator
}

func NewProcessor(collectors []Collector, calculator Calculator) *Processor {
	return &Processor{collectors: append([]Collector{}, collectors...), calculator: calculator}
}

func (p *Processor) Process(ctx context.Context, a domain.Analysis, git worker.GitSession, advance func(string) error) (worker.Output, error) {
	if p.calculator == nil || git == nil || advance == nil {
		return worker.Output{}, domain.ErrInvalidData
	}
	if err := ctx.Err(); err != nil {
		return worker.Output{}, err
	}
	if err := a.Validate(); err != nil {
		return worker.Output{}, err
	}
	if !sameRepository(a, git.Data()) {
		return worker.Output{}, domain.ErrInvalidData
	}
	if err := advance("collecting_external"); err != nil {
		return worker.Output{}, err
	}
	collections := make([]Collection, 0, len(p.collectors))
	for _, collector := range p.collectors {
		if err := ctx.Err(); err != nil {
			return worker.Output{}, err
		}
		if collector == nil {
			return worker.Output{}, domain.ErrInvalidData
		}
		// A collector may retain its arguments. Do not let it mutate accepted
		// metadata, or mutate a previous contribution during later collection.
		metadata, err := clone(a)
		if err != nil {
			return worker.Output{}, err
		}
		collected, err := collector.Collect(ctx, metadata, git)
		if err != nil {
			return worker.Output{}, err
		}
		collected, err = clone(collected)
		if err != nil {
			return worker.Output{}, err
		}
		collections = append(collections, collected)
	}
	if err := ctx.Err(); err != nil {
		return worker.Output{}, err
	}
	data, err := Build(a, git.Data(), collections...)
	if err != nil {
		return worker.Output{}, err
	}
	if err := advance("calculating"); err != nil {
		return worker.Output{}, err
	}
	// Preserve the snapshot even if calculation modifies its own input view.
	calculationData, err := clone(data)
	if err != nil {
		return worker.Output{}, err
	}
	metadata, err := clone(a)
	if err != nil {
		return worker.Output{}, err
	}
	report, err := p.calculator.Calculate(ctx, metadata, calculationData)
	if err != nil {
		return worker.Output{}, err
	}
	if err := ctx.Err(); err != nil {
		return worker.Output{}, err
	}
	return worker.Output{Snapshot: data, Report: report}, nil
}
