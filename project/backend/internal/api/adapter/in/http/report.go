package httpapi

import (
	"context"
	"errors"
	"math"
	"net/http"
	"time"

	models "github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	apidomain "github.com/F1ameX/RepoPulse/project/backend/internal/api/domain"
)

type ReportService interface {
	GetReport(context.Context, models.VerifiedContext, string) (models.Report, error)
}

func getReport(reports ReportService, resolver ContextResolver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		verified, err := resolveAnalysisContext(r, resolver)
		if err == nil {
			err = verified.Validate(time.Now())
		}
		if err != nil {
			writeAnalysisError(w, r, err)
			return
		}
		if reports == nil {
			notImplemented(w, r)
			return
		}
		report, err := reports.GetReport(r.Context(), verified, r.PathValue("report_id"))
		if errors.Is(err, models.ErrNotFound) {
			writeError(w, 404, "REPORT_NOT_FOUND", "Отчёт недоступен")
			return
		}
		if errors.Is(err, apidomain.ErrReportUnavailable) {
			writeError(w, http.StatusServiceUnavailable, "INTERNAL_ERROR", "Не удалось загрузить отчёт")
			return
		}
		if err != nil {
			writeAnalysisError(w, r, err)
			return
		}
		// Storage retains the unrounded values. Scores and coverage at the public JSON
		// boundary use the methodology's half-up rounding to one decimal place.
		report.OverallScore = publicScore(report.OverallScore)
		report.Coverage.Percentage = publicNumber(report.Coverage.Percentage)
		for i := range report.Categories {
			category := &report.Categories[i]
			category.Score = publicScore(category.Score)
			category.Coverage.Percentage = publicNumber(category.Coverage.Percentage)
			for j := range category.Metrics {
				metric := &category.Metrics[j]
				metric.NormalizedScore = publicScore(metric.NormalizedScore)
			}
		}
		writeJSON(w, http.StatusOK, report)
	}
}

func publicScore(value *float64) *float64 {
	if value == nil {
		return nil
	}
	rounded := publicNumber(*value)
	return &rounded
}

func publicNumber(value float64) float64 { return math.Round(value*10) / 10 }
