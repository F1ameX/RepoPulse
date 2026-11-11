// Package scoring calculates deterministic scores from collected, versioned facts.
// It does not read Git, call external sources or publish reports.
package scoring

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

//go:embed config/rhs-mvp-1.1.0.json
var defaultConfig []byte

type parameters struct {
	Max             *float64           `json:"max,omitempty"`
	K               *float64           `json:"k,omitempty"`
	M0              *float64           `json:"M0,omitempty"`
	SeverityWeights map[string]float64 `json:"severity_weights,omitempty"`
}
type metricConfig struct {
	ID            string     `json:"id"`
	Weight        float64    `json:"weight"`
	Normalization string     `json:"normalization"`
	Parameters    parameters `json:"parameters"`
}
type categoryConfig struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Weight  float64        `json:"weight"`
	Metrics []metricConfig `json:"metrics"`
}
type configuration struct {
	MethodologyVersion      string             `json:"methodology_version"`
	ApprovalState           string             `json:"approval_state"`
	Platform                string             `json:"platform"`
	WindowDays              int                `json:"window_days"`
	PeriodDays              int                `json:"period_days"`
	MaxPeriods              int                `json:"max_periods"`
	DisplayDecimalPlaces    int                `json:"display_decimal_places"`
	Categories              []categoryConfig   `json:"categories"`
	FindingThresholds       map[string]float64 `json:"finding_thresholds"`
	SupportingDocuments     []string           `json:"supporting_documents"`
	Notes                   string             `json:"notes"`
	ScoringConfigVersion    string             `json:"scoring_config_version"`
	CategoryWeightBasis     string             `json:"category_weight_basis"`
	CollectorProfileVersion string             `json:"collector_profile_version"`
	ScoreMetricCount        int                `json:"score_metric_count"`
}

// Engine owns its decoded configuration; callers cannot mutate its weights.
// A new deployed configuration must have a new version; saved snapshots keep theirs.
type Engine struct{ config configuration }

func Default() (*Engine, error) { return Load(bytes.NewReader(defaultConfig)) }

// Load rejects missing weights/parameters and unknown or duplicate metric IDs.
// Category definitions share the normative configuration, including those
// whose metrics are implemented in later sprints.
func Load(reader io.Reader) (*Engine, error) {
	var cfg configuration
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return nil, invalid("configuration JSON")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, invalid("trailing configuration JSON")
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &Engine{config: cfg}, nil
}

func invalid(field string) error  { return fmt.Errorf("%w: scoring %s", domain.ErrInvalidData, field) }
func positive(value float64) bool { return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0) }

func (c configuration) validate() error {
	if strings.TrimSpace(c.MethodologyVersion) == "" || strings.TrimSpace(c.ScoringConfigVersion) == "" || strings.TrimSpace(c.CollectorProfileVersion) == "" ||
		c.Platform != "github" || c.WindowDays != 140 || c.PeriodDays != 7 || c.MaxPeriods != 20 ||
		c.DisplayDecimalPlaces != 1 || c.ScoreMetricCount != 12 || len(c.Categories) != 5 {
		return invalid("configuration metadata")
	}
	if len(c.SupportingDocuments) != 3 {
		return invalid("supporting documents")
	}
	documents := map[string]bool{}
	for _, name := range c.SupportingDocuments {
		if !domain.ValidEvidencePath(name) || strings.Contains(name, "/") || documents[name] {
			return invalid("supporting document name")
		}
		documents[name] = true
	}
	expected := map[string]map[string]string{
		"activity":      {"ACT_COMMIT_FREQ": "log", "ACT_DAYS_SINCE_LAST": "decay", "ACT_TREND": "trend"},
		"security":      {"SEC_SECRETS": "secret", "SEC_VULN_RISK": "decay", "SEC_SAST": "boolean"},
		"documentation": {"DOC_README": "boolean", "DOC_SUPPORT_SHARE": "share"},
		"testing":       {"TEST_PRESENCE": "boolean"},
		"ci_cd":         {"CI_PRESENT": "boolean", "CI_SUCCESS_RATE": "share", "CI_REQUIRED_CHECKS": "boolean"},
	}
	seen := map[string]bool{}
	categoryWeightSum := 0.0
	for _, category := range c.Categories {
		metrics, ok := expected[category.ID]
		if !ok || seen[category.ID] || !positive(category.Weight) || len(category.Metrics) != len(metrics) {
			return invalid("category configuration")
		}
		seen[category.ID] = true
		categoryWeightSum += category.Weight
		found := map[string]bool{}
		var sum float64
		for _, metric := range category.Metrics {
			normalization, known := metrics[metric.ID]
			if !known || found[metric.ID] || normalization != metric.Normalization || !positive(metric.Weight) {
				return invalid("metric configuration")
			}
			found[metric.ID] = true
			sum += metric.Weight
			p := metric.Parameters
			if (metric.Normalization != "log" && p.Max != nil) ||
				(metric.Normalization != "decay" && metric.Normalization != "trend" && p.K != nil) ||
				(metric.Normalization != "trend" && p.M0 != nil) ||
				(metric.ID != "SEC_VULN_RISK" && len(p.SeverityWeights) != 0) {
				return invalid("unexpected normalization parameter")
			}
			switch metric.Normalization {
			case "log":
				if p.Max == nil || !positive(*p.Max) {
					return invalid("log max")
				}
			case "decay":
				if p.K == nil || !positive(*p.K) {
					return invalid("decay k")
				}
			case "trend":
				if p.K == nil || !positive(*p.K) || *p.K <= 1 || p.M0 == nil || *p.M0 <= 0 || *p.M0 >= 100 {
					return invalid("trend parameters")
				}
			}
			if metric.ID == "SEC_VULN_RISK" {
				if len(p.SeverityWeights) != 4 {
					return invalid("severity weights")
				}
				for _, severity := range []string{"critical", "high", "medium", "low"} {
					if !positive(p.SeverityWeights[severity]) {
						return invalid("severity weight")
					}
				}
			}
		}
		if !positive(sum) {
			return invalid("metric weight sum")
		}
	}
	if !positive(categoryWeightSum) || len(c.FindingThresholds) != 5 {
		return invalid("category weights/thresholds")
	}
	for _, id := range []string{"ACT_COMMIT_FREQ", "ACT_DAYS_SINCE_LAST", "ACT_TREND", "DOC_SUPPORT_SHARE", "CI_SUCCESS_RATE"} {
		threshold, ok := c.FindingThresholds[id]
		if !ok || threshold < 0 || threshold > 100 || math.IsNaN(threshold) {
			return invalid("finding threshold")
		}
	}
	return nil
}

func (e *Engine) checkVersions(versions domain.Versions) error {
	if e == nil || versions.Methodology != e.config.MethodologyVersion || versions.ScoringConfig != e.config.ScoringConfigVersion || versions.CollectorProfile != e.config.CollectorProfileVersion {
		return invalid("snapshot/configuration version mismatch")
	}
	return nil
}
