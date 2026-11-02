package scoring

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

func engine(t *testing.T) *Engine {
	t.Helper()
	e, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func versions() domain.Versions {
	return domain.Versions{Schema: "mvp-contract-1.1.0", Methodology: "rhs-mvp-1.1.0", ScoringConfig: "rhs-mvp-1.1.0", CollectorProfile: "github-mvp-1.1.0", SecretRules: "secrets-mvp-1.0.0", RecommendationCatalog: "recommendations-mvp-1.0.0", ExplanationSchema: "catalog-selection-1.0.0", Prompt: "catalog-selection-1.0.0"}
}

var assessment = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

func window() domain.Window {
	return domain.Window{From: assessment.Add(-140 * 24 * time.Hour), Until: assessment}
}
func completeData(count int, periods ...int) ActivityData {
	return ActivityData{History: domain.SourceState{Source: "git", Scope: "history", Status: "complete"}, WindowCommits: count, LastCommitAt: pointer(assessment.Add(-24 * time.Hour)), Periods: periods}
}
func closeNumber(t *testing.T, actual *float64, want float64) {
	t.Helper()
	if actual == nil || math.Abs(*actual-want) > 1e-10 {
		t.Fatalf("got %v want %.15g", actual, want)
	}
}
func rawNumber(t *testing.T, input domain.MetricInput) *float64 {
	t.Helper()
	raw, err := json.Marshal(input.RawValue)
	if err != nil {
		t.Fatal(err)
	}
	var number *float64
	if err := json.Unmarshal(raw, &number); err != nil {
		t.Fatal(err)
	}
	return number
}

// These are the unmodified published FR-17 fixtures from document #933 v1.0.1.
// Their synthetic SHA labels are not Git object IDs; this adapter prepares the
// aggregate facts that the real collector (#18490) must prove independently.
func TestPublishedActivityFixtures(t *testing.T) {
	paths, err := filepath.Glob("testdata/ACT-*.json")
	if err != nil || len(paths) != 7 {
		t.Fatalf("fixtures: %v %d", err, len(paths))
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			encoded, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var fixture struct {
				Input struct {
					At            time.Time `json:"assessment_time"`
					HistoryStatus string    `json:"history_status"`
					HistoryReason string    `json:"history_reason"`
					Commits       []struct {
						At   time.Time `json:"committer_date"`
						Main bool      `json:"is_main"`
					} `json:"commits"`
					Periods []struct {
						Commits int `json:"commits"`
					} `json:"periods"`
				} `json:"input"`
				Expected struct {
					Metrics map[string]struct {
						Raw    *float64 `json:"raw_value"`
						Score  *float64 `json:"normalized_score"`
						Status string   `json:"status"`
						Reason *string  `json:"reason"`
					} `json:"metrics"`
				} `json:"expected"`
			}
			if err := json.Unmarshal(encoded, &fixture); err != nil {
				t.Fatal(err)
			}
			data := ActivityData{History: domain.SourceState{Source: "git", Scope: "history", Status: fixture.Input.HistoryStatus}}
			if fixture.Input.HistoryReason != "" {
				data.History.Reason = reason(fixture.Input.HistoryReason, "Обход истории прерван")
			}
			var oldest *time.Time
			w := domain.Window{From: fixture.Input.At.Add(-140 * 24 * time.Hour), Until: fixture.Input.At}
			for _, commit := range fixture.Input.Commits {
				if !commit.Main {
					continue
				}
				if oldest == nil || commit.At.Before(*oldest) {
					oldest = pointer(commit.At)
				}
				if data.LastCommitAt == nil || commit.At.After(*data.LastCommitAt) {
					data.LastCommitAt = pointer(commit.At)
				}
				if !commit.At.Before(w.From) && commit.At.Before(w.Until) {
					data.WindowCommits++
				}
			}
			if len(fixture.Input.Periods) > 0 {
				for _, period := range fixture.Input.Periods {
					data.Periods = append(data.Periods, period.Commits)
					data.WindowCommits += period.Commits
				}
				// ACT-04 specifies only the already collected full periods, not timestamps.
				data.LastCommitAt = pointer(fixture.Input.At.Add(-24 * time.Hour))
			} else if oldest != nil {
				count := min(20, int(fixture.Input.At.Sub(*oldest)/(7*24*time.Hour)))
				data.Periods = make([]int, count)
				start := fixture.Input.At.Add(-time.Duration(count) * 7 * 24 * time.Hour)
				for _, commit := range fixture.Input.Commits {
					if commit.Main && !commit.At.Before(start) && commit.At.Before(fixture.Input.At) {
						i := int(commit.At.Sub(start) / (7 * 24 * time.Hour))
						data.Periods[i]++
					}
				}
			}
			// ACT-06 expects a complete last-commit metric despite partial history.
			// This requires a separate collector guarantee; never infer it from one row.
			data.LastCommitConfirmed = fixture.Input.HistoryStatus == "partial"
			category, err := engine(t).Activity(fixture.Input.At, w, versions(), data)
			if err != nil {
				t.Fatal(err)
			}
			for _, metric := range category.Metrics {
				expected, ok := fixture.Expected.Metrics[metric.Input.ID]
				if !ok {
					continue
				}
				if metric.Input.Status != expected.Status {
					t.Fatalf("%s state %s want %s", metric.Input.ID, metric.Input.Status, expected.Status)
				}
				raw := rawNumber(t, metric.Input)
				if expected.Raw == nil {
					if raw != nil {
						t.Fatalf("%s raw should be null", metric.Input.ID)
					}
				} else {
					closeNumber(t, raw, *expected.Raw)
				}
				if expected.Score == nil {
					if metric.NormalizedScore != nil {
						t.Fatalf("%s score should be null", metric.Input.ID)
					}
				} else {
					closeNumber(t, metric.NormalizedScore, *expected.Score)
				}
				if expected.Reason != nil && (metric.Input.Reason == nil || metric.Input.Reason.Code != *expected.Reason) {
					t.Fatalf("%s reason: %+v", metric.Input.ID, metric.Input.Reason)
				}
			}
		})
	}
}

