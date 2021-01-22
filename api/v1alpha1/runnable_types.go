package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/vshn/k8up/api/v1alpha1/backend"
)

// RunnableSpec defines the fields that are necessary on the specs of all actions that are translated to k8s jobs eventually.
type RunnableSpec struct {
	// Backend contains the restic repo where the job should backup to.
	Backend *backend.Backend `json:"backend,omitempty"`

	// Resources describes the compute resource requirements (cpu, memory, etc.)
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}
