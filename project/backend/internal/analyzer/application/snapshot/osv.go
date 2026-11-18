package snapshot

import (
	"context"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type OSVQuerier interface {
	Query(context.Context, []domain.OSVQuery) []domain.OSVResult
}

func collectOSV(ctx context.Context, client OSVQuerier, collections []Collection) (Collection, error) {
	var items []domain.Dependency
	hasDependencies, incomplete := false, false
	for _, collection := range collections {
		items = append(items, collection.Dependencies...)
		for _, source := range collection.Sources {
			if source.Scope == "dependencies" {
				hasDependencies = true
				incomplete = incomplete || source.Status == "partial" || source.Status == "unavailable"
			}
		}
	}
	if !hasDependencies && len(items) == 0 {
		// No dependency collector has been installed for this processor.
		return Collection{}, nil
	}
	deps, err := NormalizeDependencies(items)
	if err != nil || len(deps) > 20 {
		return Collection{}, domain.ErrInvalidData
	}
	source := domain.SourceState{Source: "osv", Scope: domain.OSVScope}
	if len(deps) == 0 {
		if incomplete {
			source.Status = "unavailable"
			source.Reason = osvReason("OSV_DEPENDENCIES_UNAVAILABLE", "Список конкретных версий зависимостей недоступен")
		} else {
			source.Status = "not_applicable"
			source.Reason = osvReason("OSV_NO_DEPENDENCIES", "Нет конкретных версий зависимостей для запроса OSV")
		}
		return Collection{Sources: []domain.SourceState{source}}, nil
	}
	queries := make([]domain.OSVQuery, 0, len(deps))
	for _, dep := range deps {
		queries = append(queries, domain.OSVQuery{Ecosystem: dep.Ecosystem, Name: dep.Package, Version: dep.Version})
	}
	// Let a slow independent source end before the core deadline so already
	// collected sources can still be assembled and used by the calculator.
	queryCtx := ctx
	if deadline, ok := ctx.Deadline(); ok {
		var cancel context.CancelFunc
		queryCtx, cancel = context.WithDeadline(ctx, deadline.Add(-5*time.Second))
		defer cancel()
	}
	results := client.Query(queryCtx, queries)
	if len(results) != len(queries) {
		return Collection{}, domain.ErrInvalidData
	}
	var collected Collection
	var latest time.Time
	count := 0
	code := ""
	for i, result := range results {
		if result.Query != queries[i] || (len(result.Pages) == 0 && result.ReasonCode == "") {
			return Collection{}, domain.ErrInvalidData
		}
		if result.ReasonCode == "" {
			count++
		} else if code == "" {
			code = result.ReasonCode
		}
		for _, page := range result.Pages {
			if page.ObtainedAt.After(latest) {
				latest = page.ObtainedAt
			}
			for _, id := range page.VulnerabilityIDs {
				collected.Vulnerabilities = append(collected.Vulnerabilities, domain.Vulnerability{
					ID: id, Ecosystem: result.Query.Ecosystem, Package: result.Query.Name,
					Version: result.Query.Version, ObtainedAt: page.ObtainedAt,
				})
			}
		}
	}
	if !latest.IsZero() {
		source.ObtainedAt = &latest
	}
	source.Sample.Collected = &count // dependencies whose query reached the last page
	if code == "" && (incomplete || !hasDependencies) {
		code = "OSV_DEPENDENCY_COVERAGE_INCOMPLETE"
	}
	switch {
	case code == "":
		source.Status = "complete"
		total := len(queries)
		source.Sample.Total = &total
	case !latest.IsZero():
		source.Status = "partial"
		source.Reason = osvReason(code, "Получены не все данные OSV")
	default:
		source.Status = "unavailable"
		source.Reason = osvReason(code, "Данные OSV не получены")
	}
	collected.Sources = []domain.SourceState{source}
	return collected, nil
}

func osvReason(code, message string) *domain.Reason {
	return &domain.Reason{Code: code, Message: message,
		CannotConclude: "Нельзя подтвердить отсутствие уязвимостей по непроверенным зависимостям"}
}
