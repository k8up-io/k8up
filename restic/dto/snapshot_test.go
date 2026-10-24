package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Shape of `restic snapshots --json` output from restic >= 0.17: the first
// snapshot was created by restic >= 0.17 and carries a summary, the second
// one predates it and doesn't.
const resticSnapshotsJSON = `[
  {
    "time": "2026-10-24T10:00:00.123456789Z",
    "tree": "aaaa",
    "paths": ["/data/data-pvc"],
    "hostname": "my-namespace",
    "username": "root",
    "id": "1111111111111111111111111111111111111111111111111111111111111111",
    "short_id": "11111111",
    "summary": {
      "backup_start": "2026-10-24T09:59:00Z",
      "backup_end": "2026-10-24T10:00:00Z",
      "files_new": 3,
      "files_changed": 1,
      "files_unmodified": 100,
      "dirs_new": 0,
      "dirs_changed": 1,
      "dirs_unmodified": 10,
      "data_blobs": 4,
      "tree_blobs": 2,
      "data_added": 2048,
      "data_added_packed": 1024,
      "total_files_processed": 104,
      "total_bytes_processed": 524288000
    }
  },
  {
    "time": "2023-01-01T00:00:00Z",
    "tree": "bbbb",
    "paths": ["/data/data-pvc"],
    "hostname": "my-namespace",
    "id": "2222222222222222222222222222222222222222222222222222222222222222",
    "short_id": "22222222"
  }
]`

func TestSnapshot_UnmarshalSummary(t *testing.T) {
	var snapshots []Snapshot
	require.NoError(t, json.Unmarshal([]byte(resticSnapshotsJSON), &snapshots))
	require.Len(t, snapshots, 2)

	withSummary := snapshots[0]
	require.NotNil(t, withSummary.Summary)
	assert.Equal(t, uint64(524288000), withSummary.Summary.TotalBytesProcessed)
	assert.Equal(t, uint64(2048), withSummary.Summary.DataAdded)
	assert.Equal(t, uint64(1024), withSummary.Summary.DataAddedPacked)
	assert.Equal(t, uint(104), withSummary.Summary.TotalFilesProcessed)

	assert.Nil(t, snapshots[1].Summary, "snapshots from restic < 0.17 have no summary")
}

func TestSnapshot_MarshalOmitsMissingSummary(t *testing.T) {
	out, err := json.Marshal(Snapshot{ID: "abc"})
	require.NoError(t, err)
	assert.NotContains(t, string(out), "summary")

	out, err = json.Marshal(Snapshot{ID: "abc", Summary: &SnapshotSummary{TotalBytesProcessed: 42}})
	require.NoError(t, err)
	assert.Contains(t, string(out), `"total_bytes_processed":42`)
}
