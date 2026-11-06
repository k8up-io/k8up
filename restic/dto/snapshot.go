package dto

import "time"

// Snapshot models a restic a single snapshot from the
// snapshots --json subcommand.
type Snapshot struct {
	ID       string    `json:"id"`
	Time     time.Time `json:"time"`
	Tree     string    `json:"tree"`
	Paths    []string  `json:"paths"`
	Hostname string    `json:"hostname"`
	Username string    `json:"username"`
	UID      int       `json:"uid"`
	Gid      int       `json:"gid"`
	Tags     []string  `json:"tags"`
	// Summary is only present for snapshots created by restic >= 0.17.
	Summary *SnapshotSummary `json:"summary,omitempty"`
}

// SnapshotSummary holds the backup statistics restic stores in each
// snapshot (restic >= 0.17).
type SnapshotSummary struct {
	BackupStart         time.Time `json:"backup_start"`
	BackupEnd           time.Time `json:"backup_end"`
	FilesNew            uint      `json:"files_new"`
	FilesChanged        uint      `json:"files_changed"`
	FilesUnmodified     uint      `json:"files_unmodified"`
	DirsNew             uint      `json:"dirs_new"`
	DirsChanged         uint      `json:"dirs_changed"`
	DirsUnmodified      uint      `json:"dirs_unmodified"`
	DataBlobs           int       `json:"data_blobs"`
	TreeBlobs           int       `json:"tree_blobs"`
	DataAdded           uint64    `json:"data_added"`
	DataAddedPacked     uint64    `json:"data_added_packed"`
	TotalFilesProcessed uint      `json:"total_files_processed"`
	TotalBytesProcessed uint64    `json:"total_bytes_processed"`
}
