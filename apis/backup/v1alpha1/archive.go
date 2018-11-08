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

func (a ArchiveList) Len() int      { return len(a.Items) }
func (a ArchiveList) Swap(i, j int) { a.Items[i], a.Items[j] = a.Items[j], a.Items[i] }

func (a ArchiveList) Less(i, j int) bool {

	if a.Items[i].CreationTimestamp.Equal(&a.Items[j].CreationTimestamp) {
		return a.Items[i].Name < a.Items[j].Name
	}

	return a.Items[i].CreationTimestamp.Before(&a.Items[j].CreationTimestamp)
}
