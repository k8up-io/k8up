package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Backup represents a baas worker.
type Backup struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behaviour of the pod terminator.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status
	// +optional
	Spec BackupSpec `json:"spec,omitempty"`

	// Status of the backups
	// +optional
	Status BackupStatus `json:"status,omitempty"`
}

// BackupSpec is the spec for a BassWorker resource.
type BackupSpec struct {
	// DryRun will set the backup to dryrun mode or not.
	// +optional
	DryRun bool `json:"dryRun,omitempty"`
	// Schedule defines when the backup job should run
	Schedule string `json:"schedule,omitempty"`
	// CheckSchedule defines when the check jobs should run default once a week
	CheckSchedule string `json:"checkSchedule,omitempty"`
	// Backend contains the restic repo where the job should backup to.
	Backend Backend `json:"backend,omitempty"`
	// Paused indicates if the backup is currently paused or not.
	// +optional
	Paused bool `json:"paused,omitempty"`
	// KeepJobs amount of jobs to keep for later analysis
	KeepJobs int32 `json:"keepJobs,omitempty"`
	// Retention sets how many backups should be kept after a forget and prune
	Retention RetentionPolicy `json:"retention,omitempty"`
}

type BackupStatus struct {
	// LastBackup date
	// +optional
	LastBackupDate *metav1.Time `json:"lastBackupDate,omitempty"`
	// LastBackupDuration in seconds
	// +optional
	LastBackupDuration float64 `json:"lastBackupDuration,omitempty"`
	// LastBackupStatus
	// +optional
	LastBackupStatus string `json:"lastBackupDtatus,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupList is a list of BassWorker resources
type BackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Backup `json:"items"`
}

type Backend struct {
	// Password contains the repository password. ONLY for development
	Password string          `json:"password,omitempty"`
	Local    *LocalSpec      `json:"local,omitempty"`
	S3       *S3Spec         `json:"s3,omitempty"`
	GCS      *GCSSpec        `json:"gcs,omitempty"`
	Azure    *AzureSpec      `json:"azure,omitempty"`
	Swift    *SwiftSpec      `json:"swift,omitempty"`
	B2       *B2Spec         `json:"b2,omitempty"`
	Rest     *RestServerSpec `json:"rest,omitempty"`
}

type LocalSpec struct {
	corev1.VolumeSource `json:",inline"`
	MountPath           string `json:"mountPath,omitempty"`
	SubPath             string `json:"subPath,omitempty"`
}

type S3Spec struct {
	Endpoint string `json:"endpoint,omitempty"`
	Bucket   string `json:"bucket,omitempty"`
	Prefix   string `json:"prefix,omitempty"`
	Username string `json:"username,omitempty"` //ONLY for development
	Password string `json:"password,omitempty"` //ONLY for development
}

type GCSSpec struct {
	Bucket string `json:"bucket,omitempty"`
	Prefix string `json:"prefix,omitempty"`
}

type AzureSpec struct {
	Container string `json:"container,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
}

type SwiftSpec struct {
	Container string `json:"container,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
}

type B2Spec struct {
	Bucket string `json:"bucket,omitempty"`
	Prefix string `json:"prefix,omitempty"`
}

type RestServerSpec struct {
	URL string `json:"url,omitempty"`
}

type RetentionPolicy struct {
	KeepLast    int      `json:"keepLast,omitempty"`
	KeepHourly  int      `json:"keepHourly,omitempty"`
	KeepDaily   int      `json:"keepDaily,omitempty"`
	KeepWeekly  int      `json:"keepWeekly,omitempty"`
	KeepMonthly int      `json:"keepMonthly,omitempty"`
	KeepYearly  int      `json:"keepYearly,omitempty"`
	KeepTags    []string `json:"keepTags,omitempty"`
}
