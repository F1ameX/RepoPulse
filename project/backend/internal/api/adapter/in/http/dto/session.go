package dto

import (
	"github.com/F1ameX/RepoPulse/project/backend/internal/api/domain"
	"time"
)

type SessionInfo struct {
	Kind      string    `json:"kind"`
	Login     *string   `json:"login"`
	ExpiresAt time.Time `json:"expires_at"`
}

type RegistrationResult struct {
	Login   string `json:"login"`
	Message string `json:"message"`
}

type SessionContext struct {
	Account *SessionInfo `json:"account"`
	Guest   *SessionInfo `json:"guest"`
}

func Session(value *domain.SessionInfo) *SessionInfo {
	if value == nil {
		return nil
	}
	return &SessionInfo{Kind: value.Kind, Login: value.Login, ExpiresAt: value.ExpiresAt.UTC()}
}
