// Package session coordinates Gateway calls to Auth without accessing its storage.
package session

import (
	"context"

	analyzerdomain "github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/api/domain"
)

// Auth is declared by the consumer; its gRPC implementation lives in adapter/out/auth.
type Auth interface {
	GetSessionContext(context.Context, domain.SessionTokens) (domain.SessionContext, error)
	CreateGuestSession(context.Context) (domain.IssuedSession, error)
	ResolveContext(context.Context, domain.SessionTokens) (analyzerdomain.VerifiedContext, error)
}

type Service struct{ auth Auth }

func NewService(auth Auth) *Service { return &Service{auth: auth} }

func (s *Service) GetSessionContext(ctx context.Context, tokens domain.SessionTokens) (domain.SessionContext, error) {
	result, err := s.auth.GetSessionContext(ctx, tokens)
	if err != nil {
		return domain.SessionContext{}, err
	}
	// A presented invalid account must never silently become a guest operation.
	if tokens.AccountToken != nil && result.Account == nil {
		return domain.SessionContext{}, domain.ErrSessionExpired
	}
	return result, nil
}

// ContinueGuest reuses the original guest and expiry. Only an absent or invalid
// guest causes creation; this does not implement account login/logout.
func (s *Service) ContinueGuest(ctx context.Context, tokens domain.SessionTokens) (domain.IssuedSession, bool, error) {
	existing, err := s.auth.GetSessionContext(ctx, domain.SessionTokens{GuestToken: tokens.GuestToken})
	if err != nil {
		return domain.IssuedSession{}, false, err
	}
	if existing.Guest != nil && tokens.GuestToken != nil {
		return domain.IssuedSession{Token: *tokens.GuestToken, Session: *existing.Guest}, false, nil
	}
	issued, err := s.auth.CreateGuestSession(ctx)
	return issued, true, err
}

func (s *Service) ResolveContext(ctx context.Context, tokens domain.SessionTokens) (analyzerdomain.VerifiedContext, error) {
	verified, err := s.auth.ResolveContext(ctx, tokens)
	if err != nil {
		return analyzerdomain.VerifiedContext{}, err
	}
	if tokens.AccountToken != nil && verified.UserID == nil {
		return analyzerdomain.VerifiedContext{}, domain.ErrSessionExpired
	}
	return verified, nil
}