func TestActivityFormulaLimitsAndUnroundedCategory(t *testing.T) {
	e := engine(t)
	for _, test := range []struct {
		name             string
		data             ActivityData
		frequency, trend float64
	}{
		{"zero trend", completeData(10, 10, 0), 100 * math.Log1p(.5) / math.Log(11), 0},
		{"stable", completeData(20, 10, 10), 100 * math.Log(2) / math.Log(11), 80},
		{"growth 20 percent", completeData(22, 10, 12), 100 * math.Log1p(1.1) / math.Log(11), 96.727343654},
		{"frequency saturates", completeData(300, 100, 200), 100, 99.99},
	} {
		t.Run(test.name, func(t *testing.T) {
			category, err := e.Activity(assessment, window(), versions(), test.data)
			if err != nil {
				t.Fatal(err)
			}
			closeNumber(t, category.Metrics[0].NormalizedScore, test.frequency)
			if test.name != "frequency saturates" && test.name != "growth 20 percent" {
				closeNumber(t, category.Metrics[2].NormalizedScore, test.trend)
			}
			if test.name == "growth 20 percent" {
				score := *category.Metrics[2].NormalizedScore
				if score <= 96.7 || score >= 96.8 {
					t.Fatal(score)
				}
			}
			want := .35**category.Metrics[0].NormalizedScore + .35**category.Metrics[1].NormalizedScore + .3**category.Metrics[2].NormalizedScore
			closeNumber(t, category.Score, want)
			if category.Status != "complete" || category.Coverage.Calculated != 3 || category.Coverage.Percentage != 100 || category.EffectiveWeight != nil {
				t.Fatalf("%+v", category)
			}
			for _, metric := range category.Metrics {
				if metric.RuleVersion != versions().Methodology || metric.Rule == "" || metric.Rule == metric.Input.ID {
					t.Fatal("version lost")
				}
			}
		})
	}
}

