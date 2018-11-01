package v1alpha1

import (
	"strings"

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

	// GlobalOverrides is a place where where the result of the global and CRD
	// merge can be saved in the CRD without any influence on Kubernetes.
	GlobalOverrides *GlobalOverrides `json:"-"`
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

type Backend struct {
	// RepoPasswordSecretRef references a secret key to look up the restic repository password
	// +optional
	RepoPasswordSecretRef *SecretKeySelector `json:"repoPasswordSecretRef,omitempty"`
	Local                 *LocalSpec         `json:"local,omitempty"`
	S3                    *S3Spec            `json:"s3,omitempty"`
	GCS                   *GCSSpec           `json:"gcs,omitempty"`
	Azure                 *AzureSpec         `json:"azure,omitempty"`
	Swift                 *SwiftSpec         `json:"swift,omitempty"`
	B2                    *B2Spec            `json:"b2,omitempty"`
	Rest                  *RestServerSpec    `json:"rest,omitempty"`
}

// String returns a stringrepresentation of the repo
// it concatenates all repos comma separated. It may support
// multiple repos in the same instance in the future
func (b *Backend) String() string {
	allRepos := []string{}

	if b.Azure != nil {
		allRepos = append(allRepos, b.Azure.Container)
	}

	if b.B2 != nil {
		allRepos = append(allRepos, b.B2.Bucket)
	}

	if b.GCS != nil {
		allRepos = append(allRepos, b.GCS.Bucket)
	}

	if b.Local != nil {
		allRepos = append(allRepos, b.Local.MountPath)
	}

	if b.Rest != nil {
		allRepos = append(allRepos, b.Rest.URL)
	}

	if b.S3 != nil {
		allRepos = append(allRepos, b.S3.Endpoint+b.S3.Bucket)
	}

	if b.Swift != nil {
		allRepos = append(allRepos, b.Swift.Container)
	}

	return strings.Join(allRepos, ",")

}

type LocalSpec struct {
	corev1.VolumeSource `json:",inline"`
	MountPath           string `json:"mountPath,omitempty"`
	SubPath             string `json:"subPath,omitempty"`
}

type S3Spec struct {
	Endpoint                 string             `json:"endpoint,omitempty"`
	Bucket                   string             `json:"bucket,omitempty"`
	Prefix                   string             `json:"prefix,omitempty"`
	AccessKeyIDSecretRef     *SecretKeySelector `json:"accessKeyIDSecretRef,omitempty"`
	SecretAccessKeySecretRef *SecretKeySelector `json:"secretAccessKeySecretRef,omitempty"`
	Username                 string             `json:"username,omitempty"` //ONLY for development
	Password                 string             `json:"password,omitempty"` //ONLY for development
}

type GCSSpec struct {
	Bucket string `json:"bucket,omitempty"`
	Prefix string `json:"prefix,omitempty"`
}

type AzureSpec struct {
	Container string `json:"container,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
}

type SwiftSpec struct {
	Container string `json:"container,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
}

type B2Spec struct {
	Bucket string `json:"bucket,omitempty"`
	Prefix string `json:"prefix,omitempty"`
}

type RestServerSpec struct {
	URL string `json:"url,omitempty"`
}

type SecretKeySelector struct {
	// The name of the secret in the same namespace to select from.
	corev1.LocalObjectReference `json:",inline"`
	// The key of the secret to select from. Must be a valid secret key.
	Key string `json:"key"`
}

// GlobalOverrides holds information about global params that can get set via
// env variables in the operator. This is not in the normal spec as to avoid
// any overwriting and recreating schedules.
type GlobalOverrides struct {
	// RegisteredBackend is used to track what backend is actually used after
	// the merge with the global settings
	RegisteredBackend *Backend `json:"-"`
}
