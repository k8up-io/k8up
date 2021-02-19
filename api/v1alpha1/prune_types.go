package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// PruneSpec needs to contain the repository information as well as the desired
// retention policies.
type PruneSpec struct {
	RunnableSpec `json:",inline"`

	// Retention sets how many backups should be kept after a forget and prune
	Retention RetentionPolicy `json:"retention,omitempty"`
	KeepJobs  *int            `json:"keepJobs,omitempty"`
}

func (p PruneSpec) CreateObject(name, namespace string) runtime.Object {
	return &Prune{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: p,
	}
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

func (p *Prune) GetRuntimeObject() runtime.Object {
	return p
}

func (p *Prune) GetMetaObject() metav1.Object {
	return p
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

// GetObjectCreator returns the ObjectCreator instance
func (in *PruneSchedule) GetObjectCreator() ObjectCreator {
	return in
}

// IsDeduplicationSupported returns true if this job supports deduplication
func (in *PruneSchedule) IsDeduplicationSupported() bool {
	return false
}

func init() {
	SchemeBuilder.Register(&Prune{}, &PruneList{})
}