func TestPartialHistoryAndUnknownRemainUnscored(t *testing.T) {
	e := engine(t)
	data := completeData(10, 5, 5)
	data.History.Status = "partial"
	data.History.Reason = reason("GIT_COMMIT_LIMIT", "Лимит коммитов")
	category, err := e.Activity(assessment, window(), versions(), data)
	if err != nil {
		t.Fatal(err)
	}
	if category.Score != nil || category.Coverage.Calculated != 0 || category.Status != "unavailable" {
		t.Fatalf("partial history scored: %+v", category)
	}
	for _, metric := range category.Metrics {
		if metric.NormalizedScore != nil || rawNumber(t, metric.Input) != nil || metric.Input.Reason.Code != "GIT_COMMIT_LIMIT" {
			t.Fatal(metric)
		}
	}
	data.LastCommitConfirmed = true
	category, err = e.Activity(assessment, window(), versions(), data)
	if err != nil {
		t.Fatal(err)
	}
	closeNumber(t, category.Score, 100*math.Exp(-1.0/30))
	if category.Status != "partial" || category.Coverage.Calculated != 1 || category.Coverage.Partial != 1 || category.Coverage.Unavailable != 1 {
		t.Fatalf("%+v", category)
	}
	data.History.Status = "unavailable"
	data.LastCommitConfirmed = false
	data.LastCommitAt = nil
	category, err = e.Activity(assessment, window(), versions(), data)
	if err != nil || category.Score != nil {
		t.Fatalf("%+v %v", category, err)
	}
}

func TestEmptyHistoryAndZeroFrequencyAreDifferent(t *testing.T) {
	e := engine(t)
	empty, err := e.Activity(assessment, window(), versions(), ActivityData{History: domain.SourceState{Source: "git", Scope: "history", Status: "complete"}})
	if err != nil || empty.Score != nil || empty.Status != "not_applicable" || empty.Coverage.NotApplicable != 3 {
		t.Fatalf("%+v %v", empty, err)
	}
	data := completeData(0, 0, 0)
	data.LastCommitAt = pointer(assessment.Add(-150 * 24 * time.Hour))
	category, err := e.Activity(assessment, window(), versions(), data)
	if err != nil {
		t.Fatal(err)
	}
	closeNumber(t, category.Metrics[0].NormalizedScore, 0)
	if category.Coverage.Calculated != 2 {
		t.Fatal(category.Coverage)
	}
	// Zero is a real value in the renormalized denominator, rather than excluded N/A.
	closeNumber(t, category.Score, 100*math.Exp(-5)/2)
}

func TestUTCAndExactDayFloor(t *testing.T) {
	e := engine(t)
	for _, hours := range []time.Duration{0, 23*time.Hour + 59*time.Minute, 24 * time.Hour, 30 * 24 * time.Hour} {
		data := completeData(1, 0, 1)
		// The end-exclusive committer at exactly t is valid for age, but not frequency.
		if hours == 0 {
			data.WindowCommits = 0
			data.Periods = []int{}
		}
		data.LastCommitAt = pointer(assessment.Add(-hours).In(time.FixedZone("+07", 7*3600)))
		category, err := e.Activity(assessment, window(), versions(), data)
		if err != nil {
			t.Fatal(err)
		}
		closeNumber(t, rawNumber(t, category.Metrics[1].Input), math.Floor(hours.Hours()/24))
		if category.Metrics[1].Input.ObservedAt.Location() != time.UTC {
			t.Fatal("not UTC")
		}
	}
}

