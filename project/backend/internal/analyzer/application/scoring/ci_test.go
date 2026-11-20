package scoring

import (
	"slices"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/application/ci"
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

func TestCIMetricsFR23(t *testing.T) {
	engine, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	a := domain.Analysis{AssessmentAt: at, Window: domain.Window{From: at.Add(-140 * 24 * time.Hour), Until: at}}
	complete := domain.SourceState{Status: "complete"}
	partial := domain.SourceState{Status: "partial", Reason: &domain.Reason{
		Code: "PAGE_LIMIT", Message: "Получена часть данных", CannotConclude: "Полнота неизвестна",
	}}
	workflow := []domain.GitFile{{Path: ".github/workflows/ci.yaml", Mode: "100644"}}
	attempt := func(id uint64, n int, outcome string, created time.Time) ci.Attempt {
		return ci.Attempt{RunID: id, Number: n, CreatedAt: created, CompletedAt: created.Add(time.Minute), Conclusion: outcome}
	}
	base := []ci.Attempt{
		attempt(1, 1, "failure", a.Window.From),
		attempt(1, 2, "success", a.Window.From),
		{RunID: 1, Number: 3, CreatedAt: a.Window.From},
		attempt(20, 1, "failure", a.Window.From.Add(-time.Second)),
		attempt(21, 1, "failure", at),
		{RunID: 1, Number: 4, CreatedAt: a.Window.From, CompletedAt: at, Conclusion: "failure"},
	}
	base = append(base, base[1]) // identical duplicate is counted once
	for i, conclusion := range []string{"success", "success", "success", "success", "success", "failure", "failure", "cancelled", "cancelled"} {
		base = append(base, attempt(uint64(i+2), 1, conclusion, a.Window.From.Add(time.Hour)))
	}
	failures := []ci.Attempt{}
	for i, conclusion := range []string{"failure", "timed_out", "action_required", "startup_failure", "stale", "cancelled", "skipped", "neutral"} {
		failures = append(failures, attempt(uint64(i+1), 1, conclusion, a.Window.From))
	}
	for _, tc := range []struct {
		name       string
		tree       domain.SourceState
		files      []domain.GitFile
		external   ci.ExternalData
		raw        [3]string
		scores     [3]*float64
		rateReason string
		num, denom *int
		exclusions []domain.ExcludedCount
	}{
		{name: "6 successes of 8 eligible runs", tree: complete, files: workflow,
			external: ci.ExternalData{Actions: complete, Attempts: base, Rules: complete, RequiredStatusChecks: 2},
			raw:      [3]string{"true", "0.75", "true"}, scores: [3]*float64{new(100.0), new(75.0), new(100.0)},
			num: new(6), denom: new(8), exclusions: []domain.ExcludedCount{{Reason: "cancelled", Count: 2}}},
		{name: "all recognized failures, review only", tree: complete, files: workflow,
			external: ci.ExternalData{Actions: complete, Attempts: failures, Rules: complete},
			raw:      [3]string{"true", "0", "false"}, scores: [3]*float64{new(100.0), new(0.0), new(0.0)},
			num: new(0), denom: new(5), exclusions: []domain.ExcludedCount{{Reason: "cancelled", Count: 1}, {Reason: "skipped", Count: 1}, {Reason: "neutral", Count: 1}}},
		{name: "recognize external CI", tree: complete,
			files: []domain.GitFile{{Path: ".gitlab-ci.yml", Mode: "100644"}, {Path: "Jenkinsfile", Mode: "100755"},
				{Path: ".circleci/config.yml", Mode: "100644"}, {Path: "azure-pipelines.yml", Mode: "100644"}},
			raw: [3]string{"true", "null", "null"}, scores: [3]*float64{new(100.0)},
			rateReason: "CI_EXTERNAL"},
		{name: "no supported configuration", tree: complete,
			files: []domain.GitFile{{Path: ".github/workflows/link.yml", Mode: "120000"},
				{Path: ".github/workflows/nested/ci.yml", Mode: "100644"}, {Path: "other/Jenkinsfile", Mode: "100644"}},
			raw: [3]string{"false", "null", "null"}, scores: [3]*float64{new(0.0)},
			rateReason: "CI_NOT_CONFIGURED"},
		{name: "no eligible runs", tree: complete, files: workflow,
			external: ci.ExternalData{Actions: complete, Attempts: failures[5:]},
			raw:      [3]string{"true", "null", "null"}, scores: [3]*float64{new(100.0)},
			rateReason: "CI_NO_ELIGIBLE_RUNS", num: new(0), denom: new(0),
			exclusions: []domain.ExcludedCount{{Reason: "cancelled", Count: 1}, {Reason: "skipped", Count: 1}, {Reason: "neutral", Count: 1}}},
		{name: "github unavailable", tree: complete, files: workflow, raw: [3]string{"true", "null", "null"},
			scores: [3]*float64{new(100.0)}, rateReason: "CI_ACTIONS_UNAVAILABLE"},
		{name: "independent complete actions", tree: partial, files: workflow,
			external: ci.ExternalData{Actions: complete, Attempts: base, Rules: partial, RequiredStatusChecks: 1},
			raw:      [3]string{"null", "0.75", "null"}, scores: [3]*float64{nil, new(75.0)}, num: new(6), denom: new(8),
			exclusions: []domain.ExcludedCount{{Reason: "cancelled", Count: 2}}},
		{name: "interrupted actions", tree: complete, files: workflow,
			external: ci.ExternalData{Actions: partial, Attempts: base, Rules: partial},
			raw:      [3]string{"true", "null", "null"}, scores: [3]*float64{new(100.0)},
			rateReason: "PAGE_LIMIT", num: new(6), denom: new(8), exclusions: []domain.ExcludedCount{{Reason: "cancelled", Count: 2}}},
		{name: "conflicting latest attempt", tree: complete, files: workflow,
			external: ci.ExternalData{Actions: complete, Attempts: []ci.Attempt{
				attempt(1, 1, "success", a.Window.From), attempt(1, 2, "failure", a.Window.From), attempt(1, 2, "success", a.Window.From),
			}},
			raw: [3]string{"true", "null", "null"}, scores: [3]*float64{new(100.0)},
			rateReason: "CI_ACTIONS_INVALID", num: new(0), denom: new(0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := ci.Inputs(a, domain.GitData{Files: tc.files}, tc.tree, tc.external)
			var inputs []domain.MetricInput
			for _, input := range result.Inputs {
				inputs = append(inputs, input.Value)
			}
			category, err := engine.CICategory(versions(), inputs)
			if err != nil {
				t.Fatal(err)
			}
			for i, metric := range category.Metrics {
				raw, _ := metric.Input.RawValue.MarshalJSON()
				if string(raw) != tc.raw[i] || !equalOptional(metric.NormalizedScore, tc.scores[i]) ||
					(tc.scores[i] == nil && metric.Input.Reason == nil) {
					t.Fatalf("%s: raw %s, score %v, reason %+v", metric.Input.ID, raw, metric.NormalizedScore, metric.Input.Reason)
				}
			}
			rate := category.Metrics[1].Input
			if (tc.rateReason != "" && (rate.Reason == nil || rate.Reason.Code != tc.rateReason)) ||
				!equalOptional(rate.Sample.Numerator, tc.num) || !equalOptional(rate.Sample.Denominator, tc.denom) ||
				!slices.Equal(rate.Exclusions, tc.exclusions) {
				t.Fatalf("unexpected Actions sample: %+v", rate)
			}
			if tc.name == "6 successes of 8 eligible runs" {
				first := rate.Evidence[0]
				if first.RunID == nil || *first.RunID != "1" || first.Attempt == nil || *first.Attempt != 2 ||
					category.Score == nil || *category.Score != 90 || category.Status != "complete" {
					t.Fatalf("wrong attempt evidence or category result: %+v, %+v", first, category)
				}
			}
		})
	}
}

func equalOptional[T comparable](got, want *T) bool {
	return got == nil && want == nil || got != nil && want != nil && *got == *want
}
