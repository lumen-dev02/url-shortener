package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/lumen-dev02/url-shortener/internal/repository"
)

const codeLength = 6
const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var ErrInvalidURL = errors.New("invalid URL")

type Service struct {
	repo    repository.Repository
	baseURL string
}

type Shortened struct {
	Code        string
	ShortURL    string
	OriginalURL string
}

func New(repo repository.Repository, baseURL string) *Service {
	return &Service{repo: repo, baseURL: strings.TrimRight(baseURL, "/")}
}

func (s *Service) Create(ctx context.Context, rawURL string) (Shortened, error) {
	normalized := strings.TrimSpace(rawURL)
	parsed, err := url.ParseRequestURI(normalized)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return Shortened{}, ErrInvalidURL
	}

	for attempt := 0; attempt < 5; attempt++ {
		code, err := generateCode(codeLength)
		if err != nil {
			return Shortened{}, fmt.Errorf("generate code: %w", err)
		}
		err = s.repo.Create(ctx, repository.URL{Code: code, OriginalURL: normalized})
		if err == nil {
			return Shortened{Code: code, ShortURL: s.baseURL + "/" + code, OriginalURL: normalized}, nil
		}
		if !errors.Is(err, repository.ErrCodeConflict) {
			return Shortened{}, fmt.Errorf("save URL: %w", err)
		}
	}
	return Shortened{}, fmt.Errorf("create short URL: code allocation attempts exhausted")
}

func (s *Service) Resolve(ctx context.Context, code string) (string, error) {
	item, err := s.repo.Get(ctx, code)
	if err != nil {
		return "", err
	}
	return item.OriginalURL, nil
}

func generateCode(length int) (string, error) {
	random := make([]byte, length)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	code := make([]byte, length)
	for i, value := range random {
		code[i] = alphabet[int(value)%len(alphabet)]
	}
	return string(code), nil
}
