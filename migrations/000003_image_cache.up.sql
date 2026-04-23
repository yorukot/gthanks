CREATE TABLE IF NOT EXISTS image_cache (
    cache_key TEXT PRIMARY KEY,
    target_id INTEGER NOT NULL,
    image_png BLOB NOT NULL,
    cache_status TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_image_cache_expires_at ON image_cache (expires_at);
