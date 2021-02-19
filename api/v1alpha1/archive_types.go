package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

func (a ArchiveSpec) CreateObject(name, namespace string) runtime.Object {
	return &Archive{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: a,
	}
}

func (a *Archive) GetRuntimeObject() runtime.Object {
	return a
}

func (a *Archive) GetMetaObject() metav1.Object {
	return a
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

// GetObjectCreator returns the ObjectCreator instance
func (in *ArchiveSchedule) GetObjectCreator() ObjectCreator {
	return in
}

// IsDeduplicationSupported returns true if this job supports deduplication
func (in *ArchiveSchedule) IsDeduplicationSupported() bool {
	return false
}
