package backend

import (
	"fmt"

	"github.com/vshn/k8up/cfg"

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

type BackendInterface interface {
	IsEqualTo(backendInterface BackendInterface) bool
}

// GetCredentialEnv will return a map containing the correct
func (b *Backend) GetCredentialEnv() map[string]*corev1.EnvVarSource {
	vars := make(map[string]*corev1.EnvVarSource)

	if b.RepoPasswordSecretRef != nil {
		vars[cfg.ResticPasswordEnvName] = &corev1.EnvVarSource{
			SecretKeyRef: b.RepoPasswordSecretRef,
		}
	}

	if b.Azure != nil {
		return b.Azure.EnvVars(vars)
	}

	if b.B2 != nil {
		return b.B2.EnvVars(vars)
	}

	if b.GCS != nil {
		return b.GCS.EnvVars(vars)
	}

	if b.Local != nil {
		return b.Local.EnvVars(vars)
	}

	if b.Rest != nil {
		return b.Rest.EnvVars(vars)
	}

	if b.S3 != nil {
		return b.S3.EnvVars(vars)
	}

	if b.Swift != nil {
		return b.Swift.EnvVars(vars)
	}

	return nil
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
		endpoint := cfg.Config.GlobalS3Endpoint
		if b.S3.Endpoint != "" {
			endpoint = b.S3.Endpoint
		}

		bucket := cfg.Config.GlobalS3Bucket
		if b.S3.Bucket != "" {
			bucket = b.S3.Bucket
		}

		return fmt.Sprintf("s3:%v/%v", endpoint, bucket)
	}

	if b.Swift != nil {
		return fmt.Sprintf("swift:%s:%s", b.Swift.Container, b.Swift.Path)
	}

	return ""

}

func (in *Backend) IsEqualTo(backend *Backend) bool {
	// TODO: implement other backends
	for _, impl := range []BackendInterface{backend.S3} {
		if impl == nil {
			continue
		}
		if impl.IsEqualTo(in.S3) {
			return true
		}
	}
	return false
}
