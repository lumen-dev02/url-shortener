package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

func TestCreateURLValidation(t *testing.T) {
	for _, raw := range []string{"https://example.com", "http://example.com#fragment", "https://example.com/path?q=1#section", "http://localhost:8080", "http://127.0.0.1:8080", "http://[::1]:8080", "  https://example.com/path  "} {
		t.Run(raw, func(t *testing.T) {
			svc := New(newMemoryRepository(), "http://localhost:8080/")
			result, err := svc.Create(context.Background(), raw)
			if err != nil {
				t.Fatalf("valid URL rejected: %v", err)
			}
			if result.OriginalURL != strings.TrimSpace(raw) {
				t.Fatalf("unexpected original URL: %q", result.OriginalURL)
			}
			if result.ShortURL != "http://localhost:8080/"+result.Code {
				t.Fatalf("unexpected short URL: %q", result.ShortURL)
			}
			for _, char := range result.Code {
				if !strings.ContainsRune(alphabet, char) {
					t.Fatalf("unexpected character in code: %q", result.Code)
				}
			}
		})
	}
	for _, raw := range []string{"", " ", "example.com", "/relative", "ftp://example.com", "javascript:alert(1)", "http://", "http://:80", "http://.", "http://..", "https://bad host", "https://example.com:abc", "https://example.com/a\nb"} {
		t.Run("invalid_"+raw, func(t *testing.T) {
			svc := New(newMemoryRepository(), "http://localhost:8080")
			_, err := svc.Create(context.Background(), raw)
			if !errors.Is(err, ErrInvalidURL) {
				t.Fatalf("Create(%q) error = %v, want ErrInvalidURL", raw, err)
			}
		})
	}
}

type failingRepository struct {
	memoryRepository
	failures int
	calls    int
	err      error
}

func (r *failingRepository) Create(ctx context.Context, item repository.URL) error {
	r.calls++
	if r.calls <= r.failures {
		return r.err
	}
	return r.memoryRepository.Create(ctx, item)
}

func TestCreateRetriesCodeConflict(t *testing.T) {
	repo := &failingRepository{
		memoryRepository: *newMemoryRepository(),
		failures:         2,
		err:              fmt.Errorf("insert: %w", repository.ErrCodeConflict),
	}
	if _, err := New(repo, "http://localhost:8080").Create(context.Background(), "https://example.com"); err != nil {
		t.Fatal(err)
	}
	if repo.calls != 3 || len(repo.items) != 1 {
		t.Fatalf("calls = %d, saved items = %d", repo.calls, len(repo.items))
	}
}

func TestCreateStopsAfterFiveConflicts(t *testing.T) {
	repo := &failingRepository{failures: 10, err: repository.ErrCodeConflict}
	if _, err := New(repo, "http://localhost:8080").Create(context.Background(), "https://example.com"); err == nil {
		t.Fatal("expected an error after repeated code conflicts")
	}
	if repo.calls != 5 {
		t.Fatalf("calls = %d, want 5", repo.calls)
	}
}

func TestCreateDoesNotRetryDatabaseError(t *testing.T) {
	dbErr := errors.New("database unavailable")
	repo := &failingRepository{failures: 10, err: dbErr}
	_, err := New(repo, "http://localhost:8080").Create(context.Background(), "https://example.com")
	if !errors.Is(err, dbErr) || repo.calls != 1 {
		t.Fatalf("error = %v, calls = %d", err, repo.calls)
	}
}
