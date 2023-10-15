package store

import (
	"github.com/admpub/boltstore/shared"
)

// Config represents a config for a session store.
type Config struct {
	// DBOptions represents options for a database.
	DBOptions Options
	MaxLength int
}

// setDefault sets default to the config.
func (c *Config) setDefault() {
	if c.DBOptions.BucketName == nil {
		c.DBOptions.BucketName = []byte(shared.DefaultBucketName)
	}
}
