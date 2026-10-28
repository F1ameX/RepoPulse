// Package domain contains transport-independent Gateway session models.
package domain

import (
	"context"
	"time"
)

type SessionTokens struct {
	AccountToken *string
	GuestToken   *string
}

type SessionInfo struct {
	Kind      string
	Login     *string
	ExpiresAt time.Time
}

type SessionContext struct {
	Account *SessionInfo
	Guest   *SessionInfo
}

type IssuedSession struct {
	Token   string
	Session SessionInfo
}

// RegistrationResult confirms account creation; registration does not sign in.
type RegistrationResult struct {
	Login   string
	Message string
}

// FieldError is the first error of one request field (FR-27 order).
type FieldError struct{ Field, Code, Message string }

// Error is a safe client-facing failure. FieldErrors is set only for VALIDATION_FAILED.
type Error struct {
	Code, Message string
	FieldErrors   []FieldError
}

func (e *Error) Error() string { return e.Message }

var (
	ErrSessionRequired = &Error{Code: "SESSION_REQUIRED", Message: "Нужна действующая сессия"}
	ErrSessionExpired  = &Error{Code: "SESSION_EXPIRED", Message: "Сессия аккаунта недействительна. Войдите снова"}
	ErrAuthUnavailable = &Error{Code: "AUTH_UNAVAILABLE", Message: "Сервис сессий временно недоступен. Повторите попытку позже"}
	ErrInternal        = &Error{Code: "INTERNAL_ERROR", Message: "Не удалось выполнить операцию"}
)

type traceIDKey struct{}

func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, id)
}
func TraceID(ctx context.Context) string {
	id, _ := ctx.Value(traceIDKey{}).(string)
	return id
}
