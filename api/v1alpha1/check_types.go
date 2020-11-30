package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// CheckSpec defines the desired state of Check. It needs to contain the repository
// information.
type CheckSpec struct {
	PromURL  string   `json:"promURL,omitempty"`
	Backend  *Backend `json:"backend,omitempty"`
	KeepJobs *int     `json:"keepJobs,omitempty"`
}

// CheckStatus defines the observed state of Check
type CheckStatus struct {
	K8upStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Check is the Schema for the checks API
type Check struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CheckSpec   `json:"spec,omitempty"`
	Status CheckStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CheckList contains a list of Check
type CheckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Check `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Check{}, &CheckList{})
}

func (c *Check) GetRuntimeObject() runtime.Object {
	return c
}

func (c *Check) GetMetaObject() metav1.Object {
	return c
}

func (c *Check) GetType() string {
	return "check"
}

func (c *Check) GetK8upStatus() *K8upStatus {
	return &c.Status.K8upStatus
}

func (c CheckSpec) CreateObject(name, namespace string) runtime.Object {
	return &Check{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: c,
	}
}
