package store

import (
	"testing"

	"github.com/admpub/boltstore/shared"
)

func TestConfig_setDefault(t *testing.T) {
	config := Config{}
	config.setDefault()
	if string(config.DBOptions.BucketName) != shared.DefaultBucketName {
		t.Errorf("config.SessionOptions.BucketName should be %+v (actual: %+v)", shared.DefaultBucketName, string(config.DBOptions.BucketName))
	}
}
