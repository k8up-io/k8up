package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

type Backend struct {
	// RepoPasswordSecretRef references a secret key to look up the restic repository password
	RepoPasswordSecretRef *corev1.SecretKeySelector `json:"repoPasswordSecretRef,omitempty"`
	Local                 *LocalSpec                `json:"local,omitempty"`
	S3                    *S3Spec                   `json:"s3,omitempty"`
	GCS                   *GCSSpec                  `json:"gcs,omitempty"`
	Azure                 *AzureSpec                `json:"azure,omitempty"`
	Swift                 *SwiftSpec                `json:"swift,omitempty"`
	B2                    *B2Spec                   `json:"b2,omitempty"`
	Rest                  *RestServerSpec           `json:"rest,omitempty"`
}

// String returns the string representation of the repository. If no repo is
// defined it'll return empty string.
func (b *Backend) String() string {

	if b.Azure != nil {
		return fmt.Sprintf("azure:%s:/", b.Azure.Container)
	}

	if b.B2 != nil {
		return fmt.Sprintf("b2:%s:%s", b.B2.Bucket, b.B2.Path)
	}

	if b.GCS != nil {
		return fmt.Sprintf("gs:%s:/", b.GCS.Bucket)
	}

	if b.Local != nil {
		return b.Local.MountPath
	}

	if b.Rest != nil {
		return fmt.Sprintf("rest:%s", b.Rest.URL)
	}

	if b.S3 != nil {
		return fmt.Sprintf("s3:%v/%v", b.S3.Endpoint, b.S3.Bucket)
	}

	if b.Swift != nil {
		return fmt.Sprintf("swift:%s:%s", b.Swift.Container, b.Swift.Path)
	}

	return ""

}

type LocalSpec struct {
	MountPath string `json:"mountPath,omitempty"`
}

type S3Spec struct {
	Endpoint string `json:"endpoint,omitempty"`
	Bucket   string `json:"bucket,omitempty"`
}

type GCSSpec struct {
	Bucket string `json:"bucket,omitempty"`
}

type AzureSpec struct {
	Container string `json:"container,omitempty"`
}

type SwiftSpec struct {
	Container string `json:"container,omitempty"`
	Path      string `json:"path,omitempty"`
}

type B2Spec struct {
	Bucket string `json:"bucket,omitempty"`
	Path   string `json:"path,omitempty"`
}

type RestServerSpec struct {
	URL string `json:"url,omitempty"`
}
