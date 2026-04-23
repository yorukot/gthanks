CREATE TABLE IF NOT EXISTS targets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_text TEXT NOT NULL,
    normalized_target TEXT NOT NULL UNIQUE,
    target_type TEXT NOT NULL,
    owner TEXT NOT NULL,
    repo TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    last_requested_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS repos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    full_name TEXT NOT NULL UNIQUE,
    owner TEXT NOT NULL,
    name TEXT NOT NULL,
    html_url TEXT NOT NULL,
    private INTEGER NOT NULL DEFAULT 0,
    archived INTEGER NOT NULL DEFAULT 0,
    fork INTEGER NOT NULL DEFAULT 0,
    last_synced_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS target_repos (
    target_id INTEGER NOT NULL,
    repo_id INTEGER NOT NULL,
    discovered_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    PRIMARY KEY (target_id, repo_id),
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE,
    FOREIGN KEY (repo_id) REFERENCES repos(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS contributors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    identity_key TEXT NOT NULL UNIQUE,
    github_user_id INTEGER,
    login TEXT,
    avatar_url TEXT,
    html_url TEXT,
    type TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS repo_contributors (
    repo_id INTEGER NOT NULL,
    contributor_id INTEGER NOT NULL,
    contributions INTEGER NOT NULL,
    fetched_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (repo_id, contributor_id),
    FOREIGN KEY (repo_id) REFERENCES repos(id) ON DELETE CASCADE,
    FOREIGN KEY (contributor_id) REFERENCES contributors(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS query_cache (
    cache_key TEXT PRIMARY KEY,
    target_id INTEGER NOT NULL,
    response_json BLOB NOT NULL,
    cache_status TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE
);
