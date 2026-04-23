CREATE INDEX IF NOT EXISTS idx_query_cache_expires_at ON query_cache (expires_at);
CREATE INDEX IF NOT EXISTS idx_repos_owner ON repos (owner);
CREATE INDEX IF NOT EXISTS idx_target_repos_repo_id ON target_repos (repo_id);
