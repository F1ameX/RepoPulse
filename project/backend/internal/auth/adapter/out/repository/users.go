package repository

import (
	"context"

	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Users struct {
	pool *pgxpool.Pool
}

func NewUsers(pool *pgxpool.Pool) *Users {
	return &Users{
		pool: pool,
	}
}

func (u *Users) CreateAccount(ctx context.Context, login, passwordHash string) (domain.Principal, error) {
	return domain.Principal{}, domain.ErrNotImplemented
}

func (u *Users) FindAccountByLogin(ctx context.Context, login string) (domain.Principal, error) {
	return domain.Principal{}, domain.ErrNotImplemented
}
