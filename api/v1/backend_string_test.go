package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsNil(t *testing.T) {
	assert.True(t, IsNil(nil))
	assert.True(t, IsNil((*S3Spec)(nil)))
	assert.True(t, IsNil((*BackupSchedule)(nil)))
	assert.False(t, IsNil(&S3Spec{}))
	assert.False(t, IsNil("not nil"))
	assert.False(t, IsNil(42))
}

func TestRestServerSpec_String(t *testing.T) {
	tests := map[string]struct {
		url      string
		expected string
	}{
		"https URL": {
			url:      "https://backup.example.com/repo",
			expected: "rest:https://$(USER):$(PASSWORD)@backup.example.com/repo",
		},
		"http URL": {
			url:      "http://localhost:8000/repo",
			expected: "rest:http://$(USER):$(PASSWORD)@localhost:8000/repo",
		},
		"no protocol": {
			url:      "backup.example.com",
			expected: "rest:backup.example.com://$(USER):$(PASSWORD)@",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			spec := &RestServerSpec{URL: tc.url}
			assert.Equal(t, tc.expected, spec.String())
		})
	}
}

func TestAzureSpec_String(t *testing.T) {
	t.Run("with path", func(t *testing.T) {
		spec := &AzureSpec{Container: "mycontainer", Path: "/backups"}
		assert.Equal(t, "azure:mycontainer:/backups", spec.String())
	})
	t.Run("without path defaults to /", func(t *testing.T) {
		spec := &AzureSpec{Container: "mycontainer"}
		assert.Equal(t, "azure:mycontainer:/", spec.String())
	})
}

func TestSwiftSpec_String(t *testing.T) {
	spec := &SwiftSpec{Container: "mycontainer", Path: "backups"}
	assert.Equal(t, "swift:mycontainer:backups", spec.String())
}

func TestB2Spec_String(t *testing.T) {
	spec := &B2Spec{Bucket: "mybucket", Path: "backups"}
	assert.Equal(t, "b2:mybucket:backups", spec.String())
}
