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

	// PodConfigRef describes the pod spec with wich this action shall be executed.
	// It takes precedence over the Resources or PodSecurityContext field.
	// It does not allow changing the image or the command of the resulting pod.
	// This is for advanced use-cases only. Please only set this if you know what you're doing.
	PodConfigRef *corev1.LocalObjectReference `json:"podConfigRef,omitempty"`

	// Volumes List of volumes that can be mounted by containers belonging to the pod.
	Volumes *[]RunnableVolumeSpec `json:"volumes,omitempty"`

	// ActiveDeadlineSeconds specifies the duration in seconds relative to the startTime that the job may be continuously active before the system tries to terminate it.
	// Value must be positive integer if given.
	ActiveDeadlineSeconds *int64 `json:"activeDeadlineSeconds,omitempty"`
}

type RunnableVolumeSpec struct {
	// name of the volume.
	// Must be a DNS_LABEL and unique within the pod.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	Name string `json:"name"`

	// persistentVolumeClaimVolumeSource represents a reference to a
	// PersistentVolumeClaim in the same namespace.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#persistentvolumeclaims
	// +optional
	PersistentVolumeClaim *corev1.PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty"`
	// secret represents a secret that should populate this volume.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#secret
	// +optional
	Secret *corev1.SecretVolumeSource `json:"secret,omitempty"`
	// configMap represents a configMap that should populate this volume
	// +optional
	ConfigMap *corev1.ConfigMapVolumeSource `json:"configMap,omitempty"`
}

// AppendEnvFromToContainer will add EnvFromSource from the given RunnableSpec to the Container
func (in *RunnableSpec) AppendEnvFromToContainer(containerSpec *corev1.Container) {
	if in.Backend != nil {
		containerSpec.EnvFrom = append(containerSpec.EnvFrom, in.Backend.EnvFrom...)
	}
}
