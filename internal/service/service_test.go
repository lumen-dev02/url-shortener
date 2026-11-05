package service

import (
	"context"
	"errors"
	"testing"

	"github.com/lumen-dev02/url-shortener/internal/repository"
)

type memoryRepository struct {
	items map[string]repository.URL
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{items: make(map[string]repository.URL)}
}

func (m *memoryRepository) Create(_ context.Context, item repository.URL) error {
	if _, exists := m.items[item.Code]; exists {
		return repository.ErrCodeConflict
	}
	m.items[item.Code] = item
	return nil
}

func (m *memoryRepository) Get(_ context.Context, code string) (repository.URL, error) {
	item, ok := m.items[code]
	if !ok {
		return repository.URL{}, repository.ErrNotFound
	}
	return item, nil
}

func TestCreateAndResolve(t *testing.T) {
	repo := newMemoryRepository()
	svc := New(repo, "http://localhost:8080")
	created, err := svc.Create(context.Background(), "https://example.com/path")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(created.Code) != codeLength {
		t.Fatalf("code length = %d, want %d", len(created.Code), codeLength)
	}
	if created.ShortURL != "http://localhost:8080/"+created.Code {
		t.Fatalf("unexpected short URL: %s", created.ShortURL)
	}
	resolved, err := svc.Resolve(context.Background(), created.Code)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved != "https://example.com/path" {
		t.Fatalf("resolved URL = %q", resolved)
	}
}

func TestCreateRejectsInvalidURL(t *testing.T) {
	svc := New(newMemoryRepository(), "http://localhost:8080")
	_, err := svc.Create(context.Background(), "javascript:alert(1)")
	if !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("Create() error = %v, want ErrInvalidURL", err)
	}
}

func TestResolveNotFound(t *testing.T) {
	svc := New(newMemoryRepository(), "http://localhost:8080")
	_, err := svc.Resolve(context.Background(), "missing")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("Resolve() error = %v, want ErrNotFound", err)
	}
}
