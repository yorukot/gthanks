package port

import (
	"context"

	"gthanks/internal/domain"
)

type Store interface {
	GetQueryCache(ctx context.Context, cacheKey string) (*domain.CachedResponse, error)
	SaveQueryCache(ctx context.Context, record domain.QueryCacheRecord) error
	GetImageCache(ctx context.Context, cacheKey string) (*domain.CachedBinary, error)
	SaveImageCache(ctx context.Context, record domain.ImageCacheRecord) error
	GetAvatarCache(ctx context.Context, avatarURL string) (*domain.CachedBinary, error)
	SaveAvatarCache(ctx context.Context, record domain.AvatarCacheRecord) error
	SaveRepoSnapshot(ctx context.Context, target domain.Target, repo domain.Repo) error
}
