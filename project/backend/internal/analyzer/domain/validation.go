package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"
)

var metricCategories = map[string]string{
	"ACT_COMMIT_FREQ": "activity", "ACT_DAYS_SINCE_LAST": "activity", "ACT_TREND": "activity",
	"DOC_README": "documentation", "DOC_SUPPORT_SHARE": "documentation", "TEST_PRESENCE": "testing",
	"SEC_SECRETS": "security", "SEC_VULN_RISK": "security", "SEC_SAST": "security",
	"CI_PRESENT": "ci_cd", "CI_SUCCESS_RATE": "ci_cd", "CI_REQUIRED_CHECKS": "ci_cd",
}

func invalid(field string) error { return fmt.Errorf("%w: %s", ErrInvalidData, field) }

func validateState(status string, reason *Reason) error {
	switch status {
	case "complete":
		return nil
	case "partial", "unavailable", "not_applicable":
		if reason != nil && reason.Code != "" && reason.Message != "" && reason.CannotConclude != "" {
			return nil
		}
	}
	return invalid("completeness/reason")
}

func validateSample(s Sample) error {
	for _, n := range []*int{s.Collected, s.Total, s.Numerator, s.Denominator, s.Excluded} {
		if n != nil && *n < 0 {
			return invalid("sample")
		}
	}
	return nil
}

