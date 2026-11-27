package scoring

import (
	"encoding/json"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

// SecretsCategory uses the complete count of both rule kinds, including
// findings omitted from the bounded evidence list.
func (e *Engine) SecretsCategory(v domain.Versions, inputs []domain.MetricInput) (domain.Category, error) {
	if err := e.checkVersions(v); err != nil {
		return domain.Category{}, err
	}
	if v.SecretRules != "secrets-mvp-1.0.0" {
		return domain.Category{}, invalid("secret rules version")
	}
	for _, cfg := range e.config.Categories {
		if cfg.ID != "security" {
			continue
		}
		metrics := []domain.Metric{}
		for _, spec := range cfg.Metrics {
			for _, input := range inputs {
				if input.ID != spec.ID {
					continue
				}
				if err := input.Validate(); err != nil {
					return domain.Category{}, err
				}
				metric := domain.Metric{Input: input, RuleVersion: v.Methodology, Explanation: unscoredExplanation}
				if input.ID == "SEC_SECRETS" {
					metric.Rule = "Полный скан: точный формат → 0, только подозрительные присваивания → 50, нет совпадений → 100"
					metric.Explanation = "Совпадение шаблона не подтверждает действительность реквизитов. " + unscoredExplanation
					if input.Status == "complete" {
						raw, _ := input.RawValue.MarshalJSON()
						var count int
						if input.Sample.Numerator == nil || input.Sample.Denominator == nil || json.Unmarshal(raw, &count) != nil || count != *input.Sample.Denominator || *input.Sample.Numerator > count {
							return domain.Category{}, invalid("secret counts")
						}
						score := 100.0
						if count > 0 {
							score = 50
						}
						if *input.Sample.Numerator > 0 {
							score = 0
						}
						metric.NormalizedScore = &score
					}
				}
				metrics = append(metrics, metric)
			}
		}
		if len(metrics) != len(cfg.Metrics) {
			return domain.Category{}, invalid("security inputs")
		}
		return summarizeCategory(cfg, metrics, "безопасности", "Потенциальные секреты оцениваются по полному охвату истории и версии правил. Совпадения не подтверждают действительность реквизитов."), nil
	}
	return domain.Category{}, invalid("security category")
}
