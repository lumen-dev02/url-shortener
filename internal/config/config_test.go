package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/shortener")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("BASE_URL", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":8080" || cfg.BaseURL != "http://localhost:8080" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadEnvironment(t *testing.T) {
	t.Setenv("DATABASE_URL", " postgres://localhost/shortener ")
	t.Setenv("HTTP_ADDR", " 127.0.0.1:9090 ")
	t.Setenv("BASE_URL", " https://short.example/ ")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != "127.0.0.1:9090" || cfg.BaseURL != "https://short.example" || cfg.DatabaseURL != "postgres://localhost/shortener" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", " ")
	if _, err := Load(); err == nil {
		t.Fatal("expected an error for missing DATABASE_URL")
	}
}

func TestLoadRejectsInvalidBaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/shortener")
	for _, value := range []string{"example.com", "ftp://example.com", "http://:80", "http://.", "http://%", "https://user:pass@example.com", "https://example.com?q=1", "https://example.com?", "https://example.com#fragment", "https://example.com#"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("BASE_URL", value)
			if _, err := Load(); err == nil {
				t.Fatalf("expected an error for BASE_URL %q", value)
			}
		})
	}
}

func TestLoadRejectsBaseURLPath(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/shortener")
	for _, value := range []string{"https://short.example/links", "https://short.example/links/", "https://short.example/%2F"} {
		t.Setenv("BASE_URL", value)
		if _, err := Load(); err == nil {
			t.Errorf("accepted BASE_URL with a path: %q", value)
		}
	}
}
