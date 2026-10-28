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

type Error struct{ Code, Message string }

func (e *Error) Error() string { return e.Message }

var (
	ErrSessionRequired = &Error{"SESSION_REQUIRED", "Нужна действующая сессия"}
	ErrSessionExpired  = &Error{"SESSION_EXPIRED", "Сессия аккаунта недействительна. Войдите снова"}
	ErrAuthUnavailable = &Error{"AUTH_UNAVAILABLE", "Сервис сессий временно недоступен. Повторите попытку позже"}
	ErrInternal        = &Error{"INTERNAL_ERROR", "Не удалось выполнить операцию"}
)

type traceIDKey struct{}

func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, id)
}
func TraceID(ctx context.Context) string {
	id, _ := ctx.Value(traceIDKey{}).(string)
	return id
}
