package shared

import "time"

// Defaults for sessions.Options
const (
	DefaultMaxAge = 86400 * 30 // 30days
	EmptyDataAge  = 3600       // 1hour
)

// Defaults for store.Options
const (
	DefaultBucketName = "sessions"
)

// Defaults for reaper.Options
const (
	DefaultBatchSize     = 100
	DefaultCheckInterval = time.Minute
)