func TestRejectInvalidActivityData(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*ActivityData)
	}{
		{"negative count", func(d *ActivityData) { d.WindowCommits = -1 }},
		{"negative period", func(d *ActivityData) { d.Periods = []int{-1, 2} }},
		{"too many periods", func(d *ActivityData) { d.Periods = make([]int, 21) }},
		{"future commit", func(d *ActivityData) { d.LastCommitAt = pointer(assessment.Add(time.Nanosecond)) }},
		{"unknown date", func(d *ActivityData) { d.LastCommitAt = pointer(time.Time{}) }},
		{"unproven empty", func(d *ActivityData) { d.LastCommitAt = nil }},
		{"period count exceeds window", func(d *ActivityData) { d.Periods = []int{10, 10} }},
		{"last predates nonempty window", func(d *ActivityData) { d.LastCommitAt = pointer(assessment.Add(-141 * 24 * time.Hour)) }},
		{"missing partial reason", func(d *ActivityData) { d.History.Status = "partial" }},
		{"unknown history state", func(d *ActivityData) { d.History.Status = "not_applicable" }},
		{"invalid source", func(d *ActivityData) { d.History.Source = "github" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := completeData(10, 5, 5)
			test.change(&data)
			if _, err := engine(t).Activity(assessment, window(), versions(), data); !errors.Is(err, domain.ErrInvalidData) {
				t.Fatalf("%v", err)
			}
		})
	}
}

func TestSavedSnapshotReplayAndOwnership(t *testing.T) {
	e := engine(t)
	category, err := e.Activity(assessment, window(), versions(), completeData(60, 10, 20, 30))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := savedSnapshot(t, category)
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := domain.DecodeSnapshot(encoded)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := e.ReplayActivity(saved)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(category, replayed) {
		t.Fatalf("replay changed category\n%+v\n%+v", category, replayed)
	}
	replayed.Metrics[2].Input.Series[0] = 999
	*replayed.Metrics[0].Input.Sample.Numerator = 999
	if saved.Inputs[2].Series[0] != 10 || *saved.Inputs[0].Sample.Numerator != 60 {
		t.Fatal("replay mutated snapshot")
	}
	saved.Versions.ScoringConfig = "other"
	if _, err := e.ReplayActivity(saved); !errors.Is(err, domain.ErrInvalidData) {
		t.Fatal(err)
	}
}

func savedSnapshot(t *testing.T, category domain.Category) domain.Snapshot {
	t.Helper()
	encoded, err := os.ReadFile("../../../../../../contracts/examples/snapshot.json")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := domain.DecodeSnapshot(encoded)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.AssessmentAt = assessment
	snapshot.Window = window()
	snapshot.Versions = versions()
	byID := map[string]domain.MetricInput{}
	for _, metric := range category.Metrics {
		byID[metric.Input.ID] = metric.Input
	}
	for i := range snapshot.Inputs {
		if input, ok := byID[snapshot.Inputs[i].ID]; ok {
			snapshot.Inputs[i] = input
		}
	}
	return snapshot
}

func TestRejectInconsistentSavedInputs(t *testing.T) {
	e := engine(t)
	category, err := e.Activity(assessment, window(), versions(), completeData(60, 10, 20, 30))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		change func([]domain.MetricInput)
	}{
		{"negative raw", func(in []domain.MetricInput) { in[0].RawValue = scalar(-1) }},
		{"null complete", func(in []domain.MetricInput) { in[0].RawValue = domain.Scalar{} }},
		{"boolean raw", func(in []domain.MetricInput) { _ = in[0].RawValue.UnmarshalJSON([]byte("false")) }},
		{"string raw", func(in []domain.MetricInput) { _ = in[0].RawValue.UnmarshalJSON([]byte(`"7"`)) }},
		{"wrong frequency", func(in []domain.MetricInput) { in[0].RawValue = scalar(100) }},
		{"wrong denominator", func(in []domain.MetricInput) { in[0].Sample.Denominator = pointer(19) }},
		{"wrong trend", func(in []domain.MetricInput) { in[2].Series = []int{10, 10, 10} }},
		{"missing observed", func(in []domain.MetricInput) { in[1].ObservedAt = nil }},
		{"wrong age", func(in []domain.MetricInput) { in[1].RawValue = scalar(30) }},
		{"duplicate", func(in []domain.MetricInput) { in[1] = in[0] }},
		{"other metric", func(in []domain.MetricInput) { in[1].ID = "DOC_README" }},
		{"unit", func(in []domain.MetricInput) { in[0].Unit = "days" }},
		{"window", func(in []domain.MetricInput) { in[0].Window = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			encoded, _ := json.Marshal(category.Metrics)
			var metrics []domain.Metric
			_ = json.Unmarshal(encoded, &metrics)
			inputs := []domain.MetricInput{metrics[0].Input, metrics[1].Input, metrics[2].Input}
			test.change(inputs)
			if _, err := e.ActivityFromInputs(assessment, window(), versions(), inputs); !errors.Is(err, domain.ErrInvalidData) {
				t.Fatal(err)
			}
		})
	}
	if _, err := e.ActivityFromInputs(assessment, window(), versions(), nil); !errors.Is(err, domain.ErrInvalidData) {
		t.Fatal(err)
	}
}

