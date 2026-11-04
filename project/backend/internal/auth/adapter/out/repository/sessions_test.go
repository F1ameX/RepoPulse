package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
)

func TestCreateAccountSessionAndFindByHash(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	account, err := NewUsers(pool).CreateAccount(ctx, uniqueLogin("session"), "hash", time.Now().UTC().Truncate(time.Microsecond))
	if err != nil {
		t.Fatal(err)
	}
	sessions := NewSessions(pool)
	issuedAt := time.Now().UTC().Truncate(time.Microsecond)
	want := domain.Session{
		PrincipalID: account.ID, Kind: domain.KindAccount, TokenHash: "token-" + uuid.NewString(),
		IssuedAt: issuedAt, ExpiresAt: issuedAt.Add(168 * time.Hour),
	}

	created, err := sessions.CreateSession(ctx, want)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Fatal("created session must have an ID")
	}
	found, err := sessions.FindSessionByTokenHash(ctx, want.TokenHash)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != created.ID || found.PrincipalID != account.ID || found.Kind != domain.KindAccount ||
		!found.IssuedAt.Equal(issuedAt) || !found.ExpiresAt.Equal(want.ExpiresAt) || found.RevokedAt != nil {
		t.Fatalf("found = %+v, created = %+v", found, created)
	}

	// Each login creates an independent session for the same account.
	second := want
	second.TokenHash = "token-" + uuid.NewString()
	if _, err := sessions.CreateSession(ctx, second); err != nil {
		t.Fatalf("second session of the same account: %v", err)
	}

	if _, err := sessions.FindSessionByTokenHash(ctx, "missing-"+uuid.NewString()); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Fatalf("unknown hash: err = %v, want ErrSessionNotFound", err)
	}
}

func TestRevokeSessionIsIdempotentAndKeepsFirstTime(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	account, err := NewUsers(pool).CreateAccount(ctx, uniqueLogin("revoke"), "hash", time.Now().UTC().Truncate(time.Microsecond))
	if err != nil {
		t.Fatal(err)
	}
	sessions := NewSessions(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	newSession := func() domain.Session {
		s, err := sessions.CreateSession(ctx, domain.Session{PrincipalID: account.ID, Kind: domain.KindAccount,
			TokenHash: "token-" + uuid.NewString(), IssuedAt: now, ExpiresAt: now.Add(time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	current, other := newSession(), newSession()

	revoked, err := sessions.RevokeSession(ctx, current.ID, now)
	if err != nil || !revoked {
		t.Fatalf("first revoke = %v, %v; want true", revoked, err)
	}
	revoked, err = sessions.RevokeSession(ctx, current.ID, now.Add(time.Minute))
	if err != nil || revoked {
		t.Fatalf("repeated revoke = %v, %v; want false without error", revoked, err)
	}
	found, err := sessions.FindSessionByTokenHash(ctx, current.TokenHash)
	if err != nil || found.RevokedAt == nil || !found.RevokedAt.Equal(now) {
		t.Fatalf("revoked_at = %v, %v; want the first revocation time", found.RevokedAt, err)
	}
	untouched, err := sessions.FindSessionByTokenHash(ctx, other.TokenHash)
	if err != nil || untouched.RevokedAt != nil {
		t.Fatalf("independent session was revoked: %+v, %v", untouched, err)
	}
	if revoked, err := sessions.RevokeSession(ctx, uuid.NewString(), now); err != nil || revoked {
		t.Fatalf("unknown session = %v, %v; want false without error", revoked, err)
	}
}
