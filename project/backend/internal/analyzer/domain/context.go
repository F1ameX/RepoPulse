package domain

import "time"

var (
	ErrSessionRequired = &Error{Code: "SESSION_REQUIRED", Message: "Нужна действующая сессия"}
	ErrSessionExpired  = &Error{Code: "SESSION_EXPIRED", Message: "Срок действия сессии истёк"}
	ErrAuthUnavailable = &Error{Code: "AUTH_UNAVAILABLE", Message: "Сервис сессий временно недоступен. Повторите попытку позже"}
)

// VerifiedContext contains identities and expiry times verified by Auth. It is
// accepted only from trusted backend callers, never from an HTTP request body.
type VerifiedContext struct {
	UserID           *string
	GuestSessionID   *string
	AccountExpiresAt *time.Time
	GuestExpiresAt   *time.Time
}

func (c VerifiedContext) Validate(now time.Time) error {
	if (c.UserID == nil) != (c.AccountExpiresAt == nil) ||
		(c.GuestSessionID == nil) != (c.GuestExpiresAt == nil) {
		return ErrSessionRequired
	}
	if c.UserID != nil {
		if *c.UserID == "" {
			return ErrSessionRequired
		}
		// An expired account must not silently fall back to a guest identity.
		if !now.Before(*c.AccountExpiresAt) {
			return ErrSessionExpired
		}
		return nil
	}
	if c.GuestSessionID == nil || *c.GuestSessionID == "" {
		return ErrSessionRequired
	}
	if !now.Before(*c.GuestExpiresAt) {
		return ErrSessionExpired
	}
	return nil
}
