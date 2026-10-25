package kubernetes

import (
	"testing"
	"time"

	"github.com/k8up-io/k8up/v2/restic/dto"
)

func FuzzFilterAndConvert(f *testing.F) {
	// Seed with realistic inputs
	f.Add("abc12345678", "default", "s3:http://minio:9000/backup")
	f.Add("short", "default", "s3:http://minio:9000/backup")
	f.Add("", "default", "s3:http://minio:9000/backup")
	f.Add("abcdefgh", "test-ns", "rest:https://user:pass@host/path")
	f.Add("1234567", "default", "") // 7 chars - just under the [:8] slice

	f.Fuzz(func(t *testing.T, id, namespace, repository string) {
		snapshots := []dto.Snapshot{
			{
				ID:       id,
				Time:     time.Now(),
				Hostname: namespace, // must match namespace to pass filter
				Paths:    []string{"/data"},
			},
		}

		// This must not panic
		result := filterAndConvert(snapshots, namespace, repository)

		// If snapshot passed the filter, verify basic properties
		if len(result.Items) > 0 {
			if result.Items[0].Namespace != namespace {
				t.Errorf("expected namespace %q, got %q", namespace, result.Items[0].Namespace)
			}
		}
	})
}
