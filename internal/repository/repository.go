package repository

import (
	"context"
	"errors"
)

var (
	ErrNotFound = errors.New("url not found")
	ErrCodeConflict = errors.New("short code already exists")
)

type URL struct {
	Code        string
	OriginalURL string
}

type Repository interface {
	Create(ctx context.Context, item URL) error
	Get(ctx context.Context, code string) (URL, error)
}
