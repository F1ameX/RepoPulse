package scoring

import "github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"

// SecurityCategory puts the security inputs and the OSV metric in report order.
func (e *Engine) SecurityCategory(inputs []domain.MetricInput, risk *domain.Metric) (domain.Category, error) {
	for _, cfg := range e.config.Categories {
		if cfg.ID != "security" {
			continue
		}
		metrics := make([]domain.Metric, 0, len(cfg.Metrics))
		for _, spec := range cfg.Metrics {
			var input domain.MetricInput
			for _, candidate := range inputs {
				if candidate.ID == spec.ID {
					input = candidate
					break
				}
			}
			if err := input.Validate(); err != nil {
				return domain.Category{}, err
			}
			metric := domain.Metric{Input: input, RuleVersion: e.config.MethodologyVersion, Explanation: unscoredExplanation}
			switch input.ID {
			case "SEC_VULN_RISK":
				if risk != nil {
					metric = *risk
				}
			case "SEC_SAST":
				metric.Rule = "true → 100, false → 0"
				if input.Status == "complete" {
					raw, _ := input.RawValue.MarshalJSON()
					switch string(raw) {
					case "true":
						metric.NormalizedScore = new(100.0)
						metric.Explanation = "В GitHub Actions распознана настройка CodeQL или Semgrep."
					case "false":
						metric.NormalizedScore = new(0.0)
						metric.Explanation = "При полной проверке поддерживаемой конфигурации настройка SAST не обнаружена."
					default:
						return domain.Category{}, invalid("SAST boolean raw_value")
					}
				}
			}
			metrics = append(metrics, metric)
		}
		return summarizeCategory(cfg, metrics, "безопасности", "В оценку категории входят метрики с рассчитанным баллом."), nil
	}
	return domain.Category{}, invalid("missing security category configuration")
}
