// Package auth contains the Auth Service scenarios.
package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/pkg/logger"
)

// registrationMessage is the FR-27 success text; registration does not sign the user in.
const registrationMessage = "Аккаунт создан. Войдите в систему"

// Service is the entry point to Auth business logic.
type Service struct {
	guests    GuestRepository
	users     UserRepository
	tokens    TokenRepository
	generator TokenGenerator
	now       func() time.Time
	opt       domain.Options
	hasher    PasswordHasher
	dummyHash func() (string, error)
}

func NewService(guests GuestRepository, users UserRepository, tokens TokenRepository, generator TokenGenerator, now func() time.Time, opt domain.Options, hasher PasswordHasher) *Service {
	return &Service{
		guests:    guests,
		users:     users,
		tokens:    tokens,
		generator: generator,
		now:       now,
		opt:       opt,
		hasher:    hasher,
		dummyHash: sync.OnceValues(func() (string, error) {
			return hasher.Hash("repopulse-timing-equalizer")
		}),
	}
}

func (s *Service) activeSession(ctx context.Context, token *string, kind domain.Kind) (*domain.Session, error) {
	if token == nil || *token == "" {
		return nil, nil
	}

	session, err := s.tokens.FindSessionByTokenHash(ctx, s.generator.Hash(*token))
	if errors.Is(err, domain.ErrSessionNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("find %s session: %w", kind, err)
	}
	if session.Kind != kind {
		return nil, nil
	}

	if session.RevokedAt != nil {
		return nil, nil
	}

	if !s.now().Before(session.ExpiresAt) {
		return nil, nil
	}

	return &session, nil
}

