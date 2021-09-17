package v1

import (
	corev1 "k8s.io/api/core/v1"
)

// RunnableSpec defines the fields that are necessary on the specs of all actions that are translated to k8s jobs eventually.
type RunnableSpec struct {
	// Backend contains the restic repo where the job should backup to.
	Backend *Backend `json:"backend,omitempty"`

	// Resources describes the compute resource requirements (cpu, memory, etc.)
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// PodSecurityContext describes the security context with which this action shall be executed.
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
}
