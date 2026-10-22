// Package storage implements acceptance and immutable result persistence.
// Repository validation/collection, worker scheduling and HTTP are separate tasks.
package storage

import (
	"context"
	"strings"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	authdomain "github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
)

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository, now: time.Now}
}

// CreateAccepted is an internal operation after public repository validation.
// The only browser-owned identifier is requestID; all entity IDs and assessment
// timestamps are generated here. Auth, not request JSON, supplies verified.
func (s *Service) CreateAccepted(ctx context.Context, verified authdomain.VerifiedContext, requestID string,
	repository domain.RepositoryMetadata, versions domain.Versions) (domain.Analysis, error) {
	now := s.now()
	owner, err := creationOwner(verified, now)
	if err != nil {
		return domain.Analysis{}, err
	}
	at := domain.UTC(now)
	a := domain.Analysis{ID: domain.NewID(), SubmissionID: domain.NewID(), RequestID: strings.ToLower(requestID),
		Owner: owner, Repository: repository, AssessmentAt: at, CreatedAt: at,
		Window: domain.Window{From: at.Add(-140 * 24 * time.Hour), Until: at}, Versions: versions, Status: "queued"}
	if err := a.Validate(); err != nil {
		return domain.Analysis{}, err
	}
	return s.repository.Create(ctx, a)
}

// Start is an internal worker operation, not a browser-accessible state update.
func (s *Service) Start(ctx context.Context, id string, budget time.Duration) error {
	if !domain.ValidID(id) || budget <= 0 {
		return domain.ErrInvalidData
	}
	at := domain.UTC(s.now())
	return s.repository.Start(ctx, id, at, at.Add(budget))
}

// Complete accepts only normalized inputs and computed content. Metadata cannot
// be supplied or changed by its caller.
func (s *Service) Complete(ctx context.Context, id string, data domain.SnapshotData, report domain.ReportData) (domain.Result, error) {
	if !domain.ValidID(id) {
		return domain.Result{}, domain.ErrInvalidData
	}
	a, err := s.repository.Get(ctx, id)
	if err != nil {
		return domain.Result{}, err
	}
	result, err := domain.NewResult(a, domain.NewID(), domain.NewID(), s.now(), data, report)
	if err != nil {
		return domain.Result{}, err
	}
	if err := s.repository.Complete(ctx, result); err != nil {
		return domain.Result{}, err
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, verified authdomain.VerifiedContext, id string) (domain.Analysis, error) {
	if !domain.ValidID(id) {
		return domain.Analysis{}, domain.ErrNotFound
	}
	a, err := s.repository.Get(ctx, id)
	if err != nil {
		return domain.Analysis{}, err
	}
	if !canRead(verified, a.Owner, s.now()) {
		return domain.Analysis{}, domain.ErrNotFound
	}
	return a, nil
}

func (s *Service) GetResult(ctx context.Context, verified authdomain.VerifiedContext, id string) (domain.Result, error) {
	if _, err := s.Get(ctx, verified, id); err != nil {
		return domain.Result{}, err
	}
	return s.repository.GetResult(ctx, id)
}

func creationOwner(c authdomain.VerifiedContext, at time.Time) (domain.Owner, error) {
	if (c.UserID == nil) != (c.AccountExpiresAt == nil) ||
		(c.GuestSessionID == nil) != (c.GuestExpiresAt == nil) {
		return domain.Owner{}, domain.ErrUnverifiedOwner
	}
	if c.UserID != nil {
		if !domain.ValidID(*c.UserID) {
			return domain.Owner{}, domain.ErrUnverifiedOwner
		}
		if !at.Before(*c.AccountExpiresAt) {
			return domain.Owner{}, domain.ErrExpiredOwner
		}
		return domain.Owner{Kind: "account", ID: strings.ToLower(*c.UserID)}, nil
	}
	if c.GuestSessionID == nil || !domain.ValidID(*c.GuestSessionID) {
		return domain.Owner{}, domain.ErrUnverifiedOwner
	}
	if !at.Before(*c.GuestExpiresAt) {
		return domain.Owner{}, domain.ErrExpiredOwner
	}
	return domain.Owner{Kind: "guest", ID: strings.ToLower(*c.GuestSessionID)}, nil
}

func canRead(c authdomain.VerifiedContext, owner domain.Owner, at time.Time) bool {
	if owner.Kind == "account" {
		return c.UserID != nil && strings.EqualFold(*c.UserID, owner.ID) && c.AccountExpiresAt != nil && at.Before(*c.AccountExpiresAt)
	}
	return owner.Kind == "guest" && c.GuestSessionID != nil && strings.EqualFold(*c.GuestSessionID, owner.ID) &&
		c.GuestExpiresAt != nil && at.Before(*c.GuestExpiresAt)
}
