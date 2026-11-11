package scoring

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

// ActivityData is supplied by the history collector (#18490), not by HTTP.
// WindowCommits includes merges/bots in the main branch, deduplicated by SHA.
// Periods contains only full seven-day periods, oldest to newest, ending at t;
// its length is min(20, full periods of repository age). Empty is distinct from
// twenty zero counters. LastCommitConfirmed permits an independently proven
// last commit even when the window/history traversal was interrupted.
type ActivityData struct {
	History             domain.SourceState
	WindowCommits       int
	LastCommitAt        *time.Time
	LastCommitConfirmed bool
	Periods             []int
}

// Activity creates the three snapshot-ready inputs and scores their category.
// The collector owns filtering, period boundaries and proof of completeness.
func (e *Engine) Activity(at time.Time, window domain.Window, versions domain.Versions, data ActivityData) (domain.Category, error) {
	if err := e.checkContext(at, window, versions); err != nil {
		return domain.Category{}, err
	}
	if data.History.Source != "git" || data.History.Scope != "history" || data.WindowCommits < 0 || len(data.Periods) > e.config.MaxPeriods ||
		(data.LastCommitConfirmed && data.LastCommitAt == nil) {
		return domain.Category{}, invalid("activity facts")
	}
	periodSum := 0
	for _, count := range data.Periods {
		if count < 0 || count > math.MaxInt-periodSum {
			return domain.Category{}, invalid("negative period count")
		}
		periodSum += count
	}
	if periodSum > data.WindowCommits {
		return domain.Category{}, invalid("periods exceed window commits")
	}
	if data.History.Status == "complete" && data.WindowCommits > 0 && data.LastCommitAt != nil && data.LastCommitAt.Before(window.From) {
		return domain.Category{}, invalid("last commit predates window containing commits")
	}
	if data.LastCommitAt != nil && (data.LastCommitAt.IsZero() || data.LastCommitAt.After(at)) {
		return domain.Category{}, invalid("last commit date")
	}
	if data.History.Status == "complete" && data.LastCommitAt == nil && (data.WindowCommits != 0 || len(data.Periods) != 0) {
		return domain.Category{}, invalid("empty history with counters")
	}
	if data.History.Status != "complete" && data.History.Status != "partial" && data.History.Status != "unavailable" {
		return domain.Category{}, invalid("history state")
	}
	inputs := make([]domain.MetricInput, 0, 3)
	for _, spec := range []struct{ id, unit string }{{"ACT_COMMIT_FREQ", "commits_per_week"}, {"ACT_DAYS_SINCE_LAST", "days"}, {"ACT_TREND", "ratio"}} {
		input := domain.MetricInput{ID: spec.id, Unit: spec.unit, Status: data.History.Status, Reason: data.History.Reason, Window: &window,
			Series: []int{}, Evidence: []domain.Evidence{}, Exclusions: []domain.ExcludedCount{}}
		if err := input.Validate(); err != nil {
			return domain.Category{}, err
		}
		inputs = append(inputs, input)
	}
	freq, last, trend := &inputs[0], &inputs[1], &inputs[2]
	freq.Sample.Collected = pointer(data.WindowCommits)
	if data.History.Status == "complete" {
		freq.Sample.Total = pointer(data.WindowCommits)
		if data.LastCommitAt == nil {
			unavailable(freq, "not_applicable", "NO_COMMITS_AT_ALL", "В истории нет коммитов")
		} else {
			freq.RawValue = scalar(float64(data.WindowCommits) / float64(e.config.MaxPeriods))
			freq.Sample.Numerator = pointer(data.WindowCommits)
			freq.Sample.Denominator = pointer(e.config.MaxPeriods)
		}
	}
	if data.LastCommitAt != nil {
		observed := data.LastCommitAt.UTC()
		last.ObservedAt = &observed
	}
	if data.LastCommitAt != nil && (data.History.Status == "complete" || data.LastCommitConfirmed) {
		last.Status, last.Reason = "complete", nil
		last.RawValue = scalar(math.Floor(at.Sub(*data.LastCommitAt).Hours() / 24))
		last.Sample = domain.Sample{Collected: pointer(1), Total: pointer(1)}
	} else if data.History.Status == "complete" {
		unavailable(last, "not_applicable", "NO_COMMITS_AT_ALL", "В истории нет коммитов")
	}
	trend.Series = append([]int{}, data.Periods...)
	trend.Sample.Collected = pointer(len(data.Periods))
	if data.History.Status != "complete" {
		// A partial series does not establish a full trend. Preserve the source reason.
		trend.Status = "unavailable"
	} else {
		trend.Sample.Total = pointer(len(data.Periods))
		switch {
		case data.WindowCommits == 0:
			unavailable(trend, "not_applicable", "NO_COMMITS_IN_WINDOW", "В окне нет коммитов")
		case len(data.Periods) < 2:
			unavailable(trend, "not_applicable", "INSUFFICIENT_PERIODS", "Меньше двух полных недельных периодов")
		default:
			numerator, denominator, err := trendCounts(data.Periods)
			if err != nil {
				return domain.Category{}, err
			}
			trend.Sample.Numerator, trend.Sample.Denominator = pointer(numerator), pointer(denominator)
			if denominator == 0 {
				unavailable(trend, "not_applicable", "ZERO_DENOMINATOR", "Знаменатель коэффициента тренда равен нулю")
			} else {
				trend.RawValue = scalar(float64(numerator) / float64(denominator))
			}
		}
	}
	return e.ActivityFromInputs(at, window, versions, inputs)
}

