CREATE TABLE IF NOT EXISTS avatar_cache (
    avatar_url TEXT PRIMARY KEY,
    content BLOB NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_avatar_cache_expires_at ON avatar_cache (expires_at);
