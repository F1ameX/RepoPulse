package session

import (
	"context"
	"errors"
	"testing"
	"time"

	analyzerdomain "github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
	"github.com/F1ameX/RepoPulse/project/backend/internal/api/domain"
)

type fakeAuth struct {
	context  domain.SessionContext
	verified analyzerdomain.VerifiedContext
	err      error
	created  int
}

func (a *fakeAuth) GetSessionContext(context.Context, domain.SessionTokens) (domain.SessionContext, error) {
	return a.context, a.err
}
func (a *fakeAuth) ResolveContext(context.Context, domain.SessionTokens) (analyzerdomain.VerifiedContext, error) {
	return a.verified, a.err
}
func (a *fakeAuth) CreateGuestSession(context.Context) (domain.IssuedSession, error) {
	a.created++
	return domain.IssuedSession{Token: "new"}, a.err
}

func TestContinueGuestDoesNotExtendOrDuplicate(t *testing.T) {
	token := "original"
	expiry := time.Now().Add(time.Hour)
	a := &fakeAuth{context: domain.SessionContext{Guest: &domain.SessionInfo{Kind: "guest", ExpiresAt: expiry}}}
	issued, created, err := NewService(a).ContinueGuest(context.Background(), domain.SessionTokens{GuestToken: &token})
	if err != nil || created || a.created != 0 || issued.Token != token || !issued.Session.ExpiresAt.Equal(expiry) {
		t.Fatalf("unexpected replay: %+v, %v, %v", issued, created, err)
	}
	a.context.Guest = nil
	_, created, err = NewService(a).ContinueGuest(context.Background(), domain.SessionTokens{GuestToken: &token})
	if err != nil || !created || a.created != 1 {
		t.Fatal("invalid guest must be replaced exactly once")
	}
	a.err = domain.ErrAuthUnavailable
	_, _, err = NewService(a).ContinueGuest(context.Background(), domain.SessionTokens{})
	if !errors.Is(err, domain.ErrAuthUnavailable) || a.created != 1 {
		t.Fatal("Auth failure must not cause a new guest")
	}
}

func TestInvalidAccountNeverFallsBackToGuest(t *testing.T) {
	token, id := "invalid-account", "guest-id"
	expiry := time.Now().Add(time.Hour)
	a := &fakeAuth{context: domain.SessionContext{Guest: &domain.SessionInfo{Kind: "guest", ExpiresAt: expiry}},
		verified: analyzerdomain.VerifiedContext{GuestSessionID: &id, GuestExpiresAt: &expiry}}
	s := NewService(a)
	tokens := domain.SessionTokens{AccountToken: &token, GuestToken: &token}
	if _, err := s.GetSessionContext(context.Background(), tokens); !errors.Is(err, domain.ErrSessionExpired) {
		t.Fatalf("context: %v", err)
	}
	if _, err := s.ResolveContext(context.Background(), tokens); !errors.Is(err, domain.ErrSessionExpired) {
		t.Fatalf("validation: %v", err)
	}
}
