package scoring

import (
	"encoding/json"
	"fmt"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

// CICategory calculates scores from the same normalized inputs saved in the snapshot.
func (e *Engine) CICategory(versions domain.Versions, inputs []domain.MetricInput) (domain.Category, error) {
	if err := e.checkVersions(versions); err != nil {
		return domain.Category{}, err
	}
	for _, cfg := range e.config.Categories {
		if cfg.ID != "ci_cd" {
			continue
		}
		metrics := make([]domain.Metric, 0, len(cfg.Metrics))
		for _, spec := range cfg.Metrics {
			var input domain.MetricInput
			for _, candidate := range inputs {
				if candidate.ID == spec.ID {
					if input.ID != "" {
						return domain.Category{}, invalid("duplicate CI input")
					}
					input = candidate
				}
			}
			metric, err := e.ciMetric(input, spec)
			if err != nil {
				return domain.Category{}, err
			}
			metrics = append(metrics, metric)
		}
		return summarizeCategory(cfg, metrics, "CI/CD", "Балл учитывает наличие конфигурации CI, долю успешных запусков Actions и обязательные проверки основной ветки."), nil
	}
	return domain.Category{}, invalid("missing CI category configuration")
}

func (e *Engine) ciMetric(input domain.MetricInput, spec metricConfig) (domain.Metric, error) {
	if err := input.Validate(); err != nil {
		return domain.Metric{}, err
	}
	metric := domain.Metric{Input: input, RuleVersion: e.config.MethodologyVersion,
		Rule: "true → 100, false → 0", Explanation: unscoredExplanation}
	if threshold, ok := e.config.FindingThresholds[spec.ID]; ok {
		metric.Threshold = &threshold
	}
	if spec.Normalization == "share" {
		metric.Rule = "100 × успешные / учитываемые завершённые запуски"
	}
	if input.Status != "complete" {
		return metric, nil
	}

	raw, _ := input.RawValue.MarshalJSON()
	switch spec.Normalization {
	case "boolean":
		var present *bool
		if input.Unit != "bool" || json.Unmarshal(raw, &present) != nil || present == nil {
			return domain.Metric{}, invalid("CI boolean raw_value")
		}
		score := 0.0
		metric.Explanation = "В полном дереве нет распознанной конфигурации CI, поэтому метрика даёт 0 баллов."
		if input.ID == "CI_REQUIRED_CHECKS" {
			metric.Explanation = "В полном наборе применимых правил основной ветки нет обязательных CI-проверок, поэтому метрика даёт 0 баллов."
		}
		if *present {
			score = 100
			metric.Explanation = "В дереве найдена конфигурация CI, поэтому метрика даёт 100 баллов за её наличие."
			if input.ID == "CI_REQUIRED_CHECKS" {
				metric.Explanation = "Применимыми правилами основной ветки подтверждены обязательные CI-проверки, поэтому метрика даёт 100 баллов."
			}
		}
		metric.NormalizedScore = &score
	case "share":
		var share *float64
		numerator, denominator := input.Sample.Numerator, input.Sample.Denominator
		if input.Unit != "ratio" || json.Unmarshal(raw, &share) != nil || share == nil ||
			numerator == nil || denominator == nil || *denominator == 0 ||
			*numerator > *denominator || *share != float64(*numerator)/float64(*denominator) {
			return domain.Metric{}, invalid("CI success sample/raw_value mismatch")
		}
		metric.NormalizedScore = new(*share * 100)
		metric.Explanation = fmt.Sprintf("Успешны %d из %d учитываемых запусков GitHub Actions. Балл равен этой доле, умноженной на 100.",
			*numerator, *denominator)
	}
	return metric, nil
}
