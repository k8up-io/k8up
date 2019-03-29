package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PreBackupPod contains a single pod that will be started before the actual
// backup job starts running. This can be used to achieve a given state prior to
// running the backup.
type PreBackupPod struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PreBackupPodSpec `json:"spec,omitempty"`
}

// PreBackupPodSpec contains the configuration for the backup pod and the
// command
type PreBackupPodSpec struct {
	// BackupCommand will be added to the backupcommand annotation on the pod.
	BackupCommand string `json:"backupCommand,omitempty"`
	FileExtension string `json:"fileExtension,omitempty"`
	Pod           *Pod   `json:"pod,omitempty"`
}

// Pod is a dummy struct to fix some code generation issues.
type Pod struct {
	corev1.PodTemplateSpec `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PreBackupPodList holds a list of PreBackupPod
type PreBackupPodList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []PreBackupPod `json:"items"`
}
