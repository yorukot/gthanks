package port

import (
	"context"

	"gthanks/internal/domain"
)

type Store interface {
	GetQueryCache(ctx context.Context, cacheKey string) (*domain.CachedResponse, error)
	SaveQueryCache(ctx context.Context, record domain.QueryCacheRecord) error
	SaveRepoSnapshot(ctx context.Context, target domain.Target, repo domain.Repo) error
}
