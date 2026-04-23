package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gthanks/internal/domain"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) GetQueryCache(ctx context.Context, cacheKey string) (*domain.CachedResponse, error) {
	const query = `
SELECT response_json, expires_at
FROM query_cache
WHERE cache_key = ?`
	row := s.db.QueryRowContext(ctx, query, cacheKey)

	var raw []byte
	var expiresAt string
	if err := row.Scan(&raw, &expiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	parsed, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return nil, err
	}

	return &domain.CachedResponse{
		ResponseJSON: raw,
		ExpiresAt:    parsed,
	}, nil
}

func (s *Store) GetImageCache(ctx context.Context, cacheKey string) (*domain.CachedBinary, error) {
	const query = `
SELECT image_png, expires_at
FROM image_cache
WHERE cache_key = ?`
	row := s.db.QueryRowContext(ctx, query, cacheKey)

	var raw []byte
	var expiresAt string
	if err := row.Scan(&raw, &expiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	parsed, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return nil, err
	}

	return &domain.CachedBinary{
		Content:   raw,
		ExpiresAt: parsed,
	}, nil
}

func (s *Store) GetAvatarCache(ctx context.Context, avatarURL string) (*domain.CachedBinary, error) {
	const query = `
SELECT content, expires_at
FROM avatar_cache
WHERE avatar_url = ?`
	row := s.db.QueryRowContext(ctx, query, avatarURL)

	var raw []byte
	var expiresAt string
	if err := row.Scan(&raw, &expiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	parsed, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return nil, err
	}

	return &domain.CachedBinary{
		Content:   raw,
		ExpiresAt: parsed,
	}, nil
}

func (s *Store) SaveQueryCache(ctx context.Context, record domain.QueryCacheRecord) error {
	targetID, err := s.upsertTarget(ctx, record.Target)
	if err != nil {
		return err
	}

	const query = `
INSERT INTO query_cache (cache_key, target_id, response_json, cache_status, created_at, updated_at, expires_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(cache_key) DO UPDATE SET
	target_id = excluded.target_id,
	response_json = excluded.response_json,
	cache_status = excluded.cache_status,
	updated_at = excluded.updated_at,
	expires_at = excluded.expires_at`
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(
		ctx,
		query,
		record.CacheKey,
		targetID,
		record.ResponseJSON,
		record.CacheStatus,
		now,
		now,
		record.ExpiresAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) SaveImageCache(ctx context.Context, record domain.ImageCacheRecord) error {
	targetID, err := s.upsertTarget(ctx, record.Target)
	if err != nil {
		return err
	}

	const query = `
INSERT INTO image_cache (cache_key, target_id, image_png, cache_status, created_at, updated_at, expires_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(cache_key) DO UPDATE SET
	target_id = excluded.target_id,
	image_png = excluded.image_png,
	cache_status = excluded.cache_status,
	updated_at = excluded.updated_at,
	expires_at = excluded.expires_at`
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(
		ctx,
		query,
		record.CacheKey,
		targetID,
		record.ImagePNG,
		record.CacheStatus,
		now,
		now,
		record.ExpiresAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) SaveAvatarCache(ctx context.Context, record domain.AvatarCacheRecord) error {
	const query = `
INSERT INTO avatar_cache (avatar_url, content, created_at, updated_at, expires_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(avatar_url) DO UPDATE SET
	content = excluded.content,
	updated_at = excluded.updated_at,
	expires_at = excluded.expires_at`
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(
		ctx,
		query,
		record.AvatarURL,
		record.Content,
		now,
		now,
		record.ExpiresAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) SaveRepoSnapshot(ctx context.Context, target domain.Target, repo domain.Repo) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	targetID, err := s.upsertTargetTx(ctx, tx, target)
	if err != nil {
		return err
	}
	repoID, err := s.upsertRepoTx(ctx, tx, repo)
	if err != nil {
		return err
	}
	if err := s.upsertTargetRepoTx(ctx, tx, targetID, repoID); err != nil {
		return err
	}
	if err := s.replaceRepoContributorsTx(ctx, tx, repoID, repo.Contributors); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) upsertTarget(ctx context.Context, target domain.Target) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	id, err := s.upsertTargetTx(ctx, tx, target)
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func (s *Store) upsertTargetTx(ctx context.Context, tx *sql.Tx, target domain.Target) (int64, error) {
	const stmt = `
INSERT INTO targets (target_text, normalized_target, target_type, owner, repo, created_at, updated_at, last_requested_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(normalized_target) DO UPDATE SET
	target_text = excluded.target_text,
	target_type = excluded.target_type,
	owner = excluded.owner,
	repo = excluded.repo,
	updated_at = excluded.updated_at,
	last_requested_at = excluded.last_requested_at`
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, stmt, target.Input, target.NormalizedTarget, target.Mode, target.Owner, nullable(target.Repo), now, now, now); err != nil {
		return 0, err
	}

	var id int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM targets WHERE normalized_target = ?`, target.NormalizedTarget).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) upsertRepoTx(ctx context.Context, tx *sql.Tx, repo domain.Repo) (int64, error) {
	const stmt = `
INSERT INTO repos (full_name, owner, name, html_url, private, archived, fork, last_synced_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(full_name) DO UPDATE SET
	owner = excluded.owner,
	name = excluded.name,
	html_url = excluded.html_url,
	private = excluded.private,
	archived = excluded.archived,
	fork = excluded.fork,
	last_synced_at = excluded.last_synced_at,
	updated_at = excluded.updated_at`
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, stmt, repo.FullName, repo.Owner, repo.Name, repo.HTMLURL, repo.Private, repo.Archived, repo.Fork, repo.FetchedAt.UTC().Format(time.RFC3339Nano), now, now); err != nil {
		return 0, err
	}
	var id int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM repos WHERE full_name = ?`, repo.FullName).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) upsertTargetRepoTx(ctx context.Context, tx *sql.Tx, targetID, repoID int64) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := tx.ExecContext(ctx, `
INSERT INTO target_repos (target_id, repo_id, discovered_at, last_seen_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(target_id, repo_id) DO UPDATE SET
	last_seen_at = excluded.last_seen_at`, targetID, repoID, now, now)
	return err
}

