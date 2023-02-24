package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