func safeURL(raw *string) bool {
	if raw == nil {
		return true
	}
	u, err := url.Parse(*raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil &&
		u.RawQuery == "" && u.Fragment == "" && !u.ForceQuery
}

func safePath(value string) bool {
	return value != "" && value != "." && !strings.ContainsAny(value, "\\\x00\r\n:") &&
		!strings.HasPrefix(value, "/") && value != ".." && !strings.HasPrefix(value, "../") &&
		path.Clean(value) == value
}

func dependencyKey(ecosystem, pkg, version string) string {
	return ecosystem + "\x00" + pkg + "\x00" + version
}

func (d Dependency) Validate() error {
	if (d.Ecosystem != "npm" && d.Ecosystem != "Maven" && d.Ecosystem != "Go") ||
		d.Package == "" || d.Version == "" || !safePath(d.ManifestPath) || d.VersionSource == "" || len(d.Origins) == 0 {
		return invalid("dependency")
	}
	found := false
	seen := map[DependencyOrigin]bool{}
	for _, origin := range d.Origins {
		if !safePath(origin.ManifestPath) || origin.VersionSource == "" || seen[origin] {
			return invalid("dependency origins")
		}
		seen[origin] = true
		found = found || (origin.ManifestPath == d.ManifestPath && origin.VersionSource == d.VersionSource)
	}
	if !found {
		return invalid("dependency primary origin")
	}
	return nil
}

func (m MetricInput) Validate() error {
	if metricCategories[m.ID] == "" || m.Unit == "" || m.Series == nil || m.Evidence == nil || m.Exclusions == nil {
		return invalid("metric")
	}
	if err := validateState(m.Status, m.Reason); err != nil {
		return err
	}
	if err := validateSample(m.Sample); err != nil {
		return err
	}
	raw, _ := m.RawValue.MarshalJSON()
	if (m.Status == "unavailable" || m.Status == "not_applicable") && !bytes.Equal(raw, []byte("null")) {
		return invalid("unavailable raw value")
	}
	if m.ID == "SEC_SECRETS" && len(raw) > 0 && raw[0] == '"' {
		return invalid("secret raw value must not contain text")
	}
	if m.Window != nil && !m.Window.From.Before(m.Window.Until) {
		return invalid("metric window")
	}
	if m.EvidenceTotal != nil && (*m.EvidenceTotal < len(m.Evidence) || *m.EvidenceTotal < 0) {
		return invalid("evidence count")
	}
	for _, n := range m.Series {
		if n < 0 {
			return invalid("series")
		}
	}
	for _, excluded := range m.Exclusions {
		if excluded.Count < 0 || excluded.Reason == "" {
			return invalid("exclusions")
		}
	}
	seen := map[string]bool{}
	for _, e := range m.Evidence {
		if e.ID == "" || seen[e.ID] || (e.Source != "git" && e.Source != "github" && e.Source != "osv") ||
			!safeURL(e.URL) || (e.Path != nil && !safePath(*e.Path)) ||
			(e.Revision != nil && !shaPattern.MatchString(*e.Revision)) ||
			(e.Attempt != nil && *e.Attempt < 1) {
			return invalid("evidence")
		}
		seen[e.ID] = true
		if e.SecretMatchType != nil && e.Kind != "secret_match" {
			return invalid("secret evidence kind")
		}
		if e.Kind == "secret_match" {
			if e.SecretMatchType == nil || (*e.SecretMatchType != "private_key_header" && *e.SecretMatchType != "suspicious_assignment") {
				return invalid("secret match type")
			}
			// Free-form secret descriptions cannot cross the persistence boundary.
			if e.Description != secretDescription(*e.SecretMatchType) {
				return invalid("secret description")
			}
		}
		if e.Dependency != nil {
			if err := e.Dependency.Validate(); err != nil {
				return err
			}
		}
	}
	return nil
}

func secretDescription(kind string) string { return "Potential secret: " + kind }

func (s Snapshot) Validate() error {
	if !ValidID(s.ID) || !ValidID(s.AnalysisID) || s.AssessmentAt.IsZero() ||
		!s.Window.Until.Equal(s.AssessmentAt) || !s.Window.From.Equal(s.AssessmentAt.Add(-140*24*time.Hour)) {
		return invalid("snapshot metadata")
	}
	if err := s.Owner.Validate(); err != nil {
		return err
	}
	if err := s.Repository.Validate(); err != nil {
		return err
	}
	if err := s.Versions.Validate(); err != nil {
		return err
	}
	return s.SnapshotData.Validate()
}

func (s SnapshotData) Validate() error {
	if len(s.Inputs) != len(metricCategories) || len(s.Dependencies) > 20 ||
		s.Refs == nil || s.Sources == nil || s.Dependencies == nil || s.Vulnerabilities == nil {
		return invalid("snapshot inputs")
	}
	refs := map[string]bool{}
	for _, ref := range s.Refs {
		if (!strings.HasPrefix(ref.Name, "refs/heads/") && !strings.HasPrefix(ref.Name, "refs/tags/")) ||
			!shaPattern.MatchString(ref.ObjectSHA) || refs[ref.Name] {
			return invalid("refs")
		}
		refs[ref.Name] = true
	}
	for _, source := range s.Sources {
		if (source.Source != "git" && source.Source != "github" && source.Source != "osv") || source.Scope == "" {
			return invalid("source")
		}
		if err := validateState(source.Status, source.Reason); err != nil {
			return err
		}
		if err := validateSample(source.Sample); err != nil {
			return err
		}
	}
	inputs := map[string]bool{}
	evidence := map[string]bool{}
	for _, input := range s.Inputs {
		if inputs[input.ID] {
			return invalid("duplicate metric")
		}
		inputs[input.ID] = true
		if err := input.Validate(); err != nil {
			return err
		}
		for _, item := range input.Evidence {
			if evidence[item.ID] {
				return invalid("duplicate evidence ID")
			}
			evidence[item.ID] = true
		}
	}
	dependencies := map[string]bool{}
	for _, d := range s.Dependencies {
		if err := d.Validate(); err != nil {
			return err
		}
		key := dependencyKey(d.Ecosystem, d.Package, d.Version)
		if dependencies[key] {
			return invalid("duplicate dependency")
		}
		dependencies[key] = true
	}
	vulnerabilities := map[string]bool{}
	for _, v := range s.Vulnerabilities {
		key := dependencyKey(v.Ecosystem, v.Package, v.Version)
		if !dependencies[key] || v.ID == "" || v.ObtainedAt.IsZero() || !safeURL(v.EvidenceURL) ||
			vulnerabilities[v.ID+"\x00"+key] {
			return invalid("vulnerability")
		}
		if v.Severity != nil {
			switch *v.Severity {
			case "critical", "high", "medium", "low", "None":
			default:
				return invalid("severity")
			}
		}
		vulnerabilities[v.ID+"\x00"+key] = true
	}
	return nil
}

func emptySlice[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}

func normalizeInput(m *MetricInput) {
	m.Series = emptySlice(m.Series)
	m.Evidence = emptySlice(m.Evidence)
	m.Exclusions = emptySlice(m.Exclusions)
	if m.Window != nil {
		m.Window.From, m.Window.Until = UTC(m.Window.From), UTC(m.Window.Until)
	}
	normalizeTime(&m.ObservedAt)
	for i := range m.Evidence {
		e := &m.Evidence[i]
		if e.Kind == "secret_match" && e.SecretMatchType != nil {
			e.Description = secretDescription(*e.SecretMatchType)
		}
	}
}

func normalizeTime(at **time.Time) {
	if *at != nil {
		value := UTC(**at)
		*at = &value
	}
}

// PrepareSnapshotData detaches collected facts from their producers and applies
// storage precision without inventing missing observations, counts or values.
func PrepareSnapshotData(data SnapshotData) (SnapshotData, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return SnapshotData{}, invalid("snapshot JSON")
	}
	var copied SnapshotData
	if err := json.Unmarshal(encoded, &copied); err != nil {
		return SnapshotData{}, invalid("snapshot JSON")
	}
	normalizeSnapshotData(&copied)
	if err := copied.Validate(); err != nil {
		return SnapshotData{}, err
	}
	return copied, nil
}