func TestPublicScoreOnlyRoundsAtBoundary(t *testing.T) {
	for _, test := range []struct{ in, want float64 }{{0, 0}, {1.25, 1.3}, {96.727343, 96.7}, {99.95, 100}} {
		got, err := PublicScore(&test.in)
		if err != nil {
			t.Fatal(err)
		}
		closeNumber(t, got, test.want)
	}
	if got, err := PublicScore(nil); got != nil || err != nil {
		t.Fatal(got, err)
	}
	for _, value := range []float64{-1, 101, math.NaN(), math.Inf(1)} {
		if _, err := PublicScore(&value); !errors.Is(err, domain.ErrInvalidData) {
			t.Fatal(err)
		}
	}
}

func TestVersionAndWindowMismatch(t *testing.T) {
	e := engine(t)
	for _, field := range []string{"methodology", "scoring", "collector"} {
		v := versions()
		switch field {
		case "methodology":
			v.Methodology = "other"
		case "scoring":
			v.ScoringConfig = "other"
		case "collector":
			v.CollectorProfile = "other"
		}
		if _, err := e.Activity(assessment, window(), v, completeData(10, 5, 5)); !errors.Is(err, domain.ErrInvalidData) {
			t.Fatal(err)
		}
	}
	for _, w := range []domain.Window{{From: window().From.Add(time.Second), Until: assessment}, {From: window().From, Until: assessment.Add(time.Second)}} {
		if _, err := e.Activity(assessment, w, versions(), completeData(10, 5, 5)); !errors.Is(err, domain.ErrInvalidData) {
			t.Fatal(err)
		}
	}
	var absent *Engine
	if _, err := absent.Activity(assessment, window(), versions(), completeData(10, 5, 5)); !errors.Is(err, domain.ErrInvalidData) {
		t.Fatal(err)
	}
}