// ReplayActivity uses only the saved inputs, time/window and pinned versions.
// No clock, Git reader or network access is consulted during replay.
func (e *Engine) ReplayActivity(snapshot domain.Snapshot) (domain.Category, error) {
	if err := snapshot.Validate(); err != nil {
		return domain.Category{}, err
	}
	inputs := make([]domain.MetricInput, 0, 3)
	for _, input := range snapshot.Inputs {
		if input.ID == "ACT_COMMIT_FREQ" || input.ID == "ACT_DAYS_SINCE_LAST" || input.ID == "ACT_TREND" {
			inputs = append(inputs, input)
		}
	}
	return e.ActivityFromInputs(snapshot.AssessmentAt, snapshot.Window, snapshot.Versions, inputs)
}

// ActivityFromInputs is the integration boundary for already normalized snapshot
// inputs from #18490/#18491. It validates sufficient statistics against raw_value.
// A partial/unavailable input never contributes a score to this category.
func (e *Engine) ActivityFromInputs(at time.Time, window domain.Window, versions domain.Versions, inputs []domain.MetricInput) (domain.Category, error) {
	if err := e.checkContext(at, window, versions); err != nil {
		return domain.Category{}, err
	}
	if len(inputs) != 3 {
		return domain.Category{}, invalid("activity input count")
	}
	// Results own their metadata/slices; replay cannot mutate the saved snapshot.
	encoded, err := json.Marshal(inputs)
	if err != nil {
		return domain.Category{}, invalid("activity JSON")
	}
	var copied []domain.MetricInput
	if json.Unmarshal(encoded, &copied) != nil {
		return domain.Category{}, invalid("activity JSON")
	}
	byID := map[string]domain.MetricInput{}
	for _, input := range copied {
		if _, exists := byID[input.ID]; exists {
			return domain.Category{}, invalid("duplicate activity input")
		}
		if err := input.Validate(); err != nil {
			return domain.Category{}, err
		}
		byID[input.ID] = input
	}
	var cfg categoryConfig
	for _, category := range e.config.Categories {
		if category.ID == "activity" {
			cfg = category
			break
		}
	}
	metrics := make([]domain.Metric, 0, len(cfg.Metrics))
	for _, spec := range cfg.Metrics {
		input, ok := byID[spec.ID]
		if !ok {
			return domain.Category{}, invalid("missing activity input")
		}
		score, err := e.normalizeActivity(at, window, input, spec)
		if err != nil {
			return domain.Category{}, err
		}
		threshold := e.config.FindingThresholds[spec.ID]
		metric := domain.Metric{Input: input, NormalizedScore: score, RuleVersion: e.config.MethodologyVersion, Rule: e.formula(spec), Threshold: &threshold, Explanation: "Баллы рассчитаны по методике №909 без промежуточного округления."}
		if score == nil {
			metric.Explanation = "Численная оценка отсутствует: см. полноту и причину входа."
		}
		metrics = append(metrics, metric)
	}
	return summarizeCategory(cfg, metrics, "активности", "Оценка наблюдаемой активности основной ветки; не гарантирует качество кода."), nil
}

func (e *Engine) checkContext(at time.Time, window domain.Window, versions domain.Versions) error {
	if err := e.checkVersions(versions); err != nil {
		return err
	}
	if at.IsZero() || !window.Until.Equal(at) || !window.From.Equal(at.Add(-time.Duration(e.config.WindowDays)*24*time.Hour)) {
		return invalid("activity window")
	}
	return nil
}

