package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
)

// hashingGenerator maps every token to its own hash, so different tokens find different sessions.
type hashingGenerator struct{}

func (hashingGenerator) Generate() (string, string, error) { return "", "", errors.New("not used") }
func (hashingGenerator) Hash(token string) string          { return "hash:" + token }

var sessionExpiry = testNow.Add(168 * time.Hour)

func sessionsFixture() *fakeTokens {
	return &fakeTokens{sessions: map[string]domain.Session{
		"hash:account": {ID: "account-session", PrincipalID: "account-id", Kind: domain.KindAccount, ExpiresAt: sessionExpiry},
		"hash:other":   {ID: "other-session", PrincipalID: "account-id", Kind: domain.KindAccount, ExpiresAt: sessionExpiry},
		"hash:guest":   {ID: "guest-session", PrincipalID: "guest-id", Kind: domain.KindGuest, ExpiresAt: testNow.Add(24 * time.Hour)},
	}}
}

func newSessionService(tokens *fakeTokens, users *fakeUsers, now time.Time) *Service {
	return NewService(nil, users, tokens, hashingGenerator{}, func() time.Time { return now }, testOptions, nil)
}

func ptr(s string) *string { return &s }

func TestResolveContextAccount(t *testing.T) {
	verified, err := newSessionService(sessionsFixture(), &fakeUsers{}, testNow).
		ResolveContext(context.Background(), domain.SessionTokens{AccountToken: ptr("account")})
	if err != nil {
		t.Fatal(err)
	}
	if verified.UserID == nil || *verified.UserID != "account-id" || verified.AccountExpiresAt == nil ||
		!verified.AccountExpiresAt.Equal(sessionExpiry) {
		t.Fatalf("verified = %+v; want the account principal and expiry", verified)
	}
	if verified.GuestSessionID != nil || verified.GuestExpiresAt != nil {
		t.Fatalf("verified = %+v; no guest token was presented", verified)
	}
}

// FR-27 acceptance: the session is valid right before the 7-day boundary and invalid at it.
func TestResolveContextAccountExpiryBoundary(t *testing.T) {
	for name, tc := range map[string]struct {
		now   time.Time
		valid bool
	}{
		"just before expiry": {sessionExpiry.Add(-time.Microsecond), true},
		"exactly at expiry":  {sessionExpiry, false},
		"after expiry":       {sessionExpiry.Add(time.Hour), false},
	} {
		t.Run(name, func(t *testing.T) {
			verified, err := newSessionService(sessionsFixture(), &fakeUsers{}, tc.now).
				ResolveContext(context.Background(), domain.SessionTokens{AccountToken: ptr("account")})
			if err != nil {
				t.Fatal(err)
			}
			if (verified.UserID != nil) != tc.valid || (verified.AccountExpiresAt != nil) != tc.valid {
				t.Fatalf("verified = %+v, want valid=%v", verified, tc.valid)
			}
		})
	}
}

func TestResolveContextRejectsInvalidAccountTokens(t *testing.T) {
	revoked := sessionsFixture()
	session := revoked.sessions["hash:account"]
	session.RevokedAt = &testNow
	revoked.sessions["hash:account"] = session

	for name, tc := range map[string]struct {
		tokens *fakeTokens
		token  string
	}{
		"revoked session":             {revoked, "account"},
		"guest token in account slot": {sessionsFixture(), "guest"},
		"unknown token":               {sessionsFixture(), "forged"},
		"empty token":                 {sessionsFixture(), ""},
	} {
		t.Run(name, func(t *testing.T) {
			verified, err := newSessionService(tc.tokens, &fakeUsers{}, testNow).
				ResolveContext(context.Background(), domain.SessionTokens{AccountToken: ptr(tc.token)})
			if err != nil || verified.UserID != nil || verified.AccountExpiresAt != nil {
				t.Fatalf("verified = %+v, err = %v; want no account and no error", verified, err)
			}
		})
	}

	verified, err := newSessionService(sessionsFixture(), &fakeUsers{}, testNow).
		ResolveContext(context.Background(), domain.SessionTokens{GuestToken: ptr("account")})
	if err != nil || verified.GuestSessionID != nil {
		t.Fatalf("account token in guest slot accepted: %+v, %v", verified, err)
	}
}

// FR-27: account and guest contexts coexist; the owner of new operations is chosen by Analyzer.
func TestResolveContextAccountAndGuestTogether(t *testing.T) {
	verified, err := newSessionService(sessionsFixture(), &fakeUsers{}, testNow).
		ResolveContext(context.Background(), domain.SessionTokens{AccountToken: ptr("account"), GuestToken: ptr("guest")})
	if err != nil {
		t.Fatal(err)
	}
	if verified.UserID == nil || *verified.UserID != "account-id" || verified.GuestSessionID == nil || *verified.GuestSessionID != "guest-id" {
		t.Fatalf("verified = %+v; want both contexts", verified)
	}
}

