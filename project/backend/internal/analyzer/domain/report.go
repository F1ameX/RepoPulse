package domain

import "time"

type Coverage struct {
	Calculated    int     `json:"calculated"`
	Total         int     `json:"total"`
	Percentage    float64 `json:"percentage"`
	Partial       int     `json:"partial"`
	Unavailable   int     `json:"unavailable"`
	NotApplicable int     `json:"not_applicable"`
}

type Metric struct {
	Input           MetricInput `json:"input"`
	NormalizedScore *float64    `json:"normalized_score"`
	RuleVersion     string      `json:"rule_version"`
	Rule            string      `json:"rule"`
	Threshold       *float64    `json:"threshold"`
	Explanation     string      `json:"explanation"`
}

type Category struct {
	ID              string   `json:"id"`
	Score           *float64 `json:"score"`
	Status          string   `json:"status"`
	Reason          *Reason  `json:"reason"`
	Coverage        Coverage `json:"coverage"`
	EffectiveWeight *float64 `json:"effective_weight"`
	Explanation     string   `json:"explanation"`
	Metrics         []Metric `json:"metrics"`
}

type Finding struct {
	ID                  string          `json:"id"`
	RuleID              string          `json:"rule_id"`
	Category            string          `json:"category"`
	Title               string          `json:"title"`
	Description         string          `json:"description"`
	Severity            string          `json:"severity"`
	PossibleConsequence string          `json:"possible_consequence"`
	EvidenceIDs         []string        `json:"evidence_ids"`
	Vulnerabilities     []Vulnerability `json:"vulnerabilities"`
}

type Recommendation struct {
	ID              string `json:"id"`
	FindingID       string `json:"finding_id"`
	Title           string `json:"title"`
	Action          string `json:"action"`
	ExpectedBenefit string `json:"expected_benefit"`
	Priority        string `json:"priority"`
	Complexity      string `json:"complexity"`
	VariantID       string `json:"variant_id"`
	Explanation     string `json:"explanation"`
}

type Explanation struct {
	Mode           string  `json:"mode"`
	FallbackReason *string `json:"fallback_reason"`
	Model          *string `json:"model"`
	CatalogVersion string  `json:"catalog_version"`
	SchemaVersion  string  `json:"schema_version"`
	PromptVersion  string  `json:"prompt_version"`
}

// ReportData contains the computed result, without client-controlled metadata.
type ReportData struct {
	OverallScore    *float64         `json:"overall_score"`
	OverallReason   *Reason          `json:"overall_reason"`
	Summary         string           `json:"summary"`
	Coverage        Coverage         `json:"coverage"`
	Categories      []Category       `json:"categories"`
	Findings        []Finding        `json:"findings"`
	Recommendations []Recommendation `json:"recommendations"`
	Explanation     Explanation      `json:"explanation"`
}

// Report is the persisted public contract. Owner is held in the relational row,
// not in its public JSON payload.
type Report struct {
	ID           string             `json:"report_id"`
	AnalysisID   string             `json:"analysis_id"`
	SnapshotID   string             `json:"snapshot_id"`
	Repository   RepositoryMetadata `json:"repository"`
	AssessmentAt time.Time          `json:"assessment_at"`
	Window       Window             `json:"window"`
	CompletedAt  time.Time          `json:"completed_at"`
	Versions     Versions           `json:"versions"`
	Sources      []SourceState      `json:"sources"`
	Dependencies []Dependency       `json:"dependencies"`
	ReportData
}

type Result struct {
	Snapshot Snapshot
	Report   Report
}
