package auth

import (
	"context"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
)

// UserRepository stores principals (auth.principals).
type UserRepository interface {
	// CreateAccount creates exactly one account for a normalized login.
	// It returns domain.ErrLoginTaken when the login is used, including
	// concurrent registrations of the same login.
	CreateAccount(ctx context.Context, login, passwordHash string) (domain.Principal, error)
	CreateGuest(ctx context.Context) (domain.Principal, error)
	// FindAccountByLogin looks up a normalized login. It returns
	// domain.ErrInvalidCredentials when the account does not exist.
	FindAccountByLogin(ctx context.Context, login string) (domain.Principal, error)
}

// TokenRepository stores session credentials (auth.sessions) by token hash.
type TokenRepository interface {
	CreateSession(ctx context.Context, session domain.Session) (domain.Session, error)
	// FindSessionByTokenHash returns domain.ErrSessionNotFound for an unknown hash.
	FindSessionByTokenHash(ctx context.Context, tokenHash string) (domain.Session, error)
	// RevokeSession is idempotent: revoking an already revoked session succeeds.
	RevokeSession(ctx context.Context, sessionID string, at time.Time) error
}
