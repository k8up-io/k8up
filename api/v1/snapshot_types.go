package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:rbac:groups=k8up.io,resources=snapshots,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8up.io,resources=snapshots/status;snapshots/finalizers,verbs=get;update;patch

// SnapshotSpec contains all information needed about a restic snapshot so it
// can be restored.
type SnapshotSpec struct {
	ID         *string      `json:"id,omitempty"`
	Date       *metav1.Time `json:"date,omitempty"`
	Paths      *[]string    `json:"paths,omitempty"`
	Repository *string      `json:"repository,omitempty"`
}

// SnapshotStatus defines the observed state of Snapshot
type SnapshotStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Date taken",type="string",JSONPath=`.spec.date`,description="Date when snapshot was taken"
// +kubebuilder:printcolumn:name="Paths",type="string",JSONPath=`.spec.paths[*]`,description="Snapshot's paths"
// +kubebuilder:printcolumn:name="Repository",type="string",JSONPath=`.spec.repository`,description="Repository Url"

// Snapshot is the Schema for the snapshots API
type Snapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SnapshotSpec   `json:"spec,omitempty"`
	Status SnapshotStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SnapshotList contains a list of Snapshot
type SnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Snapshot `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Snapshot{}, &SnapshotList{})
}
