package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/lumen-dev02/url-shortener/internal/repository"
	"github.com/lumen-dev02/url-shortener/internal/service"
)

type Handler struct {
	service *service.Service
	logger  *slog.Logger
}

func New(shortener *service.Service, logger *slog.Logger) *Handler {
	return &Handler{service: shortener, logger: logger}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/shorten", h.create)
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /api/v1/urls/{code}/stats", h.stats)
	mux.HandleFunc("GET /{code}", h.redirect)
	return mux
}

type createRequest struct {
	OriginalURL string `json:"original_url"`
}

type createResponse struct {
	Code        string `json:"code"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var request createRequest
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "request must contain a single JSON object"})
		return
	}

	result, err := h.service.Create(r.Context(), request.OriginalURL)
	if errors.Is(err, service.ErrInvalidURL) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "url must be a valid HTTP or HTTPS URL"})
		return
	}
	if err != nil {
		h.logger.Error("create short URL", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, createResponse{
		Code: result.Code, ShortURL: result.ShortURL, OriginalURL: result.OriginalURL,
	})
}

func (h *Handler) redirect(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimSpace(r.PathValue("code"))
	if code == "" {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "short URL not found"})
		return
	}
	var originalURL string
	var err error
	if r.Method == http.MethodHead {
		var item repository.URL
		item, err = h.service.Stats(r.Context(), code)
		originalURL = item.OriginalURL
	} else {
		originalURL, err = h.service.Resolve(r.Context(), code)
	}
	if errors.Is(err, repository.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "short URL not found"})
		return
	}
	if err != nil {
		h.logger.Error("resolve short URL", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		return
	}
	http.Redirect(w, r, originalURL, http.StatusFound)
}

func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.Stats(r.Context(), r.PathValue("code"))
	if errors.Is(err, repository.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "short URL not found"})
		return
	}
	if err != nil {
		h.logger.Error("get URL stats", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
