package cfg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidate_NoOperations(t *testing.T) {
	c := &Configuration{}
	assert.NoError(t, c.Validate())
}

func TestValidatePrune_NotEnabled(t *testing.T) {
	c := &Configuration{DoPrune: false}
	assert.NoError(t, c.Validate())
}

func TestValidatePrune_ValidKeepN(t *testing.T) {
	c := &Configuration{
		DoPrune:        true,
		PruneKeepLast:  5,
		PruneKeepDaily: 14,
	}
	assert.NoError(t, c.Validate())
}

func TestValidatePrune_NegativeKeepN(t *testing.T) {
	c := &Configuration{
		DoPrune:       true,
		PruneKeepLast: -1,
	}
	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "keepLast")
	assert.Contains(t, err.Error(), "must not be negative")
}

func TestValidatePrune_ValidKeepWithin(t *testing.T) {
	c := &Configuration{
		DoPrune:         true,
		PruneKeepWithin: "24h",
	}
	assert.NoError(t, c.Validate())
}

func TestValidatePrune_InvalidKeepWithinDuration(t *testing.T) {
	c := &Configuration{
		DoPrune:         true,
		PruneKeepWithin: "not-a-duration",
	}
	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not valid")
}

func TestValidatePrune_NegativeKeepWithinDuration(t *testing.T) {
	c := &Configuration{
		DoPrune:         true,
		PruneKeepWithin: "-1h",
	}
	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must not be negative")
}

func TestValidatePrune_EmptyKeepWithinSkipped(t *testing.T) {
	c := &Configuration{
		DoPrune:         true,
		PruneKeepWithin: "",
	}
	assert.NoError(t, c.Validate())
}

func TestValidateRestore_NotEnabled(t *testing.T) {
	c := &Configuration{DoRestore: false}
	assert.NoError(t, c.Validate())
}

func TestValidateRestore_S3Valid(t *testing.T) {
	c := &Configuration{
		DoRestore:          true,
		RestoreType:        "s3",
		RestoreS3Endpoint:  "http://minio:9000",
		RestoreS3AccessKey: "access",
		RestoreS3SecretKey: "secret",
	}
	assert.NoError(t, c.Validate())
}

func TestValidateRestore_S3MissingEndpoint(t *testing.T) {
	c := &Configuration{
		DoRestore:          true,
		RestoreType:        "s3",
		RestoreS3AccessKey: "access",
		RestoreS3SecretKey: "secret",
	}
	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint")
}

func TestValidateRestore_S3MissingAccessKey(t *testing.T) {
	c := &Configuration{
		DoRestore:          true,
		RestoreType:        "s3",
		RestoreS3Endpoint:  "http://minio:9000",
		RestoreS3SecretKey: "secret",
	}
	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access key")
}

func TestValidateRestore_S3MissingSecretKey(t *testing.T) {
	c := &Configuration{
		DoRestore:          true,
		RestoreType:        "s3",
		RestoreS3Endpoint:  "http://minio:9000",
		RestoreS3AccessKey: "access",
	}
	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "secret key")
}

func TestValidateRestore_FolderValid(t *testing.T) {
	c := &Configuration{
		DoRestore:   true,
		RestoreType: "folder",
		RestoreDir:  "/restore",
	}
	assert.NoError(t, c.Validate())
}

func TestValidateRestore_FolderMissingDir(t *testing.T) {
	c := &Configuration{
		DoRestore:   true,
		RestoreType: "folder",
	}
	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "directory")
}

func TestValidateRestore_UnknownType(t *testing.T) {
	c := &Configuration{
		DoRestore:   true,
		RestoreType: "unknown",
	}
	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

func TestValidateRestore_TypeCaseInsensitive(t *testing.T) {
	c := &Configuration{
		DoRestore:          true,
		RestoreType:        "S3",
		RestoreS3Endpoint:  "http://minio:9000",
		RestoreS3AccessKey: "access",
		RestoreS3SecretKey: "secret",
	}
	assert.NoError(t, c.Validate())
}
