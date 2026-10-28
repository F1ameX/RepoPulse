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
