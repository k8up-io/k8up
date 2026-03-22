package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestS3SpecRestoreEnvVarsWithNilReceiver(t *testing.T) {
	// When a user specifies "s3:" with a nil value in YAML,
	// S3 is nil but RestoreMethod is not.
	// Calling RestoreEnvVars() on nil S3Spec must not panic.
	var s3 *S3Spec
	assert.NotPanics(t, func() {
		result := s3.RestoreEnvVars()
		assert.Nil(t, result)
	})
}
