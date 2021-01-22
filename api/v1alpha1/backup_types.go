package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// BackupSpec defines a single backup. It must contain all inforomation to connect to
// the backup repository when applied. If used with defaults or schedules the operator will
// ensure that the defaults are applied before creating the object on the API.
type BackupSpec struct {
	RunnableSpec `json:",inline"`

	// KeepJobs amount of jobs to keep for later analysis
	KeepJobs *int `json:"keepJobs,omitempty"`

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

type BackupTemplate struct {
	Tags    *[]string `json:"tags,omitempty"`
	Backend Backend   `json:"backend,omitempty"`
	Env     Env       `json:"env,omitempty"`
}

type Env struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Schedule Ref",type="string",JSONPath=`.metadata.ownerReferences[?(@.kind == "Schedule")].name`,description="Reference to Schedule"
// +kubebuilder:printcolumn:name="Completion",type="string",JSONPath=`.status.conditions[?(@.type == "Completed")].reason`,description="Status of Completion"
// +kubebuilder:printcolumn:name="PreBackup",type="string",JSONPath=`.status.conditions[?(@.type == "PreBackupPodsReady")].reason`,description="Status of PreBackupPods"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Backup is the Schema for the backups API
type Backup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BackupSpec `json:"spec,omitempty"`
	Status Status     `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BackupList contains a list of Backup
type BackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Backup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Backup{}, &BackupList{})
}

func (b *Backup) GetRuntimeObject() runtime.Object {
	return b
}

func (b *Backup) GetMetaObject() metav1.Object {
	return b
}

func (*Backup) GetType() JobType {
	return BackupType
}

// GetStatus retrieves the Status property
func (b *Backup) GetStatus() Status {
	return b.Status
}

// SetStatus sets the Status property
func (b *Backup) SetStatus(status Status) {
	b.Status = status
}

func (b *Backup) GetResources() corev1.ResourceRequirements {
	return b.Spec.Resources
}

func (b BackupSpec) CreateObject(name, namespace string) runtime.Object {
	return &Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: b,
	}
}
