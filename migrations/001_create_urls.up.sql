CREATE TABLE IF NOT EXISTS urls (
    code         VARCHAR(6) PRIMARY KEY,
    original_url TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_urls_created_at ON urls (created_at);
