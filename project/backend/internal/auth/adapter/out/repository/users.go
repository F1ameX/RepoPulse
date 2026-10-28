package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
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

func (u *Users) CreateAccount(ctx context.Context, login, passwordHash string, createdAt time.Time) (domain.Principal, error) {
	p := domain.Principal{
		ID: uuid.NewString(), Kind: domain.KindAccount,
		Login: login, PasswordHash: passwordHash,
		CreatedAt: createdAt,
	}

	_, err := u.pool.Exec(ctx, 
		`INSERT INTO auth.principals 
		(id, kind, login, password_hash, created_at)
		VALUES ($1, 'account', $2, $3, $4)`,
		p.ID, p.Login, p.PasswordHash, p.CreatedAt)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation{ //23505
		return domain.Principal{}, domain.ErrLoginTaken 
	}
	if err != nil {
		return domain.Principal{}, fmt.Errorf("insert account: %w", err)
	}

	return p, nil
}

func (u *Users) FindAccountByLogin(ctx context.Context, login string) (domain.Principal, error) {
	return domain.Principal{}, domain.ErrNotImplemented
}
