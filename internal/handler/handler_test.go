package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	items     map[string]repository.URL
	createErr error
	getErr    error
}

func (m *testRepository) Create(_ context.Context, item repository.URL) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.items[item.Code] = item
	return nil
}
func (m *testRepository) Get(_ context.Context, code string) (repository.URL, error) {
	if m.getErr != nil {
		return repository.URL{}, m.getErr
	}
	item, ok := m.items[code]
	if !ok {
		return repository.URL{}, repository.ErrNotFound
	}
	return item, nil
}

func (m *testRepository) RecordVisit(ctx context.Context, code string) (repository.URL, error) {
	item, err := m.Get(ctx, code)
	if err != nil {
		return repository.URL{}, err
	}
	item.Visits++
	m.items[code] = item
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
	var response createResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Code == "" || response.OriginalURL != "https://example.com" || response.ShortURL != "http://localhost:8080/"+response.Code {
		t.Fatalf("unexpected response: %+v", response)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("unexpected Content-Type: %s", rec.Header().Get("Content-Type"))
	}
}

func TestCreateRejectsInvalidURL(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/shorten", strings.NewReader(`{"original_url":"not-a-url"}`))
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

func TestCreateRejectsInvalidBody(t *testing.T) {
	for name, body := range map[string]string{
		"empty":            "",
		"malformed":        "{",
		"missing URL":      "{}",
		"null":             "null",
		"array":            "[]",
		"wrong type":       `{"original_url":123}`,
		"unknown field":    `{"url":"https://example.com"}`,
		"multiple objects": `{"original_url":"https://example.com"} {}`,
		"trailing garbage": `{"original_url":"https://example.com"} invalid`,
		"too large":        `{"original_url":"https://example.com/` + strings.Repeat("a", 1<<20) + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newTestHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/shorten", strings.NewReader(body)))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestCreateAndRedirect(t *testing.T) {
	h := newTestHandler()
	rec := httptest.NewRecorder()
	original := "https://example.com/path?q=1#section"
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/shorten", strings.NewReader(`{"original_url":"`+original+`"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", rec.Code, rec.Body.String())
	}
	var created createResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/"+created.Code, nil))
		if rec.Code != http.StatusFound || rec.Header().Get("Location") != original {
			t.Fatalf("%s: status = %d, Location = %q", method, rec.Code, rec.Header().Get("Location"))
		}
		if method == http.MethodHead && rec.Body.Len() != 0 {
			t.Fatal("HEAD response must have an empty body")
		}
	}
	for range 2 {
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/urls/"+created.Code+"/stats", nil))
		var stats repository.URL
		if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
			t.Fatal(err)
		}
		if rec.Code != 200 || stats.Visits != 1 || stats.OriginalURL != original {
			t.Fatalf("stats (HEAD and stats requests must not count): %d %+v", rec.Code, stats)
		}
	}
}

func TestRoutes(t *testing.T) {
	for _, tc := range []struct {
		method string
		path   string
		status int
	}{
		{http.MethodGet, "/healthz", http.StatusOK},
		{http.MethodGet, "/api/v1/shorten", http.StatusMethodNotAllowed},
		{http.MethodDelete, "/abcdef", http.StatusMethodNotAllowed},
		{http.MethodGet, "/", http.StatusNotFound},
		{http.MethodGet, "/a/b", http.StatusNotFound},
		{http.MethodGet, "/api/v1/urls/missing/stats", http.StatusNotFound},
		{http.MethodPost, "/api/v1/urls/missing/stats", http.StatusMethodNotAllowed},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newTestHandler().ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
		})
	}
}

func TestDatabaseErrorsAreNotExposed(t *testing.T) {
	repo := &testRepository{createErr: errors.New("secret database details"), getErr: errors.New("secret database details")}
	h := New(service.New(repo, "http://localhost:8080"), slog.New(slog.NewTextHandler(io.Discard, nil))).Routes()
	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/v1/shorten", `{"original_url":"https://example.com"}`},
		{http.MethodGet, "/abcdef", ""},
		{http.MethodGet, "/api/v1/urls/abcdef/stats", ""},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
		if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "secret") {
			t.Fatalf("unexpected response: %d %s", rec.Code, rec.Body.String())
		}
	}
}
