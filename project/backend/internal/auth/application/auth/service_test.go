package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
)

type fakeGenerator struct {
	token, hash string
	err         error
}

func (g fakeGenerator) Generate() (string, string, error) { return g.token, g.hash, g.err }
func (g fakeGenerator) Hash(string) string                { return g.hash }

type fakeGuests struct {
	calls               int
	hash                string
	issuedAt, expiresAt time.Time
	err                 error
}

func (g *fakeGuests) CreateGuestSession(_ context.Context, hash string, issuedAt, expiresAt time.Time) (domain.Session, error) {
	g.calls++
	g.hash, g.issuedAt, g.expiresAt = hash, issuedAt, expiresAt
	if g.err != nil {
		return domain.Session{}, g.err
	}
	return domain.Session{
		ID: "session-id", PrincipalID: "principal-id", Kind: domain.KindGuest,
		TokenHash: hash, IssuedAt: issuedAt, ExpiresAt: expiresAt,
	}, nil
}

var testNow = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

func TestCreateGuestSession(t *testing.T) {
	guests := &fakeGuests{}
	svc := NewService(guests, nil, nil, fakeGenerator{token: "raw-token", hash: "token-hash"},
		func() time.Time { return testNow })

	issued, err := svc.CreateGuestSession(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if guests.calls != 1 || guests.hash != "token-hash" {
		t.Fatalf("repository got calls=%d hash=%q; only the hash must be stored", guests.calls, guests.hash)
	}
	if !guests.issuedAt.Equal(testNow) || !guests.expiresAt.Equal(testNow.Add(24*time.Hour)) {
		t.Fatalf("issued_at=%v expires_at=%v, want now and now+24h", guests.issuedAt, guests.expiresAt)
	}
	if issued.Token != "raw-token" {
		t.Fatalf("token = %q, want the raw token for the cookie", issued.Token)
	}
	want := domain.SessionInfo{Kind: domain.KindGuest, ExpiresAt: testNow.Add(24 * time.Hour)}
	if issued.Session.Kind != want.Kind || issued.Session.Login != nil || !issued.Session.ExpiresAt.Equal(want.ExpiresAt) {
		t.Fatalf("session = %+v, want guest without login until %v", issued.Session, want.ExpiresAt)
	}
}

func TestCreateGuestSessionErrors(t *testing.T) {
	errGenerate := errors.New("entropy unavailable")
	errStore := errors.New("database unavailable")

	t.Run("generator", func(t *testing.T) {
		guests := &fakeGuests{}
		svc := NewService(guests, nil, nil, fakeGenerator{err: errGenerate}, func() time.Time { return testNow })
		if _, err := svc.CreateGuestSession(context.Background()); !errors.Is(err, errGenerate) {
			t.Fatalf("err = %v, want %v", err, errGenerate)
		}
		if guests.calls != 0 {
			t.Fatal("guest must not be stored without a token")
		}
	})

	t.Run("repository", func(t *testing.T) {
		guests := &fakeGuests{err: errStore}
		svc := NewService(guests, nil, nil, fakeGenerator{token: "raw-token", hash: "token-hash"},
			func() time.Time { return testNow })
		issued, err := svc.CreateGuestSession(context.Background())
		if !errors.Is(err, errStore) {
			t.Fatalf("err = %v, want %v", err, errStore)
		}
		if issued.Token != "" {
			t.Fatal("token must not be returned when the session was not stored")
		}
	})
}
