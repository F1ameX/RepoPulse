package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
)

type fakeHasher struct {
	hashed    []string
	err       error
	verified  []string // encoded hashes passed to Verify
	match     bool
	verifyErr error
}

func (h *fakeHasher) Hash(password string) (string, error) {
	h.hashed = append(h.hashed, password)
	return "hash-of-password", h.err
}

func (h *fakeHasher) Verify(_, encoded string) (bool, error) {
	h.verified = append(h.verified, encoded)
	return h.match, h.verifyErr
}

type fakeUsers struct {
	calls     int
	login     string
	hash      string
	createdAt time.Time
	err       error
	account   *domain.Principal // returned by FindAccountByLogin; nil means unknown login
	findErr   error
}

func (u *fakeUsers) CreateAccount(_ context.Context, login, hash string, createdAt time.Time) (domain.Principal, error) {
	u.calls++
	u.login, u.hash, u.createdAt = login, hash, createdAt
	if u.err != nil {
		return domain.Principal{}, u.err
	}
	return domain.Principal{ID: "account-id", Kind: domain.KindAccount, Login: login, PasswordHash: hash, CreatedAt: createdAt}, nil
}

func (u *fakeUsers) FindAccountByID(_ context.Context, id string) (domain.Principal, error) {
	switch {
	case u.findErr != nil:
		return domain.Principal{}, u.findErr
	case u.account == nil || u.account.ID != id:
		return domain.Principal{}, domain.ErrPrincipalNotFound
	}
	return *u.account, nil
}

func (u *fakeUsers) FindAccountByLogin(_ context.Context, login string) (domain.Principal, error) {
	u.login = login
	switch {
	case u.findErr != nil:
		return domain.Principal{}, u.findErr
	case u.account == nil:
		return domain.Principal{}, domain.ErrInvalidCredentials
	}
	return *u.account, nil
}

func newRegisterService(users *fakeUsers, hasher *fakeHasher) *Service {
	return NewService(nil, users, nil, nil, func() time.Time { return testNow.Add(1500 * time.Nanosecond) }, testOptions, hasher)
}

func TestRegisterCreatesAccount(t *testing.T) {
	users, hasher := &fakeUsers{}, &fakeHasher{}
	result, err := newRegisterService(users, hasher).Register(context.Background(), " FlameX ", "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if result.Login != "flamex" || result.Message != "Аккаунт создан. Войдите в систему" {
		t.Fatalf("result = %+v", result)
	}
	if users.login != "flamex" || users.hash != "hash-of-password" {
		t.Fatalf("stored login=%q hash=%q; want normalized login and the hash, never the password", users.login, users.hash)
	}
	if len(hasher.hashed) != 1 || hasher.hashed[0] != "correct horse battery" {
		t.Fatalf("hashed %q", hasher.hashed)
	}
	if !users.createdAt.Equal(testNow.Add(time.Microsecond)) {
		t.Fatalf("created_at = %v; want UTC truncated to microseconds", users.createdAt)
	}
}

func TestRegisterValidationKeepsFieldErrors(t *testing.T) {
	users, hasher := &fakeUsers{}, &fakeHasher{}
	_, err := newRegisterService(users, hasher).Register(context.Background(), "", "short")
	validation, ok := errors.AsType[*domain.ValidationError](err)
	if !ok || len(validation.Fields) != 2 {
		t.Fatalf("err = %#v; want ValidationError with login and password errors", err)
	}
	if users.calls != 0 || len(hasher.hashed) != 0 {
		t.Fatal("storage and hashing must not run before both fields pass validation")
	}
}

func TestRegisterLoginTaken(t *testing.T) {
	_, err := newRegisterService(&fakeUsers{err: domain.ErrLoginTaken}, &fakeHasher{}).
		Register(context.Background(), "flamex", "correct horse battery")
	if !errors.Is(err, domain.ErrLoginTaken) {
		t.Fatalf("err = %v, want ErrLoginTaken", err)
	}
}

func TestRegisterStorageFailureIsNotLoginTaken(t *testing.T) {
	errStore := errors.New("database unavailable")
	_, err := newRegisterService(&fakeUsers{err: errStore}, &fakeHasher{}).
		Register(context.Background(), "flamex", "correct horse battery")
	if !errors.Is(err, errStore) || errors.Is(err, domain.ErrLoginTaken) {
		t.Fatalf("err = %v; want the wrapped storage error", err)
	}
}

func TestRegisterHashFailure(t *testing.T) {
	users := &fakeUsers{}
	_, err := newRegisterService(users, &fakeHasher{err: errors.New("no entropy")}).
		Register(context.Background(), "flamex", "correct horse battery")
	if err == nil || users.calls != 0 {
		t.Fatalf("err = %v, calls = %d; account must not be stored without a hash", err, users.calls)
	}
}
