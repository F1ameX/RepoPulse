package scoring

import (
	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	"testing"
)

func TestSecretScores(t *testing.T) {
	e, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		counts []int
		score  float64
	}{{[]int{0, 0}, 100}, {[]int{0, 12}, 50}, {[]int{1, 12}, 0}} {
		input := domain.MetricInput{ID: "SEC_SECRETS", Unit: "secret_scan", Status: "complete", Series: []int{}, Sample: domain.Sample{Numerator: new(tc.counts[0]), Denominator: new(tc.counts[0] + tc.counts[1])}, Evidence: []domain.Evidence{}, Exclusions: []domain.ExcludedCount{}}
		input.RawValue = scalar(float64(tc.counts[0] + tc.counts[1]))
		inputs := []domain.MetricInput{input}
		for _, id := range []string{"SEC_VULN_RISK", "SEC_SAST"} {
			inputs = append(inputs, domain.MetricInput{ID: id, Unit: "risk", Status: "unavailable", Reason: reason("NO_DATA", "Нет данных"), Series: []int{}, Evidence: []domain.Evidence{}, Exclusions: []domain.ExcludedCount{}})
		}
		category, err := e.SecretsCategory(versions(), inputs)
		if err != nil {
			t.Fatal(err)
		}
		if category.Score == nil || *category.Score != tc.score || category.Metrics[0].NormalizedScore == nil || *category.Metrics[0].NormalizedScore != tc.score {
			t.Fatalf("%+v", category)
		}
	}
}
