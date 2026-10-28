package snapshot

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/worker"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type collectorFunc func(context.Context, domain.Analysis, worker.GitSession) (Collection, error)

func (f collectorFunc) Collect(ctx context.Context, a domain.Analysis, git worker.GitSession) (Collection, error) {
	return f(ctx, a, git)
}

type calculatorFunc func(context.Context, domain.Analysis, domain.SnapshotData) (domain.ReportData, error)

func (f calculatorFunc) Calculate(ctx context.Context, a domain.Analysis, data domain.SnapshotData) (domain.ReportData, error) {
	return f(ctx, a, data)
}

type gitSession struct{ data domain.GitData }

func (s gitSession) Data() domain.GitData                             { return s.data }
func (s gitSession) ReadBlob(context.Context, string) ([]byte, error) { return nil, nil }
func (s gitSession) Close() error                                     { return nil }

func TestProcessorDetachesInputsBeforeCalculation(t *testing.T) {
	a, git := accepted()
	originalSHA := *a.Repository.CommitSHA
	collected := metric(t, "DOC_README", "bool", "false", Source{"git", "tree"})
	collected.Value.Sample.Collected = ptr(0)
	collector := collectorFunc(func(_ context.Context, metadata domain.Analysis, session worker.GitSession) (Collection, error) {
		if metadata.ID != a.ID || metadata.Owner != a.Owner || metadata.Versions != a.Versions || session.Data().Repository.GitHubID != a.Repository.GitHubID {
			t.Fatal("collector received different accepted context")
		}
		*metadata.Repository.CommitSHA = "modified-by-collector"
		return Collection{Inputs: []Input{collected}}, nil
	})
	calculator := calculatorFunc(func(_ context.Context, metadata domain.Analysis, data domain.SnapshotData) (domain.ReportData, error) {
		if *metadata.Repository.CommitSHA != originalSHA || len(data.Inputs) != 12 {
			t.Fatal("metadata was corrupted or calculation received incomplete inputs")
		}
		input := find(t, data, "DOC_README")
		*input.Sample.Collected = 99
		*metadata.Repository.CommitSHA = "modified-by-calculator"
		return domain.ReportData{Summary: "calculation result"}, nil
	})
	var stages []string
	output, err := NewProcessor([]Collector{collector}, calculator).Process(t.Context(), a, gitSession{git}, func(stage string) error {
		stages = append(stages, stage)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(stages, []string{"collecting_external", "calculating"}) || *a.Repository.CommitSHA != originalSHA || output.Report.Summary != "calculation result" {
		t.Fatal("pipeline changed accepted metadata or lost calculated output")
	}
	*collected.Value.Sample.Collected = 100
	if *find(t, output.Snapshot, "DOC_README").Sample.Collected != 0 {
		t.Fatal("producer/calculator can mutate published inputs")
	}
}

func TestProcessorStopsOnCancellationInvalidCollectionAndStageFailure(t *testing.T) {
	for _, name := range []string{"cancel before", "cancel during", "collector error", "bad data", "stage error", "no calculator"} {
		t.Run(name, func(t *testing.T) {
			a, git := accepted()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			called := 0
			collector := collectorFunc(func(context.Context, domain.Analysis, worker.GitSession) (Collection, error) {
				called++
				if name == "cancel during" {
					cancel()
				}
				if name == "collector error" {
					return Collection{}, errors.New("collector failure")
				}
				if name == "bad data" {
					return Collection{Inputs: []Input{metric(t, "DOC_README", "bool", "true", Source{"github", "missing"})}}, nil
				}
				return Collection{}, nil
			})
			var calculator Calculator = calculatorFunc(func(context.Context, domain.Analysis, domain.SnapshotData) (domain.ReportData, error) {
				t.Fatal("calculation ran after a failed prerequisite")
				return domain.ReportData{}, nil
			})
			if name == "no calculator" {
				calculator = nil
			}
			if name == "cancel before" {
				cancel()
			}
			_, err := NewProcessor([]Collector{collector}, calculator).Process(ctx, a, gitSession{git}, func(string) error {
				if name == "stage error" {
					return domain.ErrInvalidTransition
				}
				return nil
			})
			if err == nil {
				t.Fatal("pipeline hid a failure")
			}
			if (name == "cancel before" || name == "no calculator" || name == "stage error") && called != 0 {
				t.Fatal("collection ran before prerequisites were verified")
			}
		})
	}
}
