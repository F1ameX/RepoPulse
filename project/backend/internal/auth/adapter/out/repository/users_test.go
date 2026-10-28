package repository

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
)

// testPool connects to a separate test database with the auth migration applied.
// The test is skipped when AUTH_TEST_DATABASE_URL is not set.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("AUTH_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("AUTH_TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	return pool
}

// uniqueLogin keeps tests independent without truncating shared tables.
func uniqueLogin(prefix string) string {
	return prefix + "_" + uuid.NewString()[:8]
}

func TestCreateAndFindAccount(t *testing.T) {
	users := NewUsers(testPool(t))
	ctx := context.Background()
	login := uniqueLogin("user")
	createdAt := time.Now().UTC().Truncate(time.Microsecond)

	created, err := users.CreateAccount(ctx, login, "stored-hash", createdAt)
	if err != nil {
		t.Fatal(err)
	}
	found, err := users.FindAccountByLogin(ctx, login)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != created.ID || found.Kind != domain.KindAccount || found.Login != login ||
		found.PasswordHash != "stored-hash" || !found.CreatedAt.Equal(createdAt) {
		t.Fatalf("found = %+v, created = %+v", found, created)
	}

	if _, err := users.FindAccountByLogin(ctx, uniqueLogin("missing")); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("unknown login: err = %v, want ErrInvalidCredentials", err)
	}
	if _, err := users.CreateAccount(ctx, login, "other-hash", createdAt); !errors.Is(err, domain.ErrLoginTaken) {
		t.Fatalf("repeated login: err = %v, want ErrLoginTaken", err)
	}
}

// FR-27 acceptance: concurrent registrations of one normalized login create at most one account.
func TestConcurrentCreateAccountCreatesOneAccount(t *testing.T) {
	pool := testPool(t)
	users := NewUsers(pool)
	login := uniqueLogin("racer")
	const attempts = 10

	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, attempts)
	for range attempts {
		wg.Go(func() {
			<-start
			_, err := users.CreateAccount(context.Background(), login, "hash", time.Now().UTC())
			errs <- err
		})
	}
	close(start)
	wg.Wait()
	close(errs)

	created := 0
	for err := range errs {
		switch {
		case err == nil:
			created++
		case !errors.Is(err, domain.ErrLoginTaken):
			t.Fatalf("unexpected error: %v", err)
		}
	}
	var rows int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM auth.principals WHERE login = $1`, login).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if created != 1 || rows != 1 {
		t.Fatalf("created = %d, rows = %d; want exactly one account", created, rows)
	}
}

func TestFindAccountByID(t *testing.T) {
	users := NewUsers(testPool(t))
	ctx := context.Background()
	login := uniqueLogin("byid")
	created, err := users.CreateAccount(ctx, login, "stored-hash", time.Now().UTC().Truncate(time.Microsecond))
	if err != nil {
		t.Fatal(err)
	}
	found, err := users.FindAccountByID(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != created.ID || found.Login != login || found.Kind != domain.KindAccount {
		t.Fatalf("found = %+v, want %+v", found, created)
	}
	if _, err := users.FindAccountByID(ctx, uuid.NewString()); !errors.Is(err, domain.ErrPrincipalNotFound) {
		t.Fatalf("unknown id: err = %v, want ErrPrincipalNotFound", err)
	}
}
