package domain

import "time"

type QueryCacheRecord struct {
	CacheKey     string
	Target       Target
	ResponseJSON []byte
	CacheStatus  string
	ExpiresAt    time.Time
}
