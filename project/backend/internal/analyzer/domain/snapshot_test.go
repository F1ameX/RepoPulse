package domain

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func fixtureSnapshot(t *testing.T) Snapshot {
	t.Helper()
	data, err := os.ReadFile("../../../../../contracts/examples/snapshot.json")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := DecodeSnapshot(data)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func fixtureReport(t *testing.T) Report {
	t.Helper()
	data, err := os.ReadFile("../../../../../contracts/examples/report-partial.json")
	if err != nil {
		t.Fatal(err)
	}
	var report Report
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	return report
}

func runningAnalysis(s Snapshot) Analysis {
	started := s.AssessmentAt
	return Analysis{ID: s.AnalysisID, SubmissionID: NewID(), RequestID: NewID(), Owner: s.Owner,
		Repository: s.Repository, AssessmentAt: s.AssessmentAt, Window: s.Window, Versions: s.Versions,
		Status: "running", CreatedAt: s.AssessmentAt, StartedAt: &started}
}

func TestSnapshotContractRoundTripPreservesUnknownZeroFalse(t *testing.T) {
	s := fixtureSnapshot(t)
	if err := json.Unmarshal([]byte("false"), &s.Inputs[3].RawValue); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeSnapshot(data)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range map[int]string{0: "null", 3: "false", 4: "0"} {
		got, _ := json.Marshal(decoded.Inputs[i].RawValue)
		if string(got) != want {
			t.Fatalf("metric %d: got %s, want %s", i, got, want)
		}
	}
	for _, field := range []string{`"numerator":null`, `"observed_at":null`, `"refs":[]`, `"collector_profile":"github-mvp-1.1.0"`} {
		if !strings.Contains(string(data), field) {
			t.Fatalf("missing %s", field)
		}
	}
	if len(decoded.Dependencies) != 3 || decoded.Dependencies[1].Ecosystem != "Maven" {
		t.Fatal("dependency profiles lost")
	}
}

func TestScalarCopiesDoNotChangeAnEarlierSnapshot(t *testing.T) {
	var original Scalar
	if err := json.Unmarshal([]byte("true"), &original); err != nil {
		t.Fatal(err)
	}
	copied := original
	if err := json.Unmarshal([]byte("null"), &copied); err != nil {
		t.Fatal(err)
	}
	got, _ := json.Marshal(original)
	if string(got) != "true" {
		t.Fatal("changing a scalar copy corrupted the saved input")
	}
}

func TestSnapshotRejectsUnsafeAndInconsistentData(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Snapshot)
	}{
		{"duplicate metric", func(s *Snapshot) { s.Inputs[1] = s.Inputs[0] }},
		{"missing reason", func(s *Snapshot) { s.Inputs[0].Reason = nil }},
		{"credential URL", func(s *Snapshot) {
			s.Repository.CanonicalURL = "https://user:password@github.com/example/repopulse-fixture"
		}},
		{"missing profile", func(s *Snapshot) { s.Versions.CollectorProfile = "" }},
		{"unknown as zero", func(s *Snapshot) { _ = json.Unmarshal([]byte("0"), &s.Inputs[0].RawValue) }},
		{"raw secret", func(s *Snapshot) {
			s.Inputs[6].Status = "complete"
			_ = json.Unmarshal([]byte(`"password=secret"`), &s.Inputs[6].RawValue)
		}},
		{"path escape", func(s *Snapshot) { s.Dependencies[0].Origins[0].ManifestPath = "../package-lock.json" }},
		{"duplicate dependency", func(s *Snapshot) { s.Dependencies = append(s.Dependencies, s.Dependencies[0]) }},
		{"missing origin", func(s *Snapshot) { s.Dependencies[0].Origins = nil }},
		{"orphan vulnerability", func(s *Snapshot) {
			s.Vulnerabilities = []Vulnerability{{ID: "OSV-1", Ecosystem: "Go", Package: "missing", Version: "v1", ObtainedAt: time.Now()}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := fixtureSnapshot(t)
			tc.change(&s)
			if err := s.Validate(); !errors.Is(err, ErrInvalidData) {
				t.Fatalf("got %v", err)
			}
		})
	}
	s := fixtureSnapshot(t)
	encoded, _ := json.Marshal(s)
	unsafe := strings.Replace(string(encoded), `"snapshot_id":`, `"password":"raw-value","snapshot_id":`, 1)
	if _, err := DecodeSnapshot([]byte(unsafe)); !errors.Is(err, ErrInvalidData) {
		t.Fatalf("unknown field accepted: %v", err)
	}
	for _, raw := range []string{`{"secret":"value"}`, `["raw file"]`} {
		var value Scalar
		if err := json.Unmarshal([]byte(raw), &value); err == nil {
			t.Fatal("non-scalar accepted")
		}
	}
}

func TestNewResultFreezesMetadataAndSanitizesSecretEvidence(t *testing.T) {
	s := fixtureSnapshot(t)
	report := fixtureReport(t)
	kind := "suspicious_assignment"
	file := "config.go"
	e := Evidence{ID: "secret-1", Source: "git", Kind: "secret_match", Description: "password=do-not-store", Path: &file, SecretMatchType: &kind}
	s.Inputs[6].Evidence = []Evidence{e}
	one := 1
	s.Inputs[6].EvidenceTotal = &one
	// Report must retain the exact same normalized input as the snapshot.
	for i := range report.Categories {
		for j := range report.Categories[i].Metrics {
			if report.Categories[i].Metrics[j].Input.ID == "SEC_SECRETS" {
				report.Categories[i].Metrics[j].Input = s.Inputs[6]
			}
		}
	}
	a := runningAnalysis(s)
	result, err := NewResult(a, NewID(), NewID(), a.AssessmentAt.Add(time.Second), s.SnapshotData, report.ReportData)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(result)
	if strings.Contains(string(raw), "do-not-store") {
		t.Fatal("raw secret persisted")
	}
	if s.Inputs[6].Evidence[0].Description != e.Description {
		t.Fatal("caller-owned evidence mutated")
	}
	if result.Snapshot.Owner != a.Owner || result.Report.AnalysisID != a.ID || result.Report.Versions != a.Versions {
		t.Fatal("metadata changed")
	}
	result.Snapshot.Owner.ID = NewID()
	if !errors.Is(result.Validate(a), ErrInvalidData) {
		t.Fatal("owner replacement accepted")
	}
}
