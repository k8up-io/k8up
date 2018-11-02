package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Archive struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              *ArchiveSpec  `json:"spec,omitempty"`
	Status            ArchiveStatus `json:"status,omitempty"`
}

// ArchiveSpec specifies how the archiv CRD looks like
// currently this is a simple wrapper for the RestoreSpec
// but this might get extended in the future.
type ArchiveSpec struct {
	*RestoreSpec `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ArchiveList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Archive `json:"items"`
}

type ArchiveStatus struct {
	JobStatus `json:",inline"`
}
