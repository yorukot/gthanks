package domain

import "time"

type QueryCacheRecord struct {
	CacheKey     string
	Target       Target
	ResponseJSON []byte
	CacheStatus  string
	ExpiresAt    time.Time
}

type ImageCacheRecord struct {
	CacheKey    string
	Target      Target
	ImagePNG    []byte
	CacheStatus string
	ExpiresAt   time.Time
}

type CachedBinary struct {
	Content   []byte
	ExpiresAt time.Time
}

type AvatarCacheRecord struct {
	AvatarURL string
	Content   []byte
	ExpiresAt time.Time
}
