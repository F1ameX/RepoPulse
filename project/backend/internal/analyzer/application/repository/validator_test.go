package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type sourceFunc func(context.Context, domain.RepositoryURL) (domain.ValidatedRepository, error)

func (f sourceFunc) Lookup(ctx context.Context, address domain.RepositoryURL) (domain.ValidatedRepository, error) {
	return f(ctx, address)
}

func validCommand(rawURL string) ValidateRepositoryCommand {
	guestID := "cfe8ce89-aec5-4773-a951-6d1b45420258"
	expiresAt := time.Now().Add(time.Hour)
	return ValidateRepositoryCommand{
		Context: domain.VerifiedContext{GuestSessionID: &guestID, GuestExpiresAt: &expiresAt},
		Request: ValidateRepositoryRequest{RepositoryURL: rawURL},
	}
}

func TestValidationRequiresContextBeforeCallingGitHub(t *testing.T) {
	validator := NewValidator(sourceFunc(func(context.Context, domain.RepositoryURL) (domain.ValidatedRepository, error) {
		t.Fatal("GitHub called without a verified context")
		return domain.ValidatedRepository{}, nil
	}), time.Second)
	_, err := validator.ValidateRepository(context.Background(), ValidateRepositoryCommand{
		Request: ValidateRepositoryRequest{RepositoryURL: "https://github.com/a/b"},
	})
	if !errors.Is(err, domain.ErrSessionRequired) {
		t.Fatal(err)
	}
}

func TestValidationRejectsURLBeforeCallingGitHub(t *testing.T) {
	validator := NewValidator(sourceFunc(func(context.Context, domain.RepositoryURL) (domain.ValidatedRepository, error) {
		t.Fatal("GitHub called for invalid URL")
		return domain.ValidatedRepository{}, nil
	}), time.Second)
	_, err := validator.ValidateRepository(context.Background(), validCommand("https://github.com/a/b/tree/main"))
	if !errors.Is(err, domain.ErrInvalidURL) {
		t.Fatal(err)
	}
}

func TestValidationPassesNormalizedURLAndBoundsSource(t *testing.T) {
	validator := NewValidator(sourceFunc(func(ctx context.Context, address domain.RepositoryURL) (domain.ValidatedRepository, error) {
		if address.CanonicalURL() != "https://github.com/a/b" {
			t.Fatalf("address: %+v", address)
		}
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > time.Second {
			t.Fatal("missing validation budget")
		}
		return domain.ValidatedRepository{}, domain.ErrRepositoryUnavailable
	}), time.Second)
	_, err := validator.ValidateRepository(context.Background(), validCommand(" https://GitHub.com/A/B.git/ "))
	if !errors.Is(err, domain.ErrRepositoryUnavailable) {
		t.Fatalf("source error lost: %v", err)
	}
}
