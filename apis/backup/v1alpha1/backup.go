package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Backup represents a baas worker.
type Backup struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behaviour of the pod terminator.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#spec-and-status
	// +optional
	Spec *BackupSpec `json:"spec,omitempty"`

	// Status of the backups
	// +optional
	Status BackupStatus `json:"status,omitempty"`
}

// BackupSpec is the spec for a BassWorker resource.
type BackupSpec struct {
	// Backend contains the restic repo where the job should backup to.
	Backend *Backend `json:"backend,omitempty"`
	// KeepJobs amount of jobs to keep for later analysis
	KeepJobs int `json:"keepJobs,omitempty"`

	// PromURL sets a prometheus push URL where the backup container send metrics to
	// +optional
	PromURL string `json:"promURL,omitempty"`
	// StatsURL sets an arbitrary URL where the wrestic container posts metrics and
	// information about the snapshots to. This is in addition to the prometheus
	// pushgateway.
	StatsURL string `json:"statsURL,omitempty"`
	// Tags is a list of arbitrary tags that get added to the backup via Restic's tagging system
	Tags []string `json:"tags,omitempty"`
}

type BackupStatus struct {
	// LastBackupStart time
	// +optional
	LastBackupStart string `json:"lastBackupStart,omitempty"`
	// LastBackupEnd time
	// +optional
	LastBackupEnd string `json:"lastBackupEnd,omitempty"`
	// LastBackupStatus
	// +optional
	LastBackupStatus string `json:"lastBackupDtatus,omitempty"`
	JobStatus        `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BackupList is a list of BassWorker resources
type BackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Backup `json:"items"`
}

type SecretKeySelector struct {
	// The name of the secret in the same namespace to select from.
	corev1.LocalObjectReference `json:",inline"`
	// The key of the secret to select from. Must be a valid secret key.
	Key string `json:"key"`
}

func (b BackupList) Len() int      { return len(b.Items) }
func (b BackupList) Swap(i, j int) { b.Items[i], b.Items[j] = b.Items[j], b.Items[i] }

func (b BackupList) Less(i, j int) bool {

	if b.Items[i].CreationTimestamp.Equal(&b.Items[j].CreationTimestamp) {
		return b.Items[i].Name < b.Items[j].Name
	}

	return b.Items[i].CreationTimestamp.Before(&b.Items[j].CreationTimestamp)
}
