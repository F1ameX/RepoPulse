package report

import (
	"context"
	"strings"
	"time"

	analyzer "github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/report/domain"
)

type Repository interface {
	Get(context.Context, string) (domain.Report, string, error)
}

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository, now: time.Now}
}

func (s *Service) GetReport(ctx context.Context, verified domain.VerifiedContext, id string) (domain.Report, error) {
	if err := verified.Validate(s.now()); err != nil {
		return domain.Report{}, err
	}
	if !analyzer.ValidID(id) {
		return domain.Report{}, domain.ErrNotFound
	}
	id = strings.ToLower(id)
	report, ownerID, err := s.repository.Get(ctx, id)
	if err != nil {
		return domain.Report{}, err
	}
	now := s.now()
	account := verified.UserID != nil && strings.EqualFold(*verified.UserID, ownerID) &&
		verified.AccountExpiresAt != nil && now.Before(*verified.AccountExpiresAt)
	guest := verified.GuestSessionID != nil && strings.EqualFold(*verified.GuestSessionID, ownerID) &&
		verified.GuestExpiresAt != nil && now.Before(*verified.GuestExpiresAt)
	if !account && !guest {
		return domain.Report{}, domain.ErrNotFound
	}
	return report, nil
}
