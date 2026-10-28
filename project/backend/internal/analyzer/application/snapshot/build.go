package snapshot

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

var metrics = []struct{ id, unit string }{
	{"ACT_COMMIT_FREQ", "commits_per_week"}, {"ACT_DAYS_SINCE_LAST", "days"}, {"ACT_TREND", "ratio"},
	{"DOC_README", "bool"}, {"DOC_SUPPORT_SHARE", "ratio"}, {"TEST_PRESENCE", "bool"},
	{"SEC_SECRETS", "secret_scan"}, {"SEC_VULN_RISK", "risk"}, {"SEC_SAST", "bool"},
	{"CI_PRESENT", "bool"}, {"CI_SUCCESS_RATE", "ratio"}, {"CI_REQUIRED_CHECKS", "bool"},
}

// Build preserves collector observations and accepted metadata. It supplies an
// explicit unknown input for each missing analyzer, never a fabricated result.
func Build(a domain.Analysis, git domain.GitData, collections ...Collection) (domain.SnapshotData, error) {
	if err := a.Validate(); err != nil {
		return domain.SnapshotData{}, err
	}
	if !sameRepository(a, git) {
		return domain.SnapshotData{}, domain.ErrInvalidData
	}
	for _, source := range git.Sources {
		if source.Source != "git" {
			return domain.SnapshotData{}, domain.ErrInvalidData
		}
	}
	data := domain.SnapshotData{Refs: git.Refs}
	sources := map[Source]domain.SourceState{}
	addSources := func(items []domain.SourceState) error {
		for _, item := range items {
			key := Source{item.Source, item.Scope}
			if _, found := sources[key]; found {
				return domain.ErrInvalidData
			}
			if item.ObtainedAt != nil && item.ObtainedAt.IsZero() {
				return domain.ErrInvalidData
			}
			sources[key] = item
			data.Sources = append(data.Sources, item)
		}
		return nil
	}
	if err := addSources(git.Sources); err != nil {
		return domain.SnapshotData{}, err
	}
	inputs := map[string]Input{}
	for _, collection := range collections {
		if err := addSources(collection.Sources); err != nil {
			return domain.SnapshotData{}, err
		}
		for _, input := range collection.Inputs {
			if _, found := inputs[input.Value.ID]; found {
				return domain.SnapshotData{}, domain.ErrInvalidData
			}
			inputs[input.Value.ID] = input
		}
		data.Dependencies = append(data.Dependencies, collection.Dependencies...)
		data.Vulnerabilities = append(data.Vulnerabilities, collection.Vulnerabilities...)
	}
	for _, metric := range metrics {
		collected, found := inputs[metric.id]
		input := collected.Value
		if !found {
			input = domain.MetricInput{ID: metric.id, Unit: metric.unit, Status: "unavailable", Reason: &domain.Reason{
				Code: "INPUT_NOT_COLLECTED", Message: "Входные данные проверки не получены",
				CannotConclude: "Результат этой проверки неизвестен"}}
		} else {
			if input.Unit != metric.unit || len(collected.Sources) == 0 || (input.ObservedAt != nil && input.ObservedAt.IsZero()) {
				return domain.SnapshotData{}, domain.ErrInvalidData
			}
			for _, key := range collected.Sources {
				source, exists := sources[key]
				if !exists {
					return domain.SnapshotData{}, domain.ErrInvalidData
				}
				if (source.Status == "unavailable" || source.Status == "not_applicable") && collected.Value.Status == "complete" {
					return domain.SnapshotData{}, domain.ErrInvalidData
				}
				if source.Status != "complete" && input.Status == "complete" {
					input.Status, input.Reason = "partial", source.Reason
				}
			}
			delete(inputs, metric.id)
		}
		windowed := strings.HasPrefix(metric.id, "ACT_") || metric.id == "CI_SUCCESS_RATE"
		if input.Window != nil && (!input.Window.From.Equal(a.Window.From) || !input.Window.Until.Equal(a.Window.Until)) {
			return domain.SnapshotData{}, domain.ErrInvalidData
		}
		if windowed {
			window := a.Window
			input.Window = &window
		} else if input.Window != nil {
			return domain.SnapshotData{}, domain.ErrInvalidData
		}
		data.Inputs = append(data.Inputs, input)
	}
	if len(inputs) != 0 {
		return domain.SnapshotData{}, domain.ErrInvalidData
	}
	// Own every nested slice/pointer before sorting or normalization.
	data, err := clone(data)
	if err != nil {
		return domain.SnapshotData{}, err
	}
	data.Dependencies, err = dependencies(data.Dependencies)
	if err != nil {
		return domain.SnapshotData{}, err
	}
	data.Vulnerabilities, err = vulnerabilities(data.Vulnerabilities)
	if err != nil {
		return domain.SnapshotData{}, err
	}
	sort.Slice(data.Refs, func(i, j int) bool { return data.Refs[i].Name < data.Refs[j].Name })
	sort.Slice(data.Sources, func(i, j int) bool {
		a, b := data.Sources[i], data.Sources[j]
		return a.Source < b.Source || a.Source == b.Source && a.Scope < b.Scope
	})
	return domain.PrepareSnapshotData(data)
}

func sameRepository(a domain.Analysis, git domain.GitData) bool {
	return reflect.DeepEqual(a.Repository, git.Repository)
}

func clone[T any](value T) (T, error) {
	var copied T
	data, err := json.Marshal(value)
	if err == nil {
		err = json.Unmarshal(data, &copied)
	}
	if err != nil {
		return copied, domain.ErrInvalidData
	}
	return copied, nil
}
