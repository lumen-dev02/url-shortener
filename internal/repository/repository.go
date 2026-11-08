package repository

import (
	"context"
	"errors"
)

var (
	ErrNotFound     = errors.New("url not found")
	ErrCodeConflict = errors.New("short code already exists")
)

type URL struct {
	Code        string `json:"code"`
	OriginalURL string `json:"original_url"`
	Visits      int64  `json:"visits"`
}

type Repository interface {
	Create(ctx context.Context, item URL) error
	Get(ctx context.Context, code string) (URL, error)
	RecordVisit(ctx context.Context, code string) (URL, error)
}
