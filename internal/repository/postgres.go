package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

func (p *Postgres) Create(ctx context.Context, item URL) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	_, err := p.pool.Exec(ctx,
		"INSERT INTO urls (code, original_url) VALUES ($1, $2)",
		item.Code, item.OriginalURL,
	)
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrCodeConflict
	}
	return err
}

func (p *Postgres) Get(ctx context.Context, code string) (URL, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var item URL
	err := p.pool.QueryRow(ctx,
		"SELECT code, original_url, visits FROM urls WHERE code = $1",
		code,
	).Scan(&item.Code, &item.OriginalURL, &item.Visits)
	if errors.Is(err, pgx.ErrNoRows) {
		return URL{}, ErrNotFound
	}
	if err != nil {
		return URL{}, err
	}
	return item, nil
}

// RecordVisit increments in PostgreSQL so concurrent redirects cannot lose updates.
func (p *Postgres) RecordVisit(ctx context.Context, code string) (URL, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var item URL
	err := p.pool.QueryRow(ctx,
		"UPDATE urls SET visits = visits + 1 WHERE code = $1 RETURNING code, original_url, visits", code,
	).Scan(&item.Code, &item.OriginalURL, &item.Visits)
	if errors.Is(err, pgx.ErrNoRows) {
		return URL{}, ErrNotFound
	}
	return item, err
}
