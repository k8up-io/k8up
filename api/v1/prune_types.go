package v1

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PruneSpec needs to contain the repository information as well as the desired
// retention policies.
type PruneSpec struct {
	RunnableSpec `json:",inline"`

	// Retention sets how many backups should be kept after a forget and prune
	Retention RetentionPolicy `json:"retention,omitempty"`
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
}

type RetentionPolicy struct {
	KeepLast    int      `json:"keepLast,omitempty"`
	KeepHourly  int      `json:"keepHourly,omitempty"`
	KeepDaily   int      `json:"keepDaily,omitempty"`
	KeepWeekly  int      `json:"keepWeekly,omitempty"`
	KeepMonthly int      `json:"keepMonthly,omitempty"`
	KeepYearly  int      `json:"keepYearly,omitempty"`
	KeepTags    []string `json:"keepTags,omitempty"`
	// Tags is a filter on what tags the policy should be applied
	// DO NOT CONFUSE THIS WITH KeepTags OR YOU'LL have a bad time
	Tags []string `json:"tags,omitempty"`
	// Hostnames is a filter on what hostnames the policy should be applied
	Hostnames []string `json:"hostnames,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Schedule Ref",type="string",JSONPath=`.metadata.ownerReferences[?(@.kind == "Schedule")].name`,description="Reference to Schedule"
// +kubebuilder:printcolumn:name="Completion",type="string",JSONPath=`.status.conditions[?(@.type == "Completed")].reason`,description="Status of Completion"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Prune is the Schema for the prunes API
type Prune struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PruneSpec `json:"spec,omitempty"`
	Status Status    `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PruneList contains a list of Prune
type PruneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Prune `json:"items"`
}

func (p *Prune) GetType() JobType {
	return PruneType
}

// GetStatus retrieves the Status property
func (p *Prune) GetStatus() Status {
	return p.Status
}

// SetStatus sets the Status property
func (p *Prune) SetStatus(status Status) {
	p.Status = status
}

// GetResources returns the resource requirements
func (p *Prune) GetResources() corev1.ResourceRequirements {
	return p.Spec.Resources
}

// GetPodSecurityContext returns the pod security context
func (p *Prune) GetPodSecurityContext() *corev1.PodSecurityContext {
	return p.Spec.PodSecurityContext
}

// GetActiveDeadlineSeconds implements JobObject
func (p *Prune) GetActiveDeadlineSeconds() *int64 {
	return p.Spec.ActiveDeadlineSeconds
}

// GetFailedJobsHistoryLimit returns failed jobs history limit.
// Returns KeepJobs if unspecified.
func (p *Prune) GetFailedJobsHistoryLimit() *int {
	if p.Spec.FailedJobsHistoryLimit != nil {
		return p.Spec.FailedJobsHistoryLimit
	}
	return p.Spec.KeepJobs
}

// GetSuccessfulJobsHistoryLimit returns successful jobs history limit.
// Returns KeepJobs if unspecified.
func (p *Prune) GetSuccessfulJobsHistoryLimit() *int {
	if p.Spec.SuccessfulJobsHistoryLimit != nil {
		return p.Spec.SuccessfulJobsHistoryLimit
	}
	return p.Spec.KeepJobs
}

// GetJobObjects returns a sortable list of jobs
func (p *PruneList) GetJobObjects() JobObjectList {
	items := make(JobObjectList, len(p.Items))
	for i := range p.Items {
		items[i] = &p.Items[i]
	}
	return items
}

// GetDeepCopy returns a deep copy
func (in *PruneSchedule) GetDeepCopy() ScheduleSpecInterface {
	return in.DeepCopy()
}

// GetRunnableSpec returns a pointer to RunnableSpec
func (in *PruneSchedule) GetRunnableSpec() *RunnableSpec {
	return &in.RunnableSpec
}

// GetSchedule returns the schedule definition
func (in *PruneSchedule) GetSchedule() ScheduleDefinition {
	return in.Schedule
}

func init() {
	SchemeBuilder.Register(&Prune{}, &PruneList{})
}

var (
	PruneKind = reflect.TypeOf(Prune{}).Name()
)