func (e *Engine) normalizeActivity(at time.Time, window domain.Window, input domain.MetricInput, spec metricConfig) (*float64, error) {
	if input.Window == nil || !input.Window.From.Equal(window.From) || !input.Window.Until.Equal(window.Until) {
		return nil, invalid("metric window mismatch")
	}
	units := map[string]string{"ACT_COMMIT_FREQ": "commits_per_week", "ACT_DAYS_SINCE_LAST": "days", "ACT_TREND": "ratio"}
	if input.Unit != units[input.ID] {
		return nil, invalid("activity unit")
	}
	raw, _ := input.RawValue.MarshalJSON()
	var value *float64
	if !bytes.Equal(raw, []byte("null")) {
		var number float64
		if json.Unmarshal(raw, &number) != nil || number < 0 || math.IsInf(number, 0) || math.IsNaN(number) {
			return nil, invalid("activity raw_value")
		}
		value = &number
	}
	if input.Sample.Collected != nil && input.Sample.Total != nil && *input.Sample.Collected > *input.Sample.Total {
		return nil, invalid("collected count exceeds total")
	}
	if len(input.Series) > e.config.MaxPeriods {
		return nil, invalid("period count")
	}
	if input.ObservedAt != nil && (input.ObservedAt.IsZero() || input.ObservedAt.After(at)) {
		return nil, invalid("observed commit date")
	}
	if input.Status != "complete" {
		return nil, nil
	}
	if value == nil {
		return nil, invalid("complete activity has null raw_value")
	}
	var result float64
	switch input.ID {
	case "ACT_COMMIT_FREQ":
		if input.Sample.Numerator == nil || input.Sample.Denominator == nil || *input.Sample.Denominator != e.config.MaxPeriods ||
			*value != float64(*input.Sample.Numerator)/float64(e.config.MaxPeriods) {
			return nil, invalid("frequency sample/raw_value mismatch")
		}
		result = 100 * math.Min(1, math.Log1p(*value)/math.Log1p(*spec.Parameters.Max))
	case "ACT_DAYS_SINCE_LAST":
		if input.ObservedAt == nil || *value != math.Floor(at.Sub(*input.ObservedAt).Hours()/24) {
			return nil, invalid("age/observed_at mismatch")
		}
		result = 100 * math.Exp(-*value / *spec.Parameters.K)
	case "ACT_TREND":
		if len(input.Series) < 2 {
			return nil, invalid("complete trend needs full periods")
		}
		numerator, denominator, err := trendCounts(input.Series)
		if err != nil {
			return nil, err
		}
		if denominator == 0 || *value != float64(numerator)/float64(denominator) || input.Sample.Numerator == nil || input.Sample.Denominator == nil ||
			*input.Sample.Numerator != numerator || *input.Sample.Denominator != denominator {
			return nil, invalid("trend series/raw_value mismatch")
		}
		if *value == 0 {
			result = 0
		} else {
			result = 50 + 50*math.Tanh(math.Log(*value)/math.Log(*spec.Parameters.K)+0.5*math.Log(*spec.Parameters.M0/(100-*spec.Parameters.M0)))
		}
	}
	return &result, nil
}

// The public contract stores the applied formula and actual configuration parameters.
func (e *Engine) formula(spec metricConfig) string {
	switch spec.Normalization {
	case "log":
		return fmt.Sprintf("x=commits/%d; 100*min(1,ln(1+x)/ln(1+%g))", e.config.MaxPeriods, *spec.Parameters.Max)
	case "decay":
		return fmt.Sprintf("days=floor((t-observed_at)/24h); 100*exp(-days/%g)", *spec.Parameters.K)
	case "trend":
		return fmt.Sprintf("R=sum(i*C[i+1])/sum(i*C[i]), i=1..p-1; R=0: 0; R>0: 50+50*tanh(ln(R)/ln(%g)+0.5*ln(%g/(100-%g)))", *spec.Parameters.K, *spec.Parameters.M0, *spec.Parameters.M0)
	}
	return ""
}

func trendCounts(series []int) (int, int, error) {
	numerator, denominator := 0, 0
	for i := 1; i < len(series); i++ {
		if series[i] < 0 || series[i-1] < 0 || series[i] > (math.MaxInt-numerator)/i || series[i-1] > (math.MaxInt-denominator)/i {
			return 0, 0, invalid("trend counters overflow")
		}
		numerator += i * series[i]
		denominator += i * series[i-1]
	}
	return numerator, denominator, nil
}
func reason(code, message string) *domain.Reason {
	return &domain.Reason{Code: code, Message: message, CannotConclude: "Нельзя делать полный вывод по отсутствующим данным"}
}
func unavailable(input *domain.MetricInput, status, code, message string) {
	input.Status = status
	input.Reason = reason(code, message)
	input.RawValue = domain.Scalar{}
}
func pointer[T any](value T) *T { return &value }
func scalar(value float64) domain.Scalar {
	var result domain.Scalar
	_ = result.UnmarshalJSON([]byte(strconv.FormatFloat(value, 'g', -1, 64)))
	return result
}

// PublicScore rounds only at the presentation boundary, never before weighting.
// Scores are nonnegative; math.Round therefore implements decimal half-up at 1dp.
func PublicScore(score *float64) (*float64, error) {
	if score == nil {
		return nil, nil
	}
	if *score < 0 || *score > 100 || math.IsNaN(*score) || math.IsInf(*score, 0) {
		return nil, invalid("public score")
	}
	return pointer(math.Round(*score*10) / 10), nil
}
