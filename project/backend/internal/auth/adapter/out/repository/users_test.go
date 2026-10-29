package repository

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
)

// authMigrations is the auth migration directory relative to this package.
const authMigrations = "../../../../../migrations/auth"

// testPool creates a disposable database on the REPOPULSE_TEST_DATABASE_URL
// server, applies the Up part of every auth migration and drops the database
// after the test. The shared database is never modified, so the test works on
// an empty CI PostgreSQL and on a developer database alike.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("REPOPULSE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("REPOPULSE_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("invalid test database configuration")
	}
	name := "repopulse_auth_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.Exec(ctx, "DROP DATABASE "+quoted+" WITH (FORCE)"); err != nil {
			t.Errorf("drop isolated test database: %v", err)
		}
		admin.Close()
	})

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid test database configuration")
	}
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close) // runs before the DROP above

	files, err := filepath.Glob(filepath.Join(authMigrations, "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("auth migrations not found: %v", err)
	}
	sort.Strings(files)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		up, _, ok := strings.Cut(string(data), "-- +goose Down")
		if !ok {
			t.Fatalf("%s has no Down section", file)
		}
		if _, err := pool.Exec(ctx, strings.TrimPrefix(up, "-- +goose Up")); err != nil {
			t.Fatalf("apply %s: %v", file, err)
		}
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
