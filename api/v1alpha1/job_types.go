package v1alpha1

// Type defines what job type this is.
type Type string

const (
	BackupType   Type = "backup"
	CheckType    Type = "check"
	ArchiveType  Type = "archive"
	RestoreType  Type = "restore"
	PruneType    Type = "prune"
	ScheduleType Type = "schedule"
)
