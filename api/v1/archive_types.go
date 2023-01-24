package v1

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ArchiveSpec defines the desired state of Archive.
type ArchiveSpec struct {
	*RestoreSpec `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Schedule Ref",type="string",JSONPath=`.metadata.ownerReferences[?(@.kind == "Schedule")].name`,description="Reference to Schedule"
// +kubebuilder:printcolumn:name="Completion",type="string",JSONPath=`.status.conditions[?(@.type == "Completed")].reason`,description="Status of Completion"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Archive is the Schema for the archives API
type Archive struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ArchiveSpec `json:"spec,omitempty"`
	Status Status      `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ArchiveList contains a list of Archive
type ArchiveList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Archive `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Archive{}, &ArchiveList{})
}

func (*Archive) GetType() JobType {
	return ArchiveType
}

// GetStatus retrieves the Status property
func (a *Archive) GetStatus() Status {
	return a.Status
}

// SetStatus sets the Status property
func (a *Archive) SetStatus(status Status) {
	a.Status = status
}

// GetResources returns the resource requirements
func (a *Archive) GetResources() corev1.ResourceRequirements {
	return a.Spec.Resources
}

// GetPodSecurityContext returns the pod security context
func (a *Archive) GetPodSecurityContext() *corev1.PodSecurityContext {
	return a.Spec.PodSecurityContext
}

// GetActiveDeadlineSeconds implements JobObject
func (a *Archive) GetActiveDeadlineSeconds() *int64 {
	return a.Spec.ActiveDeadlineSeconds
}

// GetFailedJobsHistoryLimit returns failed jobs history limit.
// Returns KeepJobs if unspecified.
func (a *Archive) GetFailedJobsHistoryLimit() *int {
	if a.Spec.FailedJobsHistoryLimit != nil {
		return a.Spec.FailedJobsHistoryLimit
	}
	return a.Spec.KeepJobs
}

// GetSuccessfulJobsHistoryLimit returns successful jobs history limit.
// Returns KeepJobs if unspecified.
func (a *Archive) GetSuccessfulJobsHistoryLimit() *int {
	if a.Spec.SuccessfulJobsHistoryLimit != nil {
		return a.Spec.SuccessfulJobsHistoryLimit
	}
	return a.Spec.KeepJobs
}

// GetJobObjects returns a sortable list of jobs
func (a *ArchiveList) GetJobObjects() JobObjectList {
	items := make(JobObjectList, len(a.Items))
	for i := range a.Items {
		items[i] = &a.Items[i]
	}
	return items
}

// GetDeepCopy returns a deep copy
func (in *ArchiveSchedule) GetDeepCopy() ScheduleSpecInterface {
	return in.DeepCopy()
}

// GetRunnableSpec returns a pointer to RunnableSpec
func (in *ArchiveSchedule) GetRunnableSpec() *RunnableSpec {
	return &in.RunnableSpec
}

// GetSchedule returns the schedule definition
func (in *ArchiveSchedule) GetSchedule() ScheduleDefinition {
	return in.Schedule
}

var (
	ArchiveKind = reflect.TypeOf(Archive{}).Name()
)