func normalizeSnapshotData(data *SnapshotData) {
	data.Refs, data.Sources = emptySlice(data.Refs), emptySlice(data.Sources)
	data.Dependencies, data.Vulnerabilities = emptySlice(data.Dependencies), emptySlice(data.Vulnerabilities)
	for i := range data.Sources {
		normalizeTime(&data.Sources[i].ObtainedAt)
	}
	for i := range data.Vulnerabilities {
		data.Vulnerabilities[i].ObtainedAt = UTC(data.Vulnerabilities[i].ObtainedAt)
	}
	for i := range data.Inputs {
		normalizeInput(&data.Inputs[i])
	}
}

// NewResult copies all caller-owned slices before normalization. Neither a worker
// nor a transport may replace accepted metadata when publishing the result.
func NewResult(a Analysis, snapshotID, reportID string, at time.Time, data SnapshotData, report ReportData) (Result, error) {
	if a.Status == "completed" || a.Status == "failed" {
		return Result{}, ErrAlreadyCompleted
	}
	if a.Status != "running" || a.StartedAt == nil {
		return Result{}, ErrInvalidTransition
	}
	at = UTC(at)
	if at.Before(*a.StartedAt) || (a.DeadlineAt != nil && !at.Before(*a.DeadlineAt)) {
		return Result{}, ErrInvalidTransition
	}
	var copied struct {
		Data   SnapshotData
		Report ReportData
	}
	encoded, err := json.Marshal(struct {
		Data   SnapshotData
		Report ReportData
	}{data, report})
	if err != nil {
		return Result{}, invalid("result JSON")
	}
	if err := json.Unmarshal(encoded, &copied); err != nil {
		return Result{}, invalid("result JSON")
	}
	data, report = copied.Data, copied.Report
	normalizeSnapshotData(&data)
	snapshot := Snapshot{ID: snapshotID, AnalysisID: a.ID, Owner: a.Owner, Repository: a.Repository,
		AssessmentAt: a.AssessmentAt, Window: a.Window, Versions: a.Versions, SnapshotData: data}
	if err := snapshot.Validate(); err != nil {
		return Result{}, err
	}
	report.Findings, report.Recommendations = emptySlice(report.Findings), emptySlice(report.Recommendations)
	for i := range report.Categories {
		for j := range report.Categories[i].Metrics {
			normalizeInput(&report.Categories[i].Metrics[j].Input)
		}
	}
	for i := range report.Findings {
		report.Findings[i].EvidenceIDs = emptySlice(report.Findings[i].EvidenceIDs)
		report.Findings[i].Vulnerabilities = emptySlice(report.Findings[i].Vulnerabilities)
	}
	result := Result{Snapshot: snapshot, Report: Report{ID: reportID, AnalysisID: a.ID, SnapshotID: snapshotID,
		Repository: a.Repository, AssessmentAt: a.AssessmentAt, Window: a.Window, CompletedAt: at,
		Versions: a.Versions, Sources: data.Sources, Dependencies: data.Dependencies, ReportData: report}}
	if err := result.Validate(a); err != nil {
		return Result{}, err
	}
	return result, nil
}

