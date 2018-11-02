package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Restore contains the restore CRD
type Restore struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              *RestoreSpec  `json:"spec,omitempty"`
	Status            RestoreStatus `json:"status,omitempty"`
}

// RestoreSpec contains all information needed to trigger a restore
type RestoreSpec struct {
	// Backend contains the backend information
	Backend       *Backend       `json:"backend,omitempty"`
	RestoreMethod *RestoreMethod `json:"restoreMethod,omitempty"`
	RestoreFilter string         `json:"restoreFilter,omitempty"`
	Snapshot      string         `json:"snapshot,omitempty"`
	KeepJobs      int            `json:"keepJobs,omitempty"`
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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// RestoreList is a list of BassWorker resources
type RestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Restore `json:"items"`
}

// JobStatus holds information about the various jobs
type JobStatus struct {
	Started  bool `json:"started,omitempty"`
	Finished bool `json:"finished,omitempty"`
	Failed   bool `json:"failed,omitempty"`
}

// RestoreStatus contains the status of a restore job
type RestoreStatus struct {
	JobStatus `json:",inline"`
}
