package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)

	// Each run gets its own schema, leaving existing tables untouched.
	schema := fmt.Sprintf("shortener_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := admin.Exec(cleanupCtx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("clean up test schema: %v", err)
		}
	})
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	up, err := os.ReadFile("../../migrations/001_create_urls.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(up)); err != nil {
		t.Fatal(err)
	}
	repo := NewPostgres(pool)
	item := URL{Code: "aB12zX", OriginalURL: "https://example.com/path?q=1#section"}
	if err := repo.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, item); !errors.Is(err, ErrCodeConflict) {
		t.Fatalf("duplicate error = %v, want ErrCodeConflict", err)
	}
	got, err := repo.Get(ctx, item.Code)
	if err != nil || got != item {
		t.Fatalf("Get() = %+v, %v; want %+v", got, err, item)
	}
	if _, err := repo.Get(ctx, "absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing code error = %v, want ErrNotFound", err)
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, err := repo.Get(canceled, item.Code); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Get error = %v", err)
	}
	if err := repo.Create(canceled, URL{Code: "cancel", OriginalURL: item.OriginalURL}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Create error = %v", err)
	}
	down, err := os.ReadFile("../../migrations/001_create_urls.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(down)); err != nil {
		t.Fatal(err)
	}
}
