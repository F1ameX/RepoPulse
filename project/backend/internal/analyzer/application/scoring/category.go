package scoring

import "github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"

// summarizeCategory uses the same order as the validated metric configuration.
func summarizeCategory(cfg categoryConfig, metrics []domain.Metric, subject, explanation string) domain.Category {
	category := domain.Category{
		ID: cfg.ID, Metrics: metrics, Explanation: explanation,
		Coverage: domain.Coverage{Total: len(cfg.Metrics)},
	}
	var score, weights float64
	for i, metric := range metrics {
		if metric.NormalizedScore != nil {
			category.Coverage.Calculated++
			weight := cfg.Metrics[i].Weight
			weights += weight
			score += (*metric.NormalizedScore - score) * (weight / weights)
		}
		switch metric.Input.Status {
		case "partial":
			category.Coverage.Partial++
		case "unavailable":
			category.Coverage.Unavailable++
		case "not_applicable":
			category.Coverage.NotApplicable++
		}
	}
	coverage := &category.Coverage
	coverage.Percentage = float64(coverage.Calculated) * 100 / float64(coverage.Total)
	switch {
	case coverage.Calculated == coverage.Total:
		category.Status = "complete"
	case coverage.Calculated > 0:
		category.Status = "partial"
		category.Reason = reason("INCOMPLETE_METRICS", "Доступны не все метрики "+subject)
	case coverage.NotApplicable == coverage.Total:
		category.Status = "not_applicable"
		category.Reason = reason("NO_APPLICABLE_METRICS", "Нет применимых метрик "+subject)
	default:
		category.Status = "unavailable"
		category.Reason = reason("NO_DATA", "Нет достаточных данных для оценки "+subject)
	}
	if weights > 0 {
		category.Score = pointer(score)
	}
	return category
}
