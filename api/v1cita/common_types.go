package v1cita

import k8upv1 "github.com/k8up-io/k8up/v2/api/v1"

// The job types that k8up deals with
const (
	CITABackupType  k8upv1.JobType = "cita-backup"
	CITARestoreType k8upv1.JobType = "cita-restore"
	FallbackType    k8upv1.JobType = "fallback"
	SwitchoverType  k8upv1.JobType = "switchover"
)

type K8upCommon struct {
	// KeepJobs amount of jobs to keep for later analysis.
	//
	// Deprecated: Use FailedJobsHistoryLimit and SuccessfulJobsHistoryLimit respectively.
	// +optional
	KeepJobs *int `json:"keepJobs,omitempty"`
	// FailedJobsHistoryLimit amount of failed jobs to keep for later analysis.
	// KeepJobs is used property is not specified.
	// +optional
	FailedJobsHistoryLimit *int `json:"failedJobsHistoryLimit,omitempty"`
	// SuccessfulJobsHistoryLimit amount of successful jobs to keep for later analysis.
	// KeepJobs is used property is not specified.
	// +optional
	SuccessfulJobsHistoryLimit *int `json:"successfulJobsHistoryLimit,omitempty"`

	// PromURL sets a prometheus push URL where the backup container send metrics to
	// +optional
	PromURL string `json:"promURL,omitempty"`

	// StatsURL sets an arbitrary URL where the restic container posts metrics and
	// information about the snapshots to. This is in addition to the prometheus
	// pushgateway.
	StatsURL string `json:"statsURL,omitempty"`

	// Tags is a list of arbitrary tags that get added to the backup via Restic's tagging system
	Tags []string `json:"tags,omitempty"`
}