func (s *Store) replaceRepoContributorsTx(ctx context.Context, tx *sql.Tx, repoID int64, contributors []domain.Contributor) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM repo_contributors WHERE repo_id = ?`, repoID); err != nil {
		return err
	}

	for _, contributor := range contributors {
		contributorID, err := s.upsertContributorTx(ctx, tx, contributor)
		if err != nil {
			return err
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		_, err = tx.ExecContext(ctx, `
INSERT INTO repo_contributors (repo_id, contributor_id, contributions, fetched_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)`,
			repoID,
			contributorID,
			contributor.Contributions,
			now,
			now,
			now,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) upsertContributorTx(ctx context.Context, tx *sql.Tx, contributor domain.Contributor) (int64, error) {
	const stmt = `
INSERT INTO contributors (identity_key, github_user_id, login, avatar_url, html_url, type, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(identity_key) DO UPDATE SET
	github_user_id = excluded.github_user_id,
	login = excluded.login,
	avatar_url = excluded.avatar_url,
	html_url = excluded.html_url,
	type = excluded.type,
	updated_at = excluded.updated_at`
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := tx.ExecContext(ctx, stmt, contributor.IdentityKey, nullableInt64(contributor.GitHubUserID), nullable(contributor.Login), nullable(contributor.AvatarURL), nullable(contributor.HTMLURL), nullable(contributor.Type), now, now)
	if err != nil {
		return 0, err
	}
	var id int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM contributors WHERE identity_key = ?`, contributor.IdentityKey).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableInt64(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}

func (s *Store) DumpResponseJSON(resp domain.ContributionResponse) ([]byte, error) {
	return json.Marshal(resp)
}

var _ = fmt.Sprintf
