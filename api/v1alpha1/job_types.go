package v1alpha1

// JobType defines what job type this is.
type JobType string

const (
	BackupType   JobType = "backup"
	CheckType    JobType = "check"
	ArchiveType  JobType = "archive"
	RestoreType  JobType = "restore"
	PruneType    JobType = "prune"
	ScheduleType JobType = "schedule"
)
