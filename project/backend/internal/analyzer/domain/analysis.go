// Package domain contains transport-independent Analyzer models.
package domain

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	ErrInvalidData       = errors.New("invalid analysis data")
	ErrNotFound          = errors.New("analysis not found")
	ErrAlreadyCompleted  = errors.New("analysis is already terminal")
	ErrInvalidTransition = errors.New("invalid analysis transition")
	ErrSubmissionChanged = errors.New("submission URL changed")
	ErrSubmissionPending = errors.New("submission is pending")
	ErrUnverifiedOwner   = errors.New("verified session required")
	ErrExpiredOwner      = errors.New("verified session expired")
)

type Owner struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// RepositoryMetadata is the accepted immutable descriptor, including its pinned
// revision, rather than a repository URL-validation response.
type RepositoryMetadata struct {
	Platform      string  `json:"platform"`
	GitHubID      string  `json:"github_id"`
	CanonicalURL  string  `json:"canonical_url"`
	Owner         string  `json:"owner"`
	Name          string  `json:"name"`
	DefaultBranch *string `json:"default_branch"`
	CommitSHA     *string `json:"commit_sha"`
}

type Window struct {
	From  time.Time `json:"from"`
	Until time.Time `json:"until"`
}

type Versions struct {
	Schema                string `json:"schema"`
	Methodology           string `json:"methodology"`
	ScoringConfig         string `json:"scoring_config"`
	CollectorProfile      string `json:"collector_profile"`
	SecretRules           string `json:"secret_rules"`
	RecommendationCatalog string `json:"recommendation_catalog"`
	ExplanationSchema     string `json:"explanation_schema"`
	Prompt                string `json:"prompt"`
}

// Analysis is persisted at acceptance. There is deliberately no metadata update
// operation: subsequent writes may only start execution or publish its result.
type Analysis struct {
	ID           string
	SubmissionID string
	RequestID    string
	Owner        Owner
	Repository   RepositoryMetadata
	AssessmentAt time.Time
	Window       Window
	Versions     Versions
	Stage        *string
	Error        *AnalysisFailure
	Status       string
	CreatedAt    time.Time
	StartedAt    *time.Time
	DeadlineAt   *time.Time
	CompletedAt  *time.Time
	ReportID     *string
}

// SubmissionFailure records a failed preflight, before any analysis job exists.
// Terminal job failures use the shared AnalysisFailure model from job.go.
type SubmissionFailure struct {
	Code    string `json:"code"`
	TraceID string `json:"trace_id"`
}

type Submission struct {
	ID           string
	RequestID    string
	Owner        Owner
	CanonicalURL string
	AssessmentAt time.Time
	State        string
	AnalysisID   *string
	Failure      *SubmissionFailure
}

var uuidPattern = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")
var shaPattern = regexp.MustCompile("^[0-9a-f]{40}$")

func ValidID(id string) bool { return uuidPattern.MatchString(id) }

// NewID generates identifiers on the server; IDs are never supplied in HTTP DTOs.
func NewID() string {
	var value [16]byte
	_, _ = rand.Read(value[:])
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", value[:4], value[4:6], value[6:8], value[8:10], value[10:])
}

// UTC uses PostgreSQL's microsecond precision so JSON and relational timestamps
// describe exactly the same accepted instant after a round trip.
func UTC(at time.Time) time.Time { return at.UTC().Truncate(time.Microsecond) }

func (o Owner) Validate() error {
	if !ValidID(o.ID) || (o.Kind != "account" && o.Kind != "guest") {
		return fmt.Errorf("%w: owner", ErrInvalidData)
	}
	return nil
}

func (r RepositoryMetadata) Validate() error {
	u, err := url.Parse(r.CanonicalURL)
	if err != nil || r.Platform != "github" || r.GitHubID == "" || r.Owner == "" || r.Name == "" ||
		u.Scheme != "https" || u.Host != "github.com" || u.User != nil ||
		u.RawQuery != "" || u.Fragment != "" || u.ForceQuery ||
		r.CanonicalURL != "https://github.com/"+strings.ToLower(r.Owner)+"/"+strings.ToLower(r.Name) ||
		u.Path != "/"+strings.ToLower(r.Owner)+"/"+strings.ToLower(r.Name) ||
		strings.HasSuffix(u.Path, ".git") || strings.ContainsAny(r.Owner+r.Name, "/\\?#% \r\n") {
		return fmt.Errorf("%w: repository", ErrInvalidData)
	}
	if r.DefaultBranch != nil && strings.TrimSpace(*r.DefaultBranch) == "" {
		return fmt.Errorf("%w: branch", ErrInvalidData)
	}
	if r.CommitSHA != nil && !shaPattern.MatchString(*r.CommitSHA) {
		return fmt.Errorf("%w: commit SHA", ErrInvalidData)
	}
	return nil
}

func (v Versions) Validate() error {
	for _, value := range []string{v.Schema, v.Methodology, v.ScoringConfig, v.CollectorProfile,
		v.SecretRules, v.RecommendationCatalog, v.ExplanationSchema, v.Prompt} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%w: versions", ErrInvalidData)
		}
	}
	return nil
}

func (a Analysis) Validate() error {
	if !ValidID(a.ID) || !ValidID(a.SubmissionID) || !ValidID(a.RequestID) ||
		a.AssessmentAt.IsZero() || a.CreatedAt.Before(a.AssessmentAt) ||
		!a.Window.Until.Equal(a.AssessmentAt) ||
		!a.Window.From.Equal(a.AssessmentAt.Add(-140*24*time.Hour)) {
		return fmt.Errorf("%w: analysis metadata", ErrInvalidData)
	}
	if err := a.Owner.Validate(); err != nil {
		return err
	}
	if err := a.Repository.Validate(); err != nil {
		return err
	}
	return a.Versions.Validate()
}
