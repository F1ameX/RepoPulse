package ci

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

func successInput(a domain.Analysis, source domain.SourceState, attempts []Attempt) (domain.SourceState, domain.MetricInput) {
	input := metricInput("CI_SUCCESS_RATE", "ratio", source)
	input.Window = &a.Window
	input.Sample = source.Sample
	if source.Status != "complete" && source.Status != "partial" {
		return source, input
	}

	type attemptKey struct {
		run    uint64
		number int
	}
	seen := make(map[attemptKey]Attempt, len(attempts))
	conflicting := make(map[attemptKey]bool)
	invalid := false
	for _, attempt := range attempts {
		if attempt.RunID == 0 || attempt.Number < 1 || attempt.CreatedAt.IsZero() ||
			(!attempt.CompletedAt.IsZero() && attempt.CompletedAt.Before(attempt.CreatedAt)) {
			invalid = true
			continue
		}
		if attempt.CreatedAt.Before(a.Window.From) || !attempt.CreatedAt.Before(a.Window.Until) ||
			attempt.CompletedAt.IsZero() || !attempt.CompletedAt.Before(a.Window.Until) {
			continue
		}
		key := attemptKey{attempt.RunID, attempt.Number}
		if previous, exists := seen[key]; exists && (previous.Conclusion != attempt.Conclusion ||
			!previous.CreatedAt.Equal(attempt.CreatedAt) || !previous.CompletedAt.Equal(attempt.CompletedAt)) {
			conflicting[key] = true
			invalid = true
		}
		seen[key] = attempt
	}
	latest := make(map[uint64]Attempt, len(seen))
	for key, attempt := range seen {
		if attempt.Number > latest[key.run].Number {
			latest[key.run] = attempt
		}
	}
	ids := make([]uint64, 0, len(latest))
	for id := range latest {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	numerator, denominator, evidenceTotal := 0, 0, 0
	exclusions := map[string]int{}
	for _, id := range ids {
		attempt := latest[id]
		if conflicting[attemptKey{id, attempt.Number}] {
			continue
		}
		switch attempt.Conclusion {
		case "success":
			numerator++
			denominator++
		case "failure", "timed_out", "action_required", "startup_failure", "stale":
			denominator++
		case "cancelled", "skipped", "neutral":
			exclusions[attempt.Conclusion]++
		default:
			invalid = true
			continue
		}
		evidenceTotal++
		if len(input.Evidence) < evidenceLimit {
			runID := strconv.FormatUint(id, 10)
			input.Evidence = append(input.Evidence, domain.Evidence{
				ID: fmt.Sprintf("CI_SUCCESS_RATE:%s:%d", runID, attempt.Number), Source: "github", Kind: "ci_run",
				Description: "Последняя завершённая попытка GitHub Actions: " + attempt.Conclusion,
				RunID:       &runID, Attempt: &attempt.Number,
			})
		}
	}
	excluded := 0
	for _, conclusion := range []string{"cancelled", "skipped", "neutral"} {
		if count := exclusions[conclusion]; count > 0 {
			input.Exclusions = append(input.Exclusions, domain.ExcludedCount{Reason: conclusion, Count: count})
			excluded += count
		}
	}
	input.Sample.Numerator, input.Sample.Denominator, input.Sample.Excluded = &numerator, &denominator, &excluded
	input.EvidenceLimited = evidenceTotal > len(input.Evidence)
	if invalid && source.Status == "complete" {
		source.Status, source.Fingerprint, source.Sample.Total = "partial", nil, nil
		source.Reason = &domain.Reason{Code: "CI_ACTIONS_INVALID", Message: "В истории GitHub Actions есть некорректные или противоречивые записи",
			CannotConclude: "Нельзя подтвердить долю успехов для полной выборки"}
		input.Sample.Total = nil
		input.Status, input.Reason = source.Status, source.Reason
	}
	if input.Status == "complete" {
		input.EvidenceTotal = &evidenceTotal
		if denominator > 0 {
			input.RawValue = scalar(strconv.FormatFloat(float64(numerator)/float64(denominator), 'g', -1, 64))
		} else {
			input.Status = "not_applicable"
			input.Reason = &domain.Reason{Code: "CI_NO_ELIGIBLE_RUNS", Message: "В окне нет учитываемых завершённых запусков GitHub Actions",
				CannotConclude: "Доля успешных запусков не определена"}
		}
	}
	return source, input
}
