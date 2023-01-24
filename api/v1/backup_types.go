package v1

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BackupSpec defines a single backup. It must contain all information to connect to
// the backup repository when applied. If used with defaults or schedules the operator will
// ensure that the defaults are applied before creating the object on the API.
type BackupSpec struct {
	RunnableSpec `json:",inline"`

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

type BackupTemplate struct {
	Tags    *[]string `json:"tags,omitempty"`
	Backend Backend   `json:"backend,omitempty"`
	Env     Env       `json:"env,omitempty"`
}

type Env struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Schedule Ref",type="string",JSONPath=`.metadata.ownerReferences[?(@.kind == "Schedule")].name`,description="Reference to Schedule"
// +kubebuilder:printcolumn:name="Completion",type="string",JSONPath=`.status.conditions[?(@.type == "Completed")].reason`,description="Status of Completion"
// +kubebuilder:printcolumn:name="PreBackup",type="string",JSONPath=`.status.conditions[?(@.type == "PreBackupPodReady")].reason`,description="Status of PreBackupPods"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Backup is the Schema for the backups API
type Backup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupSpec `json:"spec,omitempty"`
	Status Status     `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BackupList contains a list of Backup
type BackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Backup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Backup{}, &BackupList{})
}

func (*Backup) GetType() JobType {
	return BackupType
}

// GetStatus retrieves the Status property
func (b *Backup) GetStatus() Status {
	return b.Status
}

// SetStatus sets the Status property
func (b *Backup) SetStatus(status Status) {
	b.Status = status
}

// GetResources returns the resource requirements
func (b *Backup) GetResources() corev1.ResourceRequirements {
	return b.Spec.Resources
}

// GetPodSecurityContext returns the pod security context
func (b *Backup) GetPodSecurityContext() *corev1.PodSecurityContext {
	return b.Spec.PodSecurityContext
}

// GetActiveDeadlineSeconds implements JobObject
func (b *Backup) GetActiveDeadlineSeconds() *int64 {
	return b.Spec.ActiveDeadlineSeconds
}

// GetFailedJobsHistoryLimit returns failed jobs history limit.
// Returns KeepJobs if unspecified.
func (b *Backup) GetFailedJobsHistoryLimit() *int {
	if b.Spec.FailedJobsHistoryLimit != nil {
		return b.Spec.FailedJobsHistoryLimit
	}
	return b.Spec.KeepJobs
}

// GetSuccessfulJobsHistoryLimit returns successful jobs history limit.
// Returns KeepJobs if unspecified.
func (b *Backup) GetSuccessfulJobsHistoryLimit() *int {
	if b.Spec.SuccessfulJobsHistoryLimit != nil {
		return b.Spec.SuccessfulJobsHistoryLimit
	}
	return b.Spec.KeepJobs
}

// GetJobObjects returns a sortable list of jobs
func (b *BackupList) GetJobObjects() JobObjectList {
	items := make(JobObjectList, len(b.Items))
	for i := range b.Items {
		items[i] = &b.Items[i]
	}
	return items
}

// GetDeepCopy returns a deep copy
func (in *BackupSchedule) GetDeepCopy() ScheduleSpecInterface {
	return in.DeepCopy()
}

// GetRunnableSpec returns a pointer to RunnableSpec
func (in *BackupSchedule) GetRunnableSpec() *RunnableSpec {
	return &in.RunnableSpec
}

// GetSchedule returns the schedule definition
func (in *BackupSchedule) GetSchedule() ScheduleDefinition {
	return in.Schedule
}

var (
	BackupKind = reflect.TypeOf(Backup{}).Name()
)
