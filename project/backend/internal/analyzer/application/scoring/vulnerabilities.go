package scoring

import (
	"fmt"
	"slices"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

const vulnerabilityRiskID = "SEC_VULN_RISK"

// PrepareVulnerabilityRisk writes the normalized OSV input before snapshot publication.
func (e *Engine) PrepareVulnerabilityRisk(data *domain.SnapshotData) {
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
	hasOSV := false
	for i := range data.Sources {
		source := &data.Sources[i]
		if source.Source == "osv" && source.Scope == domain.OSVScope {
			hasOSV = true
			input.Status, input.Reason = source.Status, source.Reason
			input.Sample, input.ObservedAt = source.Sample, source.ObtainedAt
		} else if source.Source != "git" || source.Scope != "dependencies" {
			continue
		}
		if !slices.Contains(source.MetricIDs, vulnerabilityRiskID) {
			source.MetricIDs = append(source.MetricIDs, vulnerabilityRiskID)
		}
	}
	if !hasOSV {
		return
	}

	type key struct{ ecosystem, name, version string }
	dependencies := make(map[key]domain.Dependency, len(data.Dependencies))
	for _, dep := range data.Dependencies {
		dependencies[key{dep.Ecosystem, dep.Package, dep.Version}] = dep
	}

	input.Evidence = []domain.Evidence{}
	risk := 0.0
	unknown := false
	for i, v := range data.Vulnerabilities {
		dep := dependencies[key{v.Ecosystem, v.Package, v.Version}]
		evidence := domain.Evidence{
			ID: fmt.Sprintf("osv:%d", i+1), Source: "osv", Kind: "advisory",
			Description: "Известная уязвимость прямой зависимости",
			Path:        &dep.ManifestPath, URL: v.EvidenceURL, Dependency: &dep, VulnerabilityID: &v.ID,
		}
		input.Evidence = append(input.Evidence, evidence)
		if v.Severity == nil || params.SeverityWeights[*v.Severity] == 0 {
			unknown = true
			continue
		}
		weight := params.SeverityWeights[*v.Severity]
		risk += weight
	}
	if input.Status == "complete" && unknown {
		input.Status = "partial"
		input.Reason = reason("OSV_SEVERITY_UNKNOWN", "Не удалось определить уровень всех найденных уязвимостей")
	}
	if input.Status == "complete" {
		input.RawValue = scalar(risk)
	} else {
		input.RawValue = domain.Scalar{}
	}
	data.Inputs[index] = input
}

func vulnerabilityFindings(data domain.SnapshotData) []domain.Finding {
	if len(data.Vulnerabilities) == 0 {
		return []domain.Finding{}
	}
	index := slices.IndexFunc(data.Inputs, func(input domain.MetricInput) bool { return input.ID == vulnerabilityRiskID })
	evidence := data.Inputs[index].Evidence
	if len(evidence) == 0 {
		return []domain.Finding{}
	}
	finding := domain.Finding{
		ID: vulnerabilityRiskID, RuleID: vulnerabilityRiskID, Category: "security",
		Title:               "Известные уязвимости зависимостей",
		Description:         "OSV вернул находки для конкретных версий прямых зависимостей.",
		PossibleConsequence: "Используемые версии могут содержать известные уязвимости.",
		Severity:            "informational", Vulnerabilities: data.Vulnerabilities,
	}
	severities := []string{"informational", "low", "medium", "high", "critical"}
	for _, v := range data.Vulnerabilities {
		if v.Severity != nil && slices.Index(severities, *v.Severity) > slices.Index(severities, finding.Severity) {
			finding.Severity = *v.Severity
		}
	}
	for _, item := range evidence {
		if item.Source == "osv" && item.VulnerabilityID != nil {
			finding.EvidenceIDs = append(finding.EvidenceIDs, item.ID)
		}
	}
	return []domain.Finding{finding}
}
