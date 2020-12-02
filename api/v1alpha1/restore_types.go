package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// RestoreSpec can either contain an S3 restore point or a local one. For the local
// one you need to define an existing PVC.
type RestoreSpec struct {
	// Backend contains the backend information
	Backend       *Backend       `json:"backend,omitempty"`
	RestoreMethod *RestoreMethod `json:"restoreMethod,omitempty"`
	RestoreFilter string         `json:"restoreFilter,omitempty"`
	Snapshot      string         `json:"snapshot,omitempty"`
	KeepJobs      *int           `json:"keepJobs,omitempty"`
	// Tags is a list of arbitrary tags that get added to the backup via Restic's tagging system
	Tags []string `json:"tags,omitempty"`
}

func (r RestoreSpec) CreateObject(name, namespace string) runtime.Object {
	return &Restore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: r,
	}
}

// RestoreMethod contains how and where the restore should happen
// all the settings are mutual exclusive.
type RestoreMethod struct {
	S3     *S3Spec        `json:"s3,omitempty"`
	Folder *FolderRestore `json:"folder,omitempty"`
}

type FolderRestore struct {
	*corev1.PersistentVolumeClaimVolumeSource `json:",inline"`
}

// RestoreStatus defines the observed state of Restore
type RestoreStatus struct {
	K8upStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Restore is the Schema for the restores API
type Restore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RestoreSpec   `json:"spec,omitempty"`
	Status RestoreStatus `json:"status,omitempty"`
}

func (r *Restore) GetRuntimeObject() runtime.Object {
	return r
}

func (r *Restore) GetMetaObject() metav1.Object {
	return r
}

func (r *Restore) GetType() JobType {
	return RestoreType
}

func (r *Restore) GetK8upStatus() *K8upStatus {
	return &r.Status.K8upStatus
}

// +kubebuilder:object:root=true

// RestoreList contains a list of Restore
type RestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Restore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Restore{}, &RestoreList{})
}