func equalJSON(a, b any) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	return err == nil && bytes.Equal(left, right)
}

func validScore(score *float64) bool { return score == nil || (*score >= 0 && *score <= 100) }

// Validate also runs inside the SQL transaction, against the locked stored row.
func (r Result) Validate(a Analysis) error {
	s, report := r.Snapshot, r.Report
	if err := s.Validate(); err != nil {
		return err
	}
	if !ValidID(report.ID) || s.AnalysisID != a.ID || report.AnalysisID != a.ID || report.SnapshotID != s.ID ||
		s.Owner != a.Owner || !equalJSON(s.Repository, a.Repository) || !equalJSON(report.Repository, a.Repository) ||
		s.Versions != a.Versions || report.Versions != a.Versions ||
		!s.AssessmentAt.Equal(a.AssessmentAt) || !report.AssessmentAt.Equal(a.AssessmentAt) ||
		!equalJSON(s.Window, a.Window) || !equalJSON(report.Window, a.Window) ||
		!equalJSON(s.Sources, report.Sources) || !equalJSON(s.Dependencies, report.Dependencies) ||
		!validScore(report.OverallScore) || (report.OverallScore == nil && report.OverallReason == nil) {
		return invalid("result metadata")
	}
	if a.Status == "completed" || a.Status == "failed" {
		return ErrAlreadyCompleted
	}
	if a.Status != "running" || a.StartedAt == nil || report.CompletedAt.Before(*a.StartedAt) ||
		(a.DeadlineAt != nil && !report.CompletedAt.Before(*a.DeadlineAt)) {
		return ErrInvalidTransition
	}
	inputs := map[string]MetricInput{}
	for _, m := range s.Inputs {
		inputs[m.ID] = m
	}
	categories, metrics, evidence := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, m := range s.Inputs {
		for _, e := range m.Evidence {
			evidence[e.ID] = true
		}
	}
	for _, c := range report.Categories {
		if categories[c.ID] || !validScore(c.Score) {
			return invalid("category")
		}
		categories[c.ID] = true
		if err := validateState(c.Status, c.Reason); err != nil {
			return err
		}
		for _, m := range c.Metrics {
			if metricCategories[m.Input.ID] != c.ID || metrics[m.Input.ID] ||
				!equalJSON(m.Input, inputs[m.Input.ID]) || !validScore(m.NormalizedScore) {
				return invalid("report metric")
			}
			metrics[m.Input.ID] = true
		}
	}
	if len(categories) != 5 || len(metrics) != 12 {
		return invalid("report categories/metrics")
	}
	findings := map[string]bool{}
	for _, f := range report.Findings {
		if f.ID == "" || findings[f.ID] || metricCategories[f.RuleID] != f.Category {
			return invalid("finding")
		}
		findings[f.ID] = true
		for _, id := range f.EvidenceIDs {
			if !evidence[id] {
				return invalid("finding evidence")
			}
		}
	}
	recommendations := map[string]bool{}
	for _, item := range report.Recommendations {
		if item.ID == "" || recommendations[item.ID] || !findings[item.FindingID] {
			return invalid("recommendation")
		}
		recommendations[item.ID] = true
	}
	if report.Explanation.CatalogVersion != a.Versions.RecommendationCatalog ||
		report.Explanation.SchemaVersion != a.Versions.ExplanationSchema ||
		report.Explanation.PromptVersion != a.Versions.Prompt ||
		(report.Explanation.Mode != "ai_catalog_selection" && report.Explanation.Mode != "deterministic_fallback") {
		return invalid("explanation versions")
	}
	return nil
}
