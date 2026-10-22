// Package repository implements repository validation without creating analysis jobs.
package repository

import (
	"context"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/analyzer/domain"
)

type Source interface {
	Lookup(context.Context, domain.RepositoryURL) (domain.ValidatedRepository, error)
}

type ValidateRepositoryRequest struct {
	RepositoryURL string
}

type ValidateRepositoryCommand struct {
	Context domain.VerifiedContext
	Request ValidateRepositoryRequest
}

type Validator struct {
	source Source
	budget time.Duration
}

func NewValidator(source Source, budget time.Duration) *Validator {
	return &Validator{source: source, budget: budget}
}

func (v *Validator) ValidateRepository(ctx context.Context, command ValidateRepositoryCommand) (domain.ValidatedRepository, error) {
	if err := command.Context.Validate(time.Now()); err != nil {
		return domain.ValidatedRepository{}, err
	}
	address, err := domain.ParseRepositoryURL(command.Request.RepositoryURL)
	if err != nil {
		return domain.ValidatedRepository{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, v.budget)
	defer cancel()
	return v.source.Lookup(ctx, address)
}
