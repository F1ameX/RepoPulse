package scoring

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

func (e *Engine) securityCategory(data domain.SnapshotData) (domain.Category, error) {
	for _, cfg := range e.config.Categories {
		if cfg.ID != "security" {
			continue
		}
		var metrics []domain.Metric
		for _, spec := range cfg.Metrics {
			for _, input := range data.Inputs {
				if input.ID != spec.ID {
					continue
				}
				m, err := e.securityMetric(input, spec, data.Vulnerabilities)
				if err != nil {
					return domain.Category{}, err
				}
				metrics = append(metrics, m)
			}
		}
		return summarizeCategory(cfg, metrics, "безопасности", "Балл учитывает полный скан секретов, риск известных уязвимостей прямых зависимостей и наличие настройки SAST."), nil
	}
	return domain.Category{}, invalid("missing security configuration")
}

func (e *Engine) securityMetric(input domain.MetricInput, spec metricConfig, vulnerabilities []domain.Vulnerability) (domain.Metric, error) {
	m := domain.Metric{Input: input, RuleVersion: e.config.MethodologyVersion, Explanation: unscoredExplanation}
	raw, _ := input.RawValue.MarshalJSON()
	switch spec.ID {
	case "SEC_SECRETS":
		m.Rule = "Полный скан: точный формат → 0, подозрительный шаблон → 50, совпадений нет → 100"
		if input.Status == "complete" {
			var value *float64
			if input.Unit != "secret_scan" || json.Unmarshal(raw, &value) != nil || value == nil || (*value != 0 && *value != 50 && *value != 100) {
				return domain.Metric{}, invalid("secret scan result")
			}
			m.NormalizedScore = value
			m.Explanation = m.Rule
		}
	case "SEC_VULN_RISK":
		k := *spec.Parameters.K
		w := spec.Parameters.SeverityWeights
		m.Rule = fmt.Sprintf("Risk = %gC + %gH + %gM + %gL; 100 × exp(−Risk/%g)", w["critical"], w["high"], w["medium"], w["low"], k)
		if input.Status == "complete" {
			var value *float64
			var risk float64
			// Snapshot validation guarantees unique advisory/ecosystem/package/version tuples.
			for _, v := range vulnerabilities {
				if v.Severity == nil {
					return domain.Metric{}, invalid("complete risk with unknown severity")
				}
				risk += w[*v.Severity] // None has weight zero.
			}
			if input.Unit != "risk" || json.Unmarshal(raw, &value) != nil || value == nil || *value != risk {
				return domain.Metric{}, invalid("OSV risk/raw_value mismatch")
			}
			m.NormalizedScore = pointer(100 * math.Exp(-risk/k))
			m.Explanation = fmt.Sprintf("Суммарный риск известных уязвимостей проверенных прямых зависимостей: %g.", risk)
		}
	case "SEC_SAST":
		m.Rule = "Настройка SAST: true → 100, false → 0"
		if input.Status == "complete" {
			var value *bool
			if input.Unit != "bool" || json.Unmarshal(raw, &value) != nil || value == nil {
				return domain.Metric{}, invalid("SAST raw_value")
			}
			score := 0.0
			if *value {
				score = 100
			}
			m.NormalizedScore = &score
			m.Explanation = "Оценивается наличие настройки SAST, а не результат запуска."
		}
	}
	return m, nil
}
