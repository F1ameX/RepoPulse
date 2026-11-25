package scoring

import (
	"fmt"
	"math"
	"slices"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

const vulnerabilityRiskID = "SEC_VULN_RISK"

// VulnerabilityRisk computes the metric from the normalized snapshot, without network access.
func (e *Engine) VulnerabilityRisk(data *domain.SnapshotData) (domain.Metric, []domain.Finding) {
	var params parameters
	for _, category := range e.config.Categories {
		for _, metric := range category.Metrics {
			if metric.ID == vulnerabilityRiskID {
				params = metric.Parameters
			}
		}
	}
	index := slices.IndexFunc(data.Inputs, func(input domain.MetricInput) bool { return input.ID == vulnerabilityRiskID })
	input := data.Inputs[index]
	metric := domain.Metric{
		Input: input, RuleVersion: e.config.MethodologyVersion,
		Rule: fmt.Sprintf("100 × exp(−Risk / %g)", *params.K), Explanation: unscoredExplanation,
	}
	hasOSV := false
	for i := range data.Sources {
		source := &data.Sources[i]
		if source.Source == "osv" && source.Scope == domain.OSVScope {
			hasOSV = true
			metric.Input.Status, metric.Input.Reason = source.Status, source.Reason
			metric.Input.Sample, metric.Input.ObservedAt = source.Sample, source.ObtainedAt
		} else if source.Source != "git" || source.Scope != "dependencies" {
			continue
		}
		if !slices.Contains(source.MetricIDs, vulnerabilityRiskID) {
			source.MetricIDs = append(source.MetricIDs, vulnerabilityRiskID)
		}
	}
	if !hasOSV {
		return metric, nil
	}

	type key struct{ ecosystem, name, version string }
	dependencies := make(map[key]domain.Dependency, len(data.Dependencies))
	for _, dep := range data.Dependencies {
		dependencies[key{dep.Ecosystem, dep.Package, dep.Version}] = dep
	}

	metric.Input.Evidence = []domain.Evidence{}
	risk, highestWeight := 0.0, 0.0
	unknown := false
	finding := domain.Finding{
		ID: vulnerabilityRiskID, RuleID: vulnerabilityRiskID, Category: "security",
		Title:               "Известные уязвимости зависимостей",
		Description:         "OSV вернул находки для конкретных версий прямых зависимостей.",
		PossibleConsequence: "Используемые версии могут содержать известные уязвимости.",
		Severity:            "informational", Vulnerabilities: data.Vulnerabilities,
	}
	for i, v := range data.Vulnerabilities {
		dep := dependencies[key{v.Ecosystem, v.Package, v.Version}]
		evidence := domain.Evidence{
			ID: fmt.Sprintf("osv:%d", i+1), Source: "osv", Kind: "advisory",
			Description: "Известная уязвимость прямой зависимости",
			Path:        &dep.ManifestPath, URL: v.EvidenceURL, Dependency: &dep, VulnerabilityID: &v.ID,
		}
		metric.Input.Evidence = append(metric.Input.Evidence, evidence)
		finding.EvidenceIDs = append(finding.EvidenceIDs, evidence.ID)
		if v.Severity == nil || params.SeverityWeights[*v.Severity] == 0 {
			unknown = true
			continue
		}
		weight := params.SeverityWeights[*v.Severity]
		risk += weight
		if weight > highestWeight {
			highestWeight, finding.Severity = weight, *v.Severity
		}
	}
	if metric.Input.Status == "complete" && unknown {
		metric.Input.Status = "partial"
		metric.Input.Reason = reason("OSV_SEVERITY_UNKNOWN", "Не удалось определить уровень всех найденных уязвимостей")
	}
	if metric.Input.Status == "complete" {
		metric.Input.RawValue = scalar(risk)
		metric.NormalizedScore = pointer(100 * math.Exp(-risk / *params.K))
		metric.Explanation = "Риск равен сумме весов уникальных известных уязвимостей."
	} else {
		metric.Input.RawValue = domain.Scalar{}
	}
	data.Inputs[index] = metric.Input
	if len(finding.Vulnerabilities) > 0 {
		return metric, []domain.Finding{finding}
	}
	return metric, nil
}
