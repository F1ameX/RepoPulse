package auth

import (
	"context"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
)

// GuestRepository is an interface for only creating guest principal and session, because this
// operation must be done in transaction.
type GuestRepository interface {
	CreateGuestSession(ctx context.Context, hash string, issuedAt, expiresAt time.Time) (domain.Session, error) // CreateGuestSession
}

// UserRepository stores principals (auth.principals).
type UserRepository interface {
	// CreateAccount creates exactly one account for a normalized login.
	// It returns domain.ErrLoginTaken when the login is used, including
	// concurrent registrations of the same login.
	CreateAccount(ctx context.Context, login, passwordHash string) (domain.Principal, error)
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

// TokenGenerator generates token and its hash for guest user
type TokenGenerator interface {
	Generate() (token, hash string, err error)
	Hash(token string) string
}

// Clock is useful for testing in edge of 24 hours
type Clock interface {
	Now() time.Time
}
