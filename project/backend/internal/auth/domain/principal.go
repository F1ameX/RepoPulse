// Package domain contains Auth Service models and errors independent of transport.
package domain

import "time"

// Kind distinguishes account and guest principals and sessions.
type Kind string

const (
	KindAccount Kind = "account"
	KindGuest   Kind = "guest"
)

// Principal is an identity: an account with a normalized login or a guest.
// PasswordHash is set only for accounts; the password itself is never stored.
type Principal struct {
	ID           string
	Kind         Kind
	Login        string
	PasswordHash string
	CreatedAt    time.Time
}

// Session holds the credentials record of a principal. Only the token hash is stored.
type Session struct {
	ID          string
	PrincipalID string
	Kind        Kind
	TokenHash   string
	IssuedAt    time.Time
	ExpiresAt   time.Time
	RevokedAt   *time.Time
}

// SessionInfo describes an issued session without its token.
type SessionInfo struct {
	Kind      Kind
	Login     *string
	ExpiresAt time.Time
}

// IssuedSession is a new session; Token is returned once and never stored.
type IssuedSession struct {
	Token   string
	Session SessionInfo
}

// RegistrationResult confirms account creation; it does not sign the user in.
type RegistrationResult struct {
	Login   string
	Message string
}

// SessionTokens are the raw tokens presented by the caller; nil means absent.
type SessionTokens struct {
	AccountToken *string
	GuestToken   *string
}

// VerifiedContext holds identities confirmed by Auth; nil means absent.
type VerifiedContext struct {
	UserID           *string
	GuestSessionID   *string
	AccountExpiresAt *time.Time
	GuestExpiresAt   *time.Time
}
