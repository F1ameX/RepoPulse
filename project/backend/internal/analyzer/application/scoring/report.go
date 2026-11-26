package scoring

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

func (e *Engine) Replay(ctx context.Context, s domain.Snapshot) (domain.ReportData, error) {
	if err := s.Validate(); err != nil {
		return domain.ReportData{}, err
	}
	return e.Calculate(ctx, domain.Analysis{AssessmentAt: s.AssessmentAt, Window: s.Window, Versions: s.Versions}, s.SnapshotData)
}

// Calculate scores only normalized snapshot facts. Publication and replay both
// use the assessment window and configuration versions accepted for this analysis.
func (e *Engine) Calculate(ctx context.Context, a domain.Analysis, data domain.SnapshotData) (domain.ReportData, error) {
	if err := ctx.Err(); err != nil {
		return domain.ReportData{}, err
	}
	if err := data.Validate(); err != nil {
		return domain.ReportData{}, err
	}
	var activityInputs []domain.MetricInput
	for _, input := range data.Inputs {
		if strings.HasPrefix(input.ID, "ACT_") {
			activityInputs = append(activityInputs, input)
		}
	}
	activity, err := e.ActivityFromInputs(a.AssessmentAt, a.Window, a.Versions, activityInputs)
	if err != nil {
		return domain.ReportData{}, err
	}
	tree, err := e.TreeCategories(a.Versions, data.Inputs)
	if err != nil {
		return domain.ReportData{}, err
	}
	security, err := e.securityCategory(data)
	if err != nil {
		return domain.ReportData{}, err
	}
	ci, err := e.CICategory(a.Versions, data.Inputs)
	if err != nil {
		return domain.ReportData{}, err
	}
	r := domain.ReportData{
		Categories: append(append([]domain.Category{activity}, tree...), security, ci),
		Findings:   vulnerabilityFindings(data), Recommendations: []domain.Recommendation{},
		Explanation: domain.Explanation{Mode: "deterministic_fallback", FallbackReason: pointer("no_findings"),
			CatalogVersion: a.Versions.RecommendationCatalog, SchemaVersion: a.Versions.ExplanationSchema, PromptVersion: a.Versions.Prompt},
	}
	if len(r.Findings) > 0 {
		r.Explanation.FallbackReason = pointer("ai_not_configured")
	}
	e.aggregate(&r)
	return r, nil
}

func (e *Engine) aggregate(r *domain.ReportData) {
	config := make(map[string]categoryConfig, len(e.config.Categories))
	for _, c := range e.config.Categories {
		config[c.ID] = c
	}
	var weights float64
	for _, c := range r.Categories {
		if c.Score != nil {
			weights += config[c.ID].Weight
		}
		r.Coverage.Calculated += c.Coverage.Calculated
		r.Coverage.Total += c.Coverage.Total
		r.Coverage.Partial += c.Coverage.Partial
		r.Coverage.Unavailable += c.Coverage.Unavailable
		r.Coverage.NotApplicable += c.Coverage.NotApplicable
	}
	r.Coverage.Percentage = float64(r.Coverage.Calculated) * 100 / float64(r.Coverage.Total)
	if weights == 0 {
		r.OverallReason = reason("NO_DATA", "Нет данных для расчёта общей оценки")
		r.Summary = r.OverallReason.Message
		return
	}
	var overall, accumulated float64
	var factors []string
	for i := range r.Categories {
		c := &r.Categories[i]
		if c.Score == nil {
			continue
		}
		cfg := config[c.ID]
		c.EffectiveWeight = pointer(cfg.Weight / weights)
		accumulated += cfg.Weight
		overall += (*c.Score - overall) * (cfg.Weight / accumulated)
		contribution := *c.EffectiveWeight * *c.Score
		factors = append(factors, fmt.Sprintf("%s: %.1f", cfg.Name, math.Round(contribution*10)/10))
	}
	r.OverallScore = pointer(overall)
	r.Summary = "Вклад категорий в общую оценку, баллы: " + strings.Join(factors, "; ") + "."
}
