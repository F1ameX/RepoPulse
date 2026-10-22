package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Scalar permits only a JSON scalar, never source files, arbitrary objects or logs.
// Its zero value is unknown (null), distinct from confirmed zero and false.
type Scalar struct{ value string }

func (v Scalar) MarshalJSON() ([]byte, error) {
	if len(v.value) == 0 {
		return []byte("null"), nil
	}
	return []byte(v.value), nil
}

func (v *Scalar) UnmarshalJSON(data []byte) error {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	switch value.(type) {
	case nil, bool, float64, string:
		v.value = string(bytes.TrimSpace(data))
		return nil
	default:
		return fmt.Errorf("%w: raw_value must be scalar", ErrInvalidData)
	}
}

type Reason struct {
	Code           string `json:"code"`
	Message        string `json:"message"`
	CannotConclude string `json:"cannot_conclude"`
}

type Sample struct {
	Collected      *int    `json:"collected"`
	Total          *int    `json:"total"`
	Numerator      *int    `json:"numerator"`
	Denominator    *int    `json:"denominator"`
	Excluded       *int    `json:"excluded"`
	ExcludedReason *string `json:"excluded_reason"`
}

type SourceState struct {
	Source      string     `json:"source"`
	Scope       string     `json:"scope"`
	Status      string     `json:"status"`
	Reason      *Reason    `json:"reason"`
	Sample      Sample     `json:"sample"`
	ObtainedAt  *time.Time `json:"obtained_at"`
	Fingerprint *string    `json:"fingerprint"`
}

type GitRef struct {
	Name      string `json:"name"`
	ObjectSHA string `json:"object_sha"`
}

type DependencyOrigin struct {
	ManifestPath  string `json:"manifest_path"`
	VersionSource string `json:"version_source"`
}

type Dependency struct {
	Ecosystem     string             `json:"ecosystem"`
	Package       string             `json:"package"`
	Version       string             `json:"version"`
	ManifestPath  string             `json:"manifest_path"`
	VersionSource string             `json:"version_source"`
	Origins       []DependencyOrigin `json:"origins"`
}

type Vulnerability struct {
	ID          string    `json:"vulnerability_id"`
	Ecosystem   string    `json:"ecosystem"`
	Package     string    `json:"package"`
	Version     string    `json:"version"`
	Severity    *string   `json:"severity"`
	ObtainedAt  time.Time `json:"obtained_at"`
	EvidenceURL *string   `json:"evidence_url"`
}

// Evidence intentionally has no source line, match value, password or token field.
type Evidence struct {
	ID              string      `json:"id"`
	Source          string      `json:"source"`
	Kind            string      `json:"kind"`
	Description     string      `json:"description"`
	Path            *string     `json:"path"`
	Revision        *string     `json:"revision"`
	URL             *string     `json:"url"`
	RunID           *string     `json:"run_id"`
	Attempt         *int        `json:"attempt"`
	Dependency      *Dependency `json:"dependency"`
	VulnerabilityID *string     `json:"vulnerability_id"`
	SecretMatchType *string     `json:"secret_match_type"`
}

type ExcludedCount struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

type MetricInput struct {
	ID              string          `json:"id"`
	RawValue        Scalar          `json:"raw_value"`
	Unit            string          `json:"unit"`
	Status          string          `json:"status"`
	Reason          *Reason         `json:"reason"`
	Window          *Window         `json:"window"`
	Sample          Sample          `json:"sample"`
	Series          []int           `json:"series"`
	Evidence        []Evidence      `json:"evidence"`
	EvidenceLimited bool            `json:"evidence_limited"`
	EvidenceTotal   *int            `json:"evidence_total"`
	ObservedAt      *time.Time      `json:"observed_at"`
	Exclusions      []ExcludedCount `json:"exclusions"`
}

// SnapshotData is produced by trusted collectors; it cannot replace server-owned
// IDs, owner, accepted repository, window or versions.
type SnapshotData struct {
	Refs            []GitRef        `json:"refs"`
	Sources         []SourceState   `json:"sources"`
	Inputs          []MetricInput   `json:"inputs"`
	Dependencies    []Dependency    `json:"dependencies"`
	Vulnerabilities []Vulnerability `json:"vulnerabilities"`
}

type Snapshot struct {
	ID           string             `json:"snapshot_id"`
	AnalysisID   string             `json:"analysis_id"`
	Owner        Owner              `json:"owner"`
	Repository   RepositoryMetadata `json:"repository"`
	AssessmentAt time.Time          `json:"assessment_at"`
	Window       Window             `json:"window"`
	Versions     Versions           `json:"versions"`
	SnapshotData
}

// DecodeSnapshot rejects unknown fields, including raw secret/source fields.
func DecodeSnapshot(data []byte) (Snapshot, error) {
	var snapshot Snapshot
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("%w: snapshot JSON", ErrInvalidData)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Snapshot{}, fmt.Errorf("%w: trailing snapshot JSON", ErrInvalidData)
	}
	if err := snapshot.Validate(); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}
