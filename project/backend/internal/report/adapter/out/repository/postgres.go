package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/F1ameX/RepoPulse/project/backend/internal/report/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

func (p *Postgres) Get(ctx context.Context, id string) (domain.Report, string, error) {
	var body []byte
	var ownerID string
	// A principal ID has one kind in Auth. Matching either live verified context
	// retains the original guest's access after login without transferring ownership.
	err := p.pool.QueryRow(ctx, `SELECT payload,owner_id::text FROM report.reports WHERE id=$1`, id).Scan(&body, &ownerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Report{}, ownerID, domain.ErrNotFound
	}
	if err != nil {
		return domain.Report{}, ownerID, errors.New("report storage unavailable")
	}
	var report domain.Report
	if json.Unmarshal(body, &report) != nil || report.ID != id {
		return domain.Report{}, ownerID, errors.New("invalid stored report")
	}
	return report, ownerID, nil
}
