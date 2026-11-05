package handler

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lumen-dev02/url-shortener/internal/repository"
	"github.com/lumen-dev02/url-shortener/internal/service"
)

type testRepository struct {
	items map[string]repository.URL
}

func (m *testRepository) Create(_ context.Context, item repository.URL) error {
	m.items[item.Code] = item
	return nil
}
func (m *testRepository) Get(_ context.Context, code string) (repository.URL, error) {
	item, ok := m.items[code]
	if !ok {
		return repository.URL{}, repository.ErrNotFound
	}
	return item, nil
}

func newTestHandler() http.Handler {
	repo := &testRepository{items: make(map[string]repository.URL)}
	svc := service.New(repo, "http://localhost:8080")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(svc, logger).Routes()
}

func TestCreateReturnsCreated(t *testing.T) {
	body := bytes.NewBufferString(`{"original_url":"https://example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shorten", body)
	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "short_url") {
		t.Fatalf("response does not contain short_url: %s", rec.Body.String())
	}
}

func TestCreateRejectsInvalidURL(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shorten", strings.NewReader(`{"url":"not-a-url"}`))
	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRedirectReturnsNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