// Register validates the credentials (FR-27), stores the account with the
// Argon2id hash of the NFC password and returns the normalized login.
// It creates neither a session nor a report.
func (s *Service) Register(ctx context.Context, login string, password string) (domain.RegistrationResult, error) {
	valLogin, valPassword, err := domain.ValidateCredentials(login, password)
	if err != nil {
		return domain.RegistrationResult{}, err // *domain.ValidationError with field errors
	}
	hash, err := s.hasher.Hash(valPassword)
	if err != nil {
		return domain.RegistrationResult{}, fmt.Errorf("hash password: %w", err)
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	principal, err := s.users.CreateAccount(ctx, valLogin, hash, now)
	if errors.Is(err, domain.ErrLoginTaken) {
		return domain.RegistrationResult{}, domain.ErrLoginTaken
	} else if err != nil {
		return domain.RegistrationResult{}, fmt.Errorf("create account: %w", err)
	}
	logger.From(ctx).InfoContext(ctx, "Account created", "principal_id", principal.ID)
	return domain.RegistrationResult{
		Login:   principal.Login,
		Message: registrationMessage,
	}, nil
}

func (s *Service) Login(ctx context.Context, login string, password string) (domain.IssuedSession, error) {
	valLogin, valPassword, err := domain.ValidateCredentials(login, password)
	if err != nil {
		return domain.IssuedSession{}, err // *domain.ValidationError with field errors
	}
	principal, err := s.users.FindAccountByLogin(ctx, valLogin)
	if errors.Is(err, domain.ErrInvalidCredentials) {
		// Spend the same Argon2id work as for an existing login, so the response
		// time does not reveal whether the login is registered. The principal is
		// empty here, so its PasswordHash cannot be used.
		if dummy, hashErr := s.dummyHash(); hashErr == nil {
			_, _ = s.hasher.Verify(valPassword, dummy)
		}
		return domain.IssuedSession{}, domain.ErrInvalidCredentials
	} else if err != nil {
		return domain.IssuedSession{}, fmt.Errorf("find account: %w", err)
	}

	ok, err := s.hasher.Verify(valPassword, principal.PasswordHash)
	if err != nil {
		return domain.IssuedSession{}, fmt.Errorf("validate password: %w", err)
	}
	if !ok {
		return domain.IssuedSession{}, domain.ErrInvalidCredentials
	}

	token, hash, err := s.generator.Generate()
	if err != nil {
		return domain.IssuedSession{}, fmt.Errorf("token and hash generation: %w", err)
	}

	now := s.now().UTC().Truncate(time.Microsecond)
	session, err := s.tokens.CreateSession(ctx, domain.Session{PrincipalID: principal.ID,
		Kind: domain.KindAccount, TokenHash: hash, IssuedAt: now,
		ExpiresAt: now.Add(s.opt.AccountSessionTTL),
	})
	if err != nil {
		return domain.IssuedSession{}, fmt.Errorf("create account session: %w", err)
	}
	logger.From(ctx).InfoContext(ctx, "Account session created",
		"principal_id", session.PrincipalID,
		"session_id", session.ID,
		"expires_at", session.ExpiresAt,
	)

	return domain.IssuedSession{Token: token, Session: domain.SessionInfo{
		Kind:      session.Kind,
		Login:     &principal.Login,
		ExpiresAt: session.ExpiresAt,
	}}, nil
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

	account, err := s.activeSession(ctx, tokens.AccountToken, domain.KindAccount)
	if err != nil {
		return domain.VerifiedContext{}, err
	}
	if account != nil {
		verified.UserID = &account.PrincipalID
		verified.AccountExpiresAt = &account.ExpiresAt
	}

	guest, err := s.activeSession(ctx, tokens.GuestToken, domain.KindGuest)
	if err != nil {
		return domain.VerifiedContext{}, err
	}
	if guest != nil {
		verified.GuestSessionID = &guest.PrincipalID
		verified.GuestExpiresAt = &guest.ExpiresAt
	}

	return verified, nil
}

// GetSessionContext describes the valid account and guest sessions for the UI
// without tokens or IDs. The account login is read from the principal.
func (s *Service) GetSessionContext(ctx context.Context, tokens domain.SessionTokens) (domain.SessionContext, error) {
	var result domain.SessionContext

	account, err := s.activeSession(ctx, tokens.AccountToken, domain.KindAccount)
	if err != nil {
		return domain.SessionContext{}, err
	}
	if account != nil {
		principal, err := s.users.FindAccountByID(ctx, account.PrincipalID)
		if err != nil {
			// A session always references an existing principal (FK), so a
			// missing account is a data failure rather than "not signed in".
			return domain.SessionContext{}, fmt.Errorf("find account of session: %w", err)
		}
		result.Account = &domain.SessionInfo{Kind: domain.KindAccount, Login: &principal.Login, ExpiresAt: account.ExpiresAt}
	}

	guest, err := s.activeSession(ctx, tokens.GuestToken, domain.KindGuest)
	if err != nil {
		return domain.SessionContext{}, err
	}
	if guest != nil {
		result.Guest = &domain.SessionInfo{Kind: domain.KindGuest, ExpiresAt: guest.ExpiresAt}
	}
	return result, nil
}

// Logout revokes only the presented account session. It is idempotent: an
// absent, unknown, already revoked or expired token succeeds without changes.
// Guest sessions and other sessions of the same account are never revoked here.
func (s *Service) Logout(ctx context.Context, accountToken string) error {
	if accountToken == "" {
		return nil
	}
	session, err := s.tokens.FindSessionByTokenHash(ctx, s.generator.Hash(accountToken))
	if errors.Is(err, domain.ErrSessionNotFound) {
		return nil
	} else if err != nil {
		return fmt.Errorf("find account session: %w", err)
	}
	if session.Kind != domain.KindAccount {
		return nil
	}
	revoked, err := s.tokens.RevokeSession(ctx, session.ID, s.now().UTC().Truncate(time.Microsecond))
	if err != nil {
		return err
	}
	if revoked {
		logger.From(ctx).InfoContext(ctx, "Session revoked",
			"principal_id", session.PrincipalID,
			"session_id", session.ID)
	}
	return nil
}
