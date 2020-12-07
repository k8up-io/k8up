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

// ArchiveStatus defines the observed state of Archive.
type ArchiveStatus struct {
	K8upStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Archive is the Schema for the archives API
type Archive struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ArchiveSpec   `json:"spec,omitempty"`
	Status ArchiveStatus `json:"status,omitempty"`
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

func (a *Archive) GetK8upStatus() *K8upStatus {
	return &a.Status.K8upStatus
}

func (a *Archive) GetResources() corev1.ResourceRequirements {
	return a.Spec.Resources
}
