package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// RestoreSpec can either contain an S3 restore point or a local one. For the local
// one you need to define an existing PVC.
type RestoreSpec struct {
	RunnableSpec `json:",inline"`

	RestoreMethod *RestoreMethod `json:"restoreMethod,omitempty"`
	RestoreFilter string         `json:"restoreFilter,omitempty"`
	Snapshot      string         `json:"snapshot,omitempty"`
	KeepJobs      *int           `json:"keepJobs,omitempty"`
	// Tags is a list of arbitrary tags that get added to the backup via Restic's tagging system
	Tags []string `json:"tags,omitempty"`
}

func (r RestoreSpec) CreateObject(name, namespace string) runtime.Object {
	return &Restore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: r,
	}
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

func (r *Restore) GetRuntimeObject() runtime.Object {
	return r
}

func (r *Restore) GetMetaObject() metav1.Object {
	return r
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

// IsDeduplicationSupported returns true if this job supports deduplication
func (in *RestoreSchedule) IsDeduplicationSupported() bool {
	return false
}

// GetObjectCreator returns the ObjectCreator instance
func (in *RestoreSchedule) GetObjectCreator() ObjectCreator {
	return in
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
