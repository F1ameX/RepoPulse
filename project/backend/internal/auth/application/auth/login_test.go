package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
)

type fakeTokens struct {
	created []domain.Session
	err     error
}

func (f *fakeTokens) CreateSession(_ context.Context, session domain.Session) (domain.Session, error) {
	f.created = append(f.created, session)
	if f.err != nil {
		return domain.Session{}, f.err
	}
	session.ID = "session-id"
	return session, nil
}

func (f *fakeTokens) FindSessionByTokenHash(context.Context, string) (domain.Session, error) {
	return domain.Session{}, domain.ErrSessionNotFound
}

func (f *fakeTokens) RevokeSession(context.Context, string, time.Time) error { return nil }

var storedAccount = domain.Principal{
	ID: "account-id", Kind: domain.KindAccount, Login: "flamex", PasswordHash: "stored-hash",
}

func newLoginService(users *fakeUsers, tokens *fakeTokens, hasher *fakeHasher, gen fakeGenerator) *Service {
	now := func() time.Time { return testNow.Add(1500 * time.Nanosecond) }
	return NewService(nil, users, tokens, gen, now, testOptions, hasher)
}

func TestLoginCreatesAccountSession(t *testing.T) {
	account := storedAccount
	users, tokens, hasher := &fakeUsers{account: &account}, &fakeTokens{}, &fakeHasher{match: true}
	svc := newLoginService(users, tokens, hasher, fakeGenerator{token: "raw-token", hash: "token-hash"})

	issued, err := svc.Login(context.Background(), " FlameX ", "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if users.login != "flamex" {
		t.Fatalf("looked up login %q, want the normalized one", users.login)
	}
	if len(hasher.verified) != 1 || hasher.verified[0] != "stored-hash" {
		t.Fatalf("Verify got %q, want the stored hash", hasher.verified)
	}
	issuedAt := testNow.Add(time.Microsecond) // UTC truncated to PostgreSQL precision
	want := domain.Session{
		PrincipalID: "account-id", Kind: domain.KindAccount, TokenHash: "token-hash",
		IssuedAt: issuedAt, ExpiresAt: issuedAt.Add(testOptions.AccountSessionTTL),
	}
	if len(tokens.created) != 1 || tokens.created[0] != want {
		t.Fatalf("stored session = %+v, want %+v", tokens.created, want)
	}
	if issued.Token != "raw-token" || issued.Session.Kind != domain.KindAccount ||
		issued.Session.Login == nil || *issued.Session.Login != "flamex" || !issued.Session.ExpiresAt.Equal(want.ExpiresAt) {
		t.Fatalf("issued = %+v", issued)
	}
}

func TestLoginEachSuccessCreatesNewSession(t *testing.T) {
	account := storedAccount
	tokens := &fakeTokens{}
	svc := newLoginService(&fakeUsers{account: &account}, tokens, &fakeHasher{match: true},
		fakeGenerator{token: "raw-token", hash: "token-hash"})
	for range 2 {
		if _, err := svc.Login(context.Background(), "flamex", "correct horse battery"); err != nil {
			t.Fatal(err)
		}
	}
	if len(tokens.created) != 2 {
		t.Fatalf("sessions = %d; independent logins must create independent sessions", len(tokens.created))
	}
}

// FR-27: an unknown valid login and a wrong password give the same error, and
// both spend an Argon2id verification so timing does not reveal the login.
func TestLoginUnknownLoginAndWrongPasswordAreIndistinguishable(t *testing.T) {
	account := storedAccount
	cases := map[string]*fakeUsers{
		"unknown login":  {},
		"wrong password": {account: &account},
	}
	for name, users := range cases {
		t.Run(name, func(t *testing.T) {
			tokens, hasher := &fakeTokens{}, &fakeHasher{match: false}
			svc := newLoginService(users, tokens, hasher, fakeGenerator{token: "raw-token", hash: "token-hash"})
			_, err := svc.Login(context.Background(), "flamex", "correct horse battery")
			if !errors.Is(err, domain.ErrInvalidCredentials) {
				t.Fatalf("err = %v, want ErrInvalidCredentials", err)
			}
			if len(hasher.verified) != 1 || hasher.verified[0] == "" {
				t.Fatalf("Verify calls = %q; want exactly one with a real hash", hasher.verified)
			}
			if len(tokens.created) != 0 {
				t.Fatal("a rejected login must not create a session")
			}
		})
	}
}

func TestLoginDummyHashIsComputedOnce(t *testing.T) {
	hasher := &fakeHasher{}
	svc := newLoginService(&fakeUsers{}, &fakeTokens{}, hasher, fakeGenerator{})
	for range 3 {
		_, _ = svc.Login(context.Background(), "flamex", "correct horse battery")
	}
	if len(hasher.hashed) != 1 || len(hasher.verified) != 3 {
		t.Fatalf("Hash calls = %d, Verify calls = %d; want one dummy hash reused", len(hasher.hashed), len(hasher.verified))
	}
}

func TestLoginValidationKeepsFieldErrors(t *testing.T) {
	users, hasher := &fakeUsers{}, &fakeHasher{}
	_, err := newLoginService(users, &fakeTokens{}, hasher, fakeGenerator{}).
		Login(context.Background(), "1abc", "")
	validation, ok := errors.AsType[*domain.ValidationError](err)
	if !ok || len(validation.Fields) != 2 {
		t.Fatalf("err = %#v; want login and password field errors", err)
	}
	if users.login != "" || len(hasher.verified) != 0 {
		t.Fatal("account lookup must happen only after both fields pass validation")
	}
}

// FR-27: a service failure is a separate error, never "invalid credentials".
func TestLoginFailuresAreNotInvalidCredentials(t *testing.T) {
	account := storedAccount
	errFail := errors.New("dependency unavailable")
	cases := map[string]struct {
		users  *fakeUsers
		tokens *fakeTokens
		hasher *fakeHasher
		gen    fakeGenerator
	}{
		"account lookup": {&fakeUsers{findErr: errFail}, &fakeTokens{}, &fakeHasher{}, fakeGenerator{}},
		"broken hash":    {&fakeUsers{account: &account}, &fakeTokens{}, &fakeHasher{verifyErr: errFail}, fakeGenerator{}},
		"token":          {&fakeUsers{account: &account}, &fakeTokens{}, &fakeHasher{match: true}, fakeGenerator{err: errFail}},
		"session store":  {&fakeUsers{account: &account}, &fakeTokens{err: errFail}, &fakeHasher{match: true}, fakeGenerator{token: "t", hash: "h"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			issued, err := newLoginService(tc.users, tc.tokens, tc.hasher, tc.gen).
				Login(context.Background(), "flamex", "correct horse battery")
			if !errors.Is(err, errFail) || errors.Is(err, domain.ErrInvalidCredentials) {
				t.Fatalf("err = %v; want the wrapped failure", err)
			}
			if issued.Token != "" {
				t.Fatal("no token may be returned on failure")
			}
		})
	}
}
