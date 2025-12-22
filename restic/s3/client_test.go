package s3

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseBucketAndPrefix(t *testing.T) {
	tests := map[string]struct {
		path           string
		expectedBucket string
		expectedPrefix string
	}{
		"empty path": {
			path:           "",
			expectedBucket: "",
			expectedPrefix: "",
		},
		"just slash": {
			path:           "/",
			expectedBucket: "",
			expectedPrefix: "",
		},
		"bucket only": {
			path:           "/mybucket",
			expectedBucket: "mybucket",
			expectedPrefix: "",
		},
		"bucket only without leading slash": {
			path:           "mybucket",
			expectedBucket: "mybucket",
			expectedPrefix: "",
		},
		"bucket with single path segment": {
			path:           "/mybucket/mypath",
			expectedBucket: "mybucket",
			expectedPrefix: "mypath",
		},
		"bucket with multi-segment path": {
			path:           "/mybucket/path/to/data",
			expectedBucket: "mybucket",
			expectedPrefix: "path/to/data",
		},
		"bucket with trailing slash": {
			path:           "/mybucket/",
			expectedBucket: "mybucket",
			expectedPrefix: "",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			bucket, prefix := parseBucketAndPrefix(tt.path)
			assert.Equal(t, tt.expectedBucket, bucket)
			assert.Equal(t, tt.expectedPrefix, prefix)
		})
	}
}

func Test_Client_objectPath(t *testing.T) {
	tests := map[string]struct {
		prefix       string
		name         string
		expectedPath string
	}{
		"no prefix": {
			prefix:       "",
			name:         "file.txt",
			expectedPath: "file.txt",
		},
		"with prefix": {
			prefix:       "my/path",
			name:         "file.txt",
			expectedPath: "my/path/file.txt",
		},
		"with prefix and nested name": {
			prefix:       "backups",
			name:         "2024/01/data.tar",
			expectedPath: "backups/2024/01/data.tar",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := &Client{prefix: tt.prefix}
			result := c.objectPath(tt.name)
			assert.Equal(t, tt.expectedPath, result)
		})
	}
}
