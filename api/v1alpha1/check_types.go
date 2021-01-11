package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// CheckSpec defines the desired state of Check. It needs to contain the repository
// information.
type CheckSpec struct {
	RunnableSpec `json:",inline"`

	// PromURL sets a prometheus push URL where the backup container send metrics to
	// +optional
	PromURL string `json:"promURL,omitempty"`

	// KeepJobs amount of jobs to keep for later analysis
	KeepJobs *int `json:"keepJobs,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Check is the Schema for the checks API
type Check struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CheckSpec `json:"spec,omitempty"`
	Status Status    `json:"status,omitempty"`
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

func (c *Check) GetType() JobType {
	return CheckType
}

func (c *Check) GetStatus() *Status {
	return &c.Status
}

func (c *Check) GetResources() corev1.ResourceRequirements {
	return c.Spec.Resources
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
