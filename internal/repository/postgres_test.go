package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
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
	visitsMigration, err := os.ReadFile("../../migrations/002_add_visits.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	// Existing links must survive the upgrade with a zero counter.
	if _, err := pool.Exec(ctx, "INSERT INTO urls(code, original_url) VALUES ('legacy', 'https://example.com')"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(visitsMigration)); err != nil {
		t.Fatal(err)
	}
	repo := NewPostgres(pool)
	if old, err := repo.Get(ctx, "legacy"); err != nil || old.Visits != 0 {
		t.Fatalf("upgraded link = %+v, %v", old, err)
	}
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
	var wg sync.WaitGroup
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := repo.RecordVisit(ctx, item.Code); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if got, err := repo.Get(ctx, item.Code); err != nil || got.Visits != 25 {
		t.Fatalf("concurrent visits = %+v, %v", got, err)
	}
	if _, err := repo.RecordVisit(ctx, "absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing visit: %v", err)
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, err := repo.RecordVisit(canceled, item.Code); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled visit: %v", err)
	}
	if _, err := repo.Get(canceled, item.Code); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Get error = %v", err)
	}
	if err := repo.Create(canceled, URL{Code: "cancel", OriginalURL: item.OriginalURL}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Create error = %v", err)
	}
	downVisits, err := os.ReadFile("../../migrations/002_add_visits.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(downVisits)); err != nil {
		t.Fatal(err)
	}
	var original string
	if err := pool.QueryRow(ctx, "SELECT original_url FROM urls WHERE code=$1", item.Code).Scan(&original); err != nil || original != item.OriginalURL {
		t.Fatalf("rollback lost link: %q, %v", original, err)
	}
	down, err := os.ReadFile("../../migrations/001_create_urls.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(down)); err != nil {
		t.Fatal(err)
	}
}
