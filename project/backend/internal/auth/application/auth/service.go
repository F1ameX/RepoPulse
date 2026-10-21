// Package auth contains the Auth Service scenarios.
package auth

import (
	"context"

	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
)

// Service is the entry point to Auth business logic.
// Scenarios are not implemented yet and return domain.ErrNotImplemented.
type Service struct {
	users  UserRepository
	tokens TokenRepository
}

func NewService(users UserRepository, tokens TokenRepository) *Service {
	return &Service{users: users, tokens: tokens}
}

func (s *Service) Register(context.Context, string, string) (domain.RegistrationResult, error) {
	return domain.RegistrationResult{}, domain.ErrNotImplemented
}

func (s *Service) Login(context.Context, string, string) (domain.IssuedSession, error) {
	return domain.IssuedSession{}, domain.ErrNotImplemented
}

func (s *Service) CreateGuestSession(context.Context) (domain.IssuedSession, error) {
	return domain.IssuedSession{}, domain.ErrNotImplemented
}

func (s *Service) ResolveContext(context.Context, domain.SessionTokens) (domain.VerifiedContext, error) {
	return domain.VerifiedContext{}, domain.ErrNotImplemented
}

func (s *Service) Logout(context.Context, string) error {
	return domain.ErrNotImplemented
}
