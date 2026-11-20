package kubernetes

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeRepositoryURL(t *testing.T) {
	tests := map[string]struct {
		input    string
		expected string
	}{
		"REST with credentials": {
			input:    "rest:https://backup:secret@backup.example.com/repo",
			expected: "rest:https://redacted:redacted@backup.example.com/repo",
		},
		"REST with user only": {
			input:    "rest:https://user@backup.example.com/repo",
			expected: "rest:https://redacted@backup.example.com/repo",
		},
		"S3 no credentials": {
			input:    "s3:http://minio:9000/bucket",
			expected: "s3:http://minio:9000/bucket",
		},
		"Azure no credentials": {
			input:    "azure:container:/path",
			expected: "azure:container:/path",
		},
		"REST with env var placeholders": {
			input:    "rest:https://$(USER):$(PASSWORD)@backup.example.com/repo",
			expected: "rest:https://redacted:redacted@backup.example.com/repo",
		},
		"empty string": {
			input:    "",
			expected: "",
		},
		"local path": {
			input:    "/tmp/restic-repo",
			expected: "/tmp/restic-repo",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := sanitizeRepositoryURL(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
