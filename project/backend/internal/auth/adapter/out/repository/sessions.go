package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Sessions struct {
	pool *pgxpool.Pool
}

func NewSessions(pool *pgxpool.Pool) *Sessions {
	return &Sessions{
		pool: pool,
	}
}

func (r *Sessions) CreateSession(ctx context.Context, session domain.Session) (domain.Session, error) {
	session.ID = uuid.NewString()

	_, err := r.pool.Exec(ctx,
		`INSERT INTO auth.sessions
		(id, principal_id, kind, token_hash, issued_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)`, session.ID, session.PrincipalID,
		session.Kind, session.TokenHash, session.IssuedAt, session.ExpiresAt)
	if err != nil {
		return domain.Session{}, fmt.Errorf("insert new session: %w", err)
	}
	return session, nil

}

func (r *Sessions) FindSessionByTokenHash(ctx context.Context, tokenHash string) (domain.Session, error) {
	var s domain.Session
	err := r.pool.QueryRow(ctx,
		`SELECT id, principal_id, kind, token_hash,
	issued_at, expires_at, revoked_at
	FROM auth.sessions
	WHERE token_hash = $1`, tokenHash,
	).Scan(&s.ID, &s.PrincipalID, &s.Kind, &s.TokenHash,
		&s.IssuedAt, &s.ExpiresAt, &s.RevokedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Session{}, domain.ErrSessionNotFound
	}

	if err != nil {
		return domain.Session{}, err
	}

	return s, nil
}

func (r *Sessions) RevokeSession(ctx context.Context, sessionID string, at time.Time) (bool, error) {
	// revoked_at IS NULL makes a repeated logout a no-op that keeps the first revocation time.
	tag, err := r.pool.Exec(ctx,
		`UPDATE auth.sessions SET revoked_at = $2 WHERE id = $1 AND revoked_at IS NULL`, sessionID, at)
	if err != nil {
		return false, fmt.Errorf("revoke session: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *Sessions) CreateGuestSession(ctx context.Context, hash string, issuedAt, expiresAt time.Time) (domain.Session, error) {
	s := domain.Session{
		ID: uuid.NewString(), PrincipalID: uuid.NewString(),
		Kind: domain.KindGuest, TokenHash: hash, IssuedAt: issuedAt,
		ExpiresAt: expiresAt,
	}

	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO auth.principals 
		(id, kind, created_at) VALUES ($1, 'guest', $2)`,
			s.PrincipalID, issuedAt); err != nil {
			return fmt.Errorf("insert guest principal: %w", err)
		}

		_, err := tx.Exec(ctx, `INSERT INTO auth.sessions (id, 
		principal_id, kind, token_hash, issued_at, expires_at)
		VALUES ($1, $2, 'guest', $3, $4, $5)`,
			s.ID, s.PrincipalID, hash, issuedAt, expiresAt)

		return err
	})
	if err != nil {
		return domain.Session{}, err
	}
	return s, nil
}
