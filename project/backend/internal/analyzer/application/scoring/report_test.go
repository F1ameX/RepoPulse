package scoring

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

// Manual reference from the Redmine scoring methodology: trend ratio 11/10,
// two of three documents, one critical and one high advisory, six successful
// out of eight counted runs. Expected public Score: 71.6, coverage: 12/12.
func scoreSnapshot(t *testing.T, e *Engine) domain.Snapshot {
	t.Helper()
	facts := completeData(100, 10, 11)
	facts.LastCommitAt = pointer(assessment.Add(-7 * 24 * time.Hour))
	activity, err := e.Activity(assessment, window(), versions(), facts)
	if err != nil {
		t.Fatal(err)
	}
	s := savedSnapshot(t, activity)
	for i := range s.Inputs {
		m := &s.Inputs[i]
		var raw string
		switch m.ID {
		case "DOC_README", "TEST_PRESENCE", "CI_PRESENT":
			m.Unit, raw = "bool", "true"
		case "SEC_SAST", "CI_REQUIRED_CHECKS":
			m.Unit, raw = "bool", "false"
		case "SEC_SECRETS":
			m.Unit, raw = "secret_scan", "50"
		case "SEC_VULN_RISK":
			m.Unit, raw = "risk", "15"
		case "CI_SUCCESS_RATE":
			m.Unit, raw, m.Window = "ratio", "0.75", &s.Window
			m.Sample.Numerator, m.Sample.Denominator = pointer(6), pointer(8)
		case "DOC_SUPPORT_SHARE":
			m.Unit, raw = "ratio", "0.6666666666666666"
			m.Sample.Numerator, m.Sample.Denominator = pointer(2), pointer(3)
		default:
			continue
		}
		m.Status, m.Reason = "complete", nil
		if err := m.RawValue.UnmarshalJSON([]byte(raw)); err != nil {
			t.Fatal(err)
		}
	}
	for i, severity := range []string{"critical", "high"} {
		d := s.Dependencies[i]
		s.Vulnerabilities = append(s.Vulnerabilities, domain.Vulnerability{ID: "TEST-1", Package: d.Package,
			Ecosystem: d.Ecosystem, Version: d.Version, ObtainedAt: assessment, Severity: pointer(severity)})
	}
	return s
}

func TestFullScoreAndReplay(t *testing.T) {
	e := engine(t)
	s := scoreSnapshot(t, e)
	r, err := e.Replay(t.Context(), s)
	if err != nil {
		t.Fatal(err)
	}
	if r.OverallScore == nil || math.Round(*r.OverallScore*10)/10 != 71.6 || r.OverallReason != nil ||
		r.Coverage != (domain.Coverage{Total: 12, Calculated: 12, Percentage: 100}) {
		t.Fatalf("score %v, coverage %+v", r.OverallScore, r.Coverage)
	}
	for _, c := range r.Categories {
		w := 2.0
		if c.ID == "security" || c.ID == "activity" {
			w = 3
		}
		closeNumber(t, c.EffectiveWeight, w/12)
		for _, m := range c.Metrics {
			if m.Rule == "" || m.RuleVersion != s.Versions.Methodology {
				t.Fatalf("missing provenance for %s", m.Input.ID)
			}
		}
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := domain.DecodeSnapshot(b)
	if err != nil {
		t.Fatal(err)
	}
	again, err := engine(t).Replay(t.Context(), saved)
	if err != nil || !reflect.DeepEqual(again, r) {
		t.Fatalf("replay changed saved result: %v", err)
	}
	saved.Versions.ScoringConfig += "-new"
	if _, err := e.Replay(t.Context(), saved); !errors.Is(err, domain.ErrInvalidData) {
		t.Fatalf("unexpected configuration accepted: %v", err)
	}
}

func TestPartialSecurityAndMissingCategories(t *testing.T) {
	e := engine(t)
	s := scoreSnapshot(t, e)
	for i := range s.Inputs {
		unavailable(&s.Inputs[i], "not_applicable", "NO_DATA", "Нет данных")
	}
	r, err := e.Replay(t.Context(), s)
	if err != nil || r.OverallScore != nil || r.OverallReason == nil || r.Coverage.NotApplicable != 12 || r.Coverage.Total != 12 {
		t.Fatalf("all N/A: %+v, %v", r.Coverage, err)
	}
	for i := range s.Inputs {
		m := &s.Inputs[i]
		switch m.ID {
		case "DOC_README":
			m.Status, m.Reason = "complete", nil
			if err := m.RawValue.UnmarshalJSON([]byte("true")); err != nil {
				t.Fatal(err)
			}
		case "SEC_SECRETS":
			m.Status, m.Reason, m.RawValue = "complete", nil, scalar(0)
		case "SEC_VULN_RISK":
			m.Status, m.RawValue = "partial", scalar(15)
		}
	}
	r, err = e.Replay(t.Context(), s)
	if err != nil {
		t.Fatal(err)
	}
	closeNumber(t, r.OverallScore, 40)
	if r.Coverage.Calculated != 2 || r.Coverage.Partial != 1 || r.Coverage.Total != 12 {
		t.Fatalf("partial sample counted: %+v", r.Coverage)
	}
}

func TestAllCalculatedZeroIsNotMissing(t *testing.T) {
	e := engine(t)
	r := domain.ReportData{}
	for _, cfg := range e.config.Categories {
		n := len(cfg.Metrics)
		r.Categories = append(r.Categories, domain.Category{ID: cfg.ID, Score: pointer(0.0), Coverage: domain.Coverage{Total: n, Calculated: n}})
	}
	e.aggregate(&r)
	closeNumber(t, r.OverallScore, 0)
	if r.OverallReason != nil || r.Coverage.Calculated != 12 || r.Coverage.Percentage != 100 {
		t.Fatalf("all zero: %+v", r)
	}
}

func TestRejectInconsistentCompleteSecurityInputs(t *testing.T) {
	e := engine(t)
	for _, tc := range []struct{ id, raw string }{
		{"SEC_SECRETS", "25"}, {"SEC_VULN_RISK", "0"}, {"SEC_SAST", "0"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			s := scoreSnapshot(t, e)
			for i := range s.Inputs {
				if s.Inputs[i].ID == tc.id {
					if err := s.Inputs[i].RawValue.UnmarshalJSON([]byte(tc.raw)); err != nil {
						t.Fatal(err)
					}
				}
			}
			if _, err := e.Replay(t.Context(), s); !errors.Is(err, domain.ErrInvalidData) {
				t.Fatalf("inconsistent complete input accepted: %v", err)
			}
		})
	}
}
