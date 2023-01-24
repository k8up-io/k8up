package v1

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RestoreSpec can either contain an S3 restore point or a local one. For the local
// one you need to define an existing PVC.
type RestoreSpec struct {
	RunnableSpec `json:",inline"`

	RestoreMethod *RestoreMethod `json:"restoreMethod,omitempty"`
	RestoreFilter string         `json:"restoreFilter,omitempty"`
	Snapshot      string         `json:"snapshot,omitempty"`
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
	// Tags is a list of arbitrary tags that get added to the backup via Restic's tagging system
	Tags []string `json:"tags,omitempty"`
}

// RestoreMethod contains how and where the restore should happen
// all the settings are mutual exclusive.
type RestoreMethod struct {
	S3     *S3Spec        `json:"s3,omitempty"`
	Folder *FolderRestore `json:"folder,omitempty"`
}

type FolderRestore struct {
	*corev1.PersistentVolumeClaimVolumeSource `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Schedule Ref",type="string",JSONPath=`.metadata.ownerReferences[?(@.kind == "Schedule")].name`,description="Reference to Schedule"
// +kubebuilder:printcolumn:name="Completion",type="string",JSONPath=`.status.conditions[?(@.type == "Completed")].reason`,description="Status of Completion"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Restore is the Schema for the restores API
type Restore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RestoreSpec `json:"spec,omitempty"`
	Status Status      `json:"status,omitempty"`
}

func (r *Restore) GetType() JobType {
	return RestoreType
}

// GetStatus retrieves the Status property
func (r *Restore) GetStatus() Status {
	return r.Status
}

// SetStatus sets the Status property
func (r *Restore) SetStatus(status Status) {
	r.Status = status
}

// GetResources returns the resource requirements
func (r *Restore) GetResources() corev1.ResourceRequirements {
	return r.Spec.Resources
}

// GetPodSecurityContext returns the pod security context
func (r *Restore) GetPodSecurityContext() *corev1.PodSecurityContext {
	return r.Spec.PodSecurityContext
}

// GetActiveDeadlineSeconds implements JobObject
func (r *Restore) GetActiveDeadlineSeconds() *int64 {
	return r.Spec.ActiveDeadlineSeconds
}

// GetFailedJobsHistoryLimit returns failed jobs history limit.
// Returns KeepJobs if unspecified.
func (r *Restore) GetFailedJobsHistoryLimit() *int {
	if r.Spec.FailedJobsHistoryLimit != nil {
		return r.Spec.FailedJobsHistoryLimit
	}
	return r.Spec.KeepJobs
}

// GetSuccessfulJobsHistoryLimit returns successful jobs history limit.
// Returns KeepJobs if unspecified.
func (r *Restore) GetSuccessfulJobsHistoryLimit() *int {
	if r.Spec.SuccessfulJobsHistoryLimit != nil {
		return r.Spec.SuccessfulJobsHistoryLimit
	}
	return r.Spec.KeepJobs
}

// GetJobObjects returns a sortable list of jobs
func (r *RestoreList) GetJobObjects() JobObjectList {
	items := make(JobObjectList, len(r.Items))
	for i := range r.Items {
		items[i] = &r.Items[i]
	}
	return items
}

// GetDeepCopy returns a deep copy
func (in *RestoreSchedule) GetDeepCopy() ScheduleSpecInterface {
	return in.DeepCopy()
}

// GetRunnableSpec returns a pointer to RunnableSpec
func (in *RestoreSchedule) GetRunnableSpec() *RunnableSpec {
	return &in.RunnableSpec
}

// GetSchedule returns the schedule definition
func (in *RestoreSchedule) GetSchedule() ScheduleDefinition {
	return in.Schedule
}

// +kubebuilder:object:root=true

// RestoreList contains a list of Restore
type RestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Restore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Restore{}, &RestoreList{})
}

var (
	RestoreKind = reflect.TypeOf(Restore{}).Name()
)