func TestResolveContextStorageFailure(t *testing.T) {
	tokens := sessionsFixture()
	tokens.findErr = errors.New("database unavailable")
	_, err := newSessionService(tokens, &fakeUsers{}, testNow).
		ResolveContext(context.Background(), domain.SessionTokens{AccountToken: ptr("account")})
	if !errors.Is(err, tokens.findErr) {
		t.Fatalf("err = %v; a storage failure must not look like a missing session", err)
	}
}

func TestGetSessionContextAccountAndGuest(t *testing.T) {
	account := storedAccount
	result, err := newSessionService(sessionsFixture(), &fakeUsers{account: &account}, testNow).
		GetSessionContext(context.Background(), domain.SessionTokens{AccountToken: ptr("account"), GuestToken: ptr("guest")})
	if err != nil {
		t.Fatal(err)
	}
	if result.Account == nil || result.Account.Kind != domain.KindAccount || result.Account.Login == nil ||
		*result.Account.Login != "flamex" || !result.Account.ExpiresAt.Equal(sessionExpiry) {
		t.Fatalf("account = %+v", result.Account)
	}
	if result.Guest == nil || result.Guest.Kind != domain.KindGuest || result.Guest.Login != nil {
		t.Fatalf("guest = %+v; guest has no login", result.Guest)
	}
}

func TestGetSessionContextMissingAccountIsFailure(t *testing.T) {
	_, err := newSessionService(sessionsFixture(), &fakeUsers{}, testNow).
		GetSessionContext(context.Background(), domain.SessionTokens{AccountToken: ptr("account")})
	if !errors.Is(err, domain.ErrPrincipalNotFound) {
		t.Fatalf("err = %v; a session without its account is a data failure", err)
	}
}

func TestLogoutRevokesOnlyCurrentAccountSession(t *testing.T) {
	tokens := sessionsFixture()
	svc := newSessionService(tokens, &fakeUsers{}, testNow)

	if err := svc.Logout(context.Background(), "account"); err != nil {
		t.Fatal(err)
	}
	if len(tokens.revoked) != 1 || tokens.revoked[0] != "account-session" {
		t.Fatalf("revoked = %q; want only the presented session", tokens.revoked)
	}

	// The revoked token is rejected; another session of the same account and the guest stay valid.
	verified, err := svc.ResolveContext(context.Background(), domain.SessionTokens{AccountToken: ptr("account")})
	if err != nil || verified.UserID != nil {
		t.Fatalf("revoked session reused: %+v, %v", verified, err)
	}
	verified, err = svc.ResolveContext(context.Background(), domain.SessionTokens{AccountToken: ptr("other"), GuestToken: ptr("guest")})
	if err != nil || verified.UserID == nil || verified.GuestSessionID == nil {
		t.Fatalf("independent sessions were affected: %+v, %v", verified, err)
	}
}

func TestLogoutIsIdempotent(t *testing.T) {
	tokens := sessionsFixture()
	svc := newSessionService(tokens, &fakeUsers{}, testNow)
	for _, token := range []string{"account", "account", "forged", ""} {
		if err := svc.Logout(context.Background(), token); err != nil {
			t.Fatalf("Logout(%q) = %v; want success", token, err)
		}
	}
	revokedAt := tokens.sessions["hash:account"].RevokedAt
	if revokedAt == nil || !revokedAt.Equal(testNow) {
		t.Fatalf("revoked_at = %v; a repeated logout must keep the first revocation time", revokedAt)
	}
}

func TestLogoutDoesNotRevokeGuestSession(t *testing.T) {
	tokens := sessionsFixture()
	if err := newSessionService(tokens, &fakeUsers{}, testNow).Logout(context.Background(), "guest"); err != nil {
		t.Fatal(err)
	}
	if len(tokens.revoked) != 0 {
		t.Fatalf("revoked = %q; logout must not touch the guest session", tokens.revoked)
	}
}

func TestLogoutStorageFailure(t *testing.T) {
	tokens := sessionsFixture()
	tokens.findErr = errors.New("database unavailable")
	if err := newSessionService(tokens, &fakeUsers{}, testNow).Logout(context.Background(), "account"); !errors.Is(err, tokens.findErr) {
		t.Fatalf("err = %v; an unconfirmed logout must not report success", err)
	}
}
