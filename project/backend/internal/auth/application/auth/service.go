// Package auth contains the Auth Service scenarios.
package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/logger"
)

// Service is the entry point to Auth business logic.
// Account registration, login and logout remain scaffold operations.
type Service struct {
	guests    GuestRepository
	users     UserRepository
	tokens    TokenRepository
	generator TokenGenerator
	now       func() time.Time
	opt       domain.Options
}

func NewService(guests GuestRepository, users UserRepository, tokens TokenRepository, generator TokenGenerator, now func() time.Time, opt domain.Options) *Service {
	return &Service{
		guests:    guests,
		users:     users,
		tokens:    tokens,
		generator: generator,
		now:       now,
		opt: opt,
	}
}

func (s *Service) Register(ctx context.Context, login string, password string) (domain.RegistrationResult, error) {
	now := s.now().UTC().Truncate(time.Microsecond)
	var hash string 
	_ , err := s.users.CreateAccount(ctx, login, hash, now)
	if err != nil {
		return domain.RegistrationResult{}, err
	}
	return domain.RegistrationResult{}, domain.ErrNotImplemented
}

func (s *Service) Login(context.Context, string, string) (domain.IssuedSession, error) {
	return domain.IssuedSession{}, domain.ErrNotImplemented
}

func (s *Service) CreateGuestSession(ctx context.Context) (domain.IssuedSession, error) {
	token, hash, err := s.generator.Generate()
	if err != nil {
		return domain.IssuedSession{}, fmt.Errorf("create token: %w", err)
	}
	// PostgreSQL timestamps have microsecond precision; the initial response
	// and subsequent reads must describe the exact same stored expiry.
	now := s.now().UTC().Truncate(time.Microsecond)
	session, err := s.guests.CreateGuestSession(ctx, hash, now, now.Add(s.opt.GuestSessionTTL))
	if err != nil {
		return domain.IssuedSession{}, fmt.Errorf("create guest: %w", err)
	}

	logger.From(ctx).InfoContext(ctx, "Guest session created",
		"principal_id", session.PrincipalID,
		"session_id", session.ID,
		"expires_at", session.ExpiresAt)

	return domain.IssuedSession{Token: token,
		Session: domain.SessionInfo{Kind: domain.KindGuest,
			Login: nil, ExpiresAt: session.ExpiresAt}}, nil
}

func (s *Service) ResolveContext(ctx context.Context, tokens domain.SessionTokens) (domain.VerifiedContext, error) {
	var verified domain.VerifiedContext
	if tokens.GuestToken != nil {
		session, err := s.tokens.FindSessionByTokenHash(ctx, s.generator.Hash(*tokens.GuestToken))
		switch {
		case errors.Is(err, domain.ErrSessionNotFound):
			// неизвестный токен: гостя нет
		case err != nil:
			return domain.VerifiedContext{}, fmt.Errorf("find guest session: %w", err)
		case session.Kind == domain.KindGuest && session.RevokedAt == nil && s.now().Before(session.ExpiresAt):
			verified.GuestSessionID = &session.PrincipalID
			verified.GuestExpiresAt = &session.ExpiresAt
		}
	}

	return verified, nil
}

func (s *Service) GetSessionContext(ctx context.Context, tokens domain.SessionTokens) (domain.SessionContext, error) {
	verified, err := s.ResolveContext(ctx, tokens)
	if err != nil {
		return domain.SessionContext{}, err
	}
	result := domain.SessionContext{}
	if verified.GuestSessionID != nil && verified.GuestExpiresAt != nil {
		result.Guest = &domain.SessionInfo{Kind: domain.KindGuest, ExpiresAt: *verified.GuestExpiresAt}
	}
	return result, nil
}

func (s *Service) Logout(context.Context, string) error {
	return domain.ErrNotImplemented
}