func TestConfigurationIsDecodedAndChecked(t *testing.T) {
	for _, test := range []struct {
		name     string
		from, to string
	}{
		{"missing weight", `"weight":0.35`, `"unused_weight":0.35`},
		{"zero weight", `"weight":0.35`, `"weight":0`},
		{"negative category", `"weight":3`, `"weight":-3`},
		{"missing max", `"max":10`, `"max":null`},
		{"negative decay", `"k":30`, `"k":-30`},
		{"bad trend k", `"k":1.2`, `"k":1`},
		{"bad trend M0", `"M0":80`, `"M0":100`},
		{"unknown id", `"ACT_COMMIT_FREQ"`, `"ACT_UNKNOWN"`},
		{"duplicate id", `"ACT_DAYS_SINCE_LAST"`, `"ACT_COMMIT_FREQ"`},
		{"duplicate category", `"security"`, `"activity"`},
		{"missing severity", `"critical":10,`, ` `},
		{"missing version", `"scoring_config_version":"rhs-mvp-1.1.0"`, `"scoring_config_version":""`},
		{"wrong count", `"score_metric_count":12`, `"score_metric_count":13`},
		{"wrong window", `"window_days": 140`, `"window_days": 139`},
		{"missing threshold", `"ACT_COMMIT_FREQ":50,`, ``},
	} {
		t.Run(test.name, func(t *testing.T) {
			changed := strings.Replace(string(defaultConfig), test.from, test.to, 1)
			if changed == string(defaultConfig) {
				t.Fatal("bad mutation")
			}
			if _, err := Load(strings.NewReader(changed)); !errors.Is(err, domain.ErrInvalidData) {
				t.Fatal(err)
			}
		})
	}
	for _, value := range []string{"null", "{}", string(defaultConfig) + "{}", string(defaultConfig) + "garbage"} {
		if _, err := Load(strings.NewReader(value)); !errors.Is(err, domain.ErrInvalidData) {
			t.Fatal(err)
		}
	}
	// Prove parameters/weights are read from the supplied version, not hardcoded.
	changed := strings.Replace(string(defaultConfig), `"max":10`, `"max":20`, 1)
	changed = strings.ReplaceAll(changed, "rhs-mvp-1.1.0", "rhs-test-2")
	custom, err := Load(strings.NewReader(changed))
	if err != nil {
		t.Fatal(err)
	}
	v := versions()
	v.Methodology = "rhs-test-2"
	v.ScoringConfig = "rhs-test-2"
	category, err := custom.Activity(assessment, window(), v, completeData(200, 100, 100))
	if err != nil {
		t.Fatal(err)
	}
	closeNumber(t, category.Metrics[0].NormalizedScore, 100*math.Log(11)/math.Log(21))
}

func TestTrendApplicabilityAndCounterOverflow(t *testing.T) {
	for _, test := range []struct {
		name   string
		counts []int
		reason string
	}{{"one full week", []int{5}, "INSUFFICIENT_PERIODS"}, {"zero denominator", []int{0, 5}, "ZERO_DENOMINATOR"}, {"no full weeks", nil, "INSUFFICIENT_PERIODS"}} {
		t.Run(test.name, func(t *testing.T) {
			category, err := engine(t).Activity(assessment, window(), versions(), completeData(5, test.counts...))
			if err != nil {
				t.Fatal(err)
			}
			trend := category.Metrics[2]
			if trend.NormalizedScore != nil || trend.Input.Status != "not_applicable" || trend.Input.Reason.Code != test.reason {
				t.Fatalf("%+v", trend)
			}
		})
	}
	if _, _, err := trendCounts([]int{0, math.MaxInt, math.MaxInt}); !errors.Is(err, domain.ErrInvalidData) {
		t.Fatal(err)
	}
}

func TestAvailableWeightsComeFromVersionedConfig(t *testing.T) {
	changed := strings.Replace(string(defaultConfig), `"weight":0.35`, `"weight":0.7`, 1)
	changed = strings.ReplaceAll(changed, "rhs-mvp-1.1.0", "rhs-test-weights")
	e, err := Load(strings.NewReader(changed))
	if err != nil {
		t.Fatal(err)
	}
	v := versions()
	v.Methodology = "rhs-test-weights"
	v.ScoringConfig = "rhs-test-weights"
	data := completeData(0, 0, 0)
	data.LastCommitAt = pointer(assessment.Add(-30 * 24 * time.Hour))
	category, err := e.Activity(assessment, window(), v, data)
	if err != nil {
		t.Fatal(err)
	}
	closeNumber(t, category.Score, (.35*100/math.E)/(.7+.35))
	// Relative weights remain valid at large scales; intermediate products cannot overflow.
	changed = strings.ReplaceAll(string(defaultConfig), `"weight":0.35`, `"weight":3e307`)
	changed = strings.Replace(changed, `"weight":0.3,`, `"weight":3e307,`, 1)
	e, err = Load(strings.NewReader(changed))
	if err != nil {
		t.Fatal(err)
	}
	category, err = e.Activity(assessment, window(), versions(), completeData(20, 10, 10))
	if err != nil {
		t.Fatal(err)
	}
	closeNumber(t, category.Score, (*category.Metrics[0].NormalizedScore+*category.Metrics[1].NormalizedScore+80)/3)
}
