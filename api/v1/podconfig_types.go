package v1

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups=k8up.io,resources=podconfigs,verbs=get;list;watch

// PodConfigSpec contains the podTemplate definition.
type PodConfigSpec struct {
	Template corev1.PodTemplateSpec `json:"template,omitempty"`
}

// PodConfigStatus defines the observed state of Snapshot
type PodConfigStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PodConfig is the Schema for the PodConcig API
// Any annotations and labels set on this object will also be set on
// the final pod.
type PodConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PodConfigSpec   `json:"spec,omitempty"`
	Status PodConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SnapshotList contains a list of Snapshot
type PodConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PodConfig `json:"items"`
}

func NewPodConfig(ctx context.Context, name, namespace string, c client.Client) (*PodConfig, error) {
	config := &PodConfig{}
	err := c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, config)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return config, nil
}

func init() {
	SchemeBuilder.Register(&PodConfig{}, &PodConfigList{})
}
