// Package analysis implements submission replay and persisted status reads.
package analysis

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

// Store reserves owner/request_id before any mutable external source is read.
type Store interface {
	ReserveSubmission(context.Context, domain.Submission) (domain.Submission, bool, error)
	AcceptSubmission(context.Context, domain.Analysis) (domain.Analysis, error)
	RejectSubmission(context.Context, string, domain.Failure) error
	Get(context.Context, string) (domain.Analysis, error)
}

type Source interface {
	LookupForAnalysis(context.Context, domain.RepositoryURL) (domain.RepositoryMetadata, error)
}

type CreateAnalysisRequest struct {
	RequestID     string
	RepositoryURL string
}

type CreateAnalysisCommand struct {
	Context domain.VerifiedContext
	Request CreateAnalysisRequest
}

type GetAnalysisQuery struct {
	Context    domain.VerifiedContext
	AnalysisID string
}

type Service struct {
	store    Store
	source   Source
	versions domain.Versions
	budget   time.Duration
	now      func() time.Time
}

func NewService(store Store, source Source, versions domain.Versions, budget time.Duration) *Service {
	return &Service{store: store, source: source, versions: versions, budget: budget, now: time.Now}
}

var ErrRequestID = &domain.Error{Code: "VALIDATION_FAILED", Message: "request_id должен быть UUID отправки"}

func (s *Service) CreateAnalysis(ctx context.Context, command CreateAnalysisCommand) (domain.Analysis, error) {
	at := domain.UTC(s.now())
	owner, err := creationOwner(command.Context, at)
	if err != nil {
		return domain.Analysis{}, err
	}
	address, err := domain.ParseRepositoryURL(command.Request.RepositoryURL)
	if err != nil {
		return domain.Analysis{}, err
	}
	if !domain.ValidID(command.Request.RequestID) {
		return domain.Analysis{}, ErrRequestID
	}
	if err := s.versions.Validate(); err != nil || s.budget <= 0 {
		return domain.Analysis{}, domain.ErrInvalidData
	}
	// A disconnected browser must not interrupt an already reserved submission.
	// The independent bounded context still keeps logging and diagnostic values.
	work, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.budget)
	defer cancel()
	submission, fresh, err := s.store.ReserveSubmission(work, domain.Submission{
		ID: domain.NewID(), RequestID: strings.ToLower(command.Request.RequestID), Owner: owner,
		CanonicalURL: address.CanonicalURL(), AssessmentAt: at, State: "pending",
	})
	if err != nil {
		return domain.Analysis{}, err
	}
	if submission.CanonicalURL != address.CanonicalURL() {
		return domain.Analysis{}, domain.ErrSubmissionChanged
	}
	if !fresh {
		if submission.AnalysisID != nil {
			return s.store.Get(work, *submission.AnalysisID)
		}
		if submission.Failure != nil {
			return domain.Analysis{}, submissionError(submission.Failure.Code)
		}
		return domain.Analysis{}, domain.ErrSubmissionPending
	}
	repository, err := s.source.LookupForAnalysis(work, address)
	if err == nil && repository.Validate() != nil {
		err = domain.ErrRepositoryCheck
	}
	if err != nil {
		failure := submissionError(errorCode(err))
		// Finalize the reserved outcome even when the source exhausted its budget.
		finish, stop := context.WithTimeout(context.WithoutCancel(work), 3*time.Second)
		defer stop()
		if saveErr := s.store.RejectSubmission(finish, submission.ID, domain.Failure{Code: errorCode(failure)}); saveErr != nil {
			return domain.Analysis{}, saveErr
		}
		return domain.Analysis{}, failure
	}
	a := domain.Analysis{
		ID: domain.NewID(), SubmissionID: submission.ID, RequestID: submission.RequestID,
		Owner: submission.Owner, Repository: repository, AssessmentAt: submission.AssessmentAt,
		Window:   domain.Window{From: submission.AssessmentAt.Add(-140 * 24 * time.Hour), Until: submission.AssessmentAt},
		Versions: s.versions, Status: "queued", CreatedAt: domain.UTC(s.now()),
	}
	if err := a.Validate(); err != nil {
		return domain.Analysis{}, err
	}
	finish, stop := context.WithTimeout(context.WithoutCancel(work), 3*time.Second)
	defer stop()
	// Unknown freshness always creates a new analysis for a new request_id.
	// No lookup by URL, SHA or TTL claims a reusable report before #18538.
	return s.store.AcceptSubmission(finish, a)
}

func (s *Service) GetAnalysis(ctx context.Context, query GetAnalysisQuery) (domain.Analysis, error) {
	now := s.now()
	if err := query.Context.Validate(now); err != nil {
		return domain.Analysis{}, err
	}
	if !domain.ValidID(query.AnalysisID) {
		return domain.Analysis{}, domain.ErrNotFound
	}
	read, cancel := context.WithTimeout(ctx, s.budget)
	defer cancel()
	a, err := s.store.Get(read, query.AnalysisID)
	if err != nil {
		return domain.Analysis{}, err
	}
	c := query.Context
	allowed := a.Owner.Kind == "account" && c.UserID != nil && strings.EqualFold(a.Owner.ID, *c.UserID) &&
		c.AccountExpiresAt != nil && now.Before(*c.AccountExpiresAt)
	allowed = allowed || (a.Owner.Kind == "guest" && c.GuestSessionID != nil && strings.EqualFold(a.Owner.ID, *c.GuestSessionID) &&
		c.GuestExpiresAt != nil && now.Before(*c.GuestExpiresAt))
	if !allowed {
		return domain.Analysis{}, domain.ErrNotFound
	}
	switch a.Status {
	case "queued", "running", "failed":
		if a.ReportID != nil {
			return domain.Analysis{}, domain.ErrInvalidData
		}
	case "completed":
		if a.ReportID == nil || !domain.ValidID(*a.ReportID) || a.CompletedAt == nil {
			return domain.Analysis{}, domain.ErrInvalidData
		}
	default:
		return domain.Analysis{}, domain.ErrInvalidData
	}
	return a, nil
}

func creationOwner(c domain.VerifiedContext, at time.Time) (domain.Owner, error) {
	if err := c.Validate(at); err != nil {
		return domain.Owner{}, err
	}
	owner := domain.Owner{Kind: "guest"}
	if c.UserID != nil {
		owner.Kind, owner.ID = "account", strings.ToLower(*c.UserID)
	} else {
		owner.ID = strings.ToLower(*c.GuestSessionID)
	}
	if owner.Validate() != nil {
		return domain.Owner{}, domain.ErrSessionRequired
	}
	return owner, nil
}

func errorCode(err error) string {
	var public *domain.Error
	if errors.As(err, &public) {
		return public.Code
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return domain.ErrUpstreamTimeout.Code
	}
	return domain.ErrRepositoryCheck.Code
}

func submissionError(code string) error {
	switch code {
	case "REPOSITORY_NOT_FOUND_OR_PRIVATE":
		return domain.ErrRepositoryUnavailable
	case "UPSTREAM_TIMEOUT":
		return domain.ErrUpstreamTimeout
	case "UPSTREAM_RATE_LIMITED":
		return domain.ErrUpstreamRateLimited
	default:
		return domain.ErrRepositoryCheck
	}
}
