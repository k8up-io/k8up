package v1alpha1

import (
	"fmt"

	"github.com/vshn/k8up/constants"
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

// GetCredentialEnv will return a map containing the correct
func (b *Backend) GetCredentialEnv() map[string]*corev1.EnvVarSource {
	vars := make(map[string]*corev1.EnvVarSource)

	vars[constants.ResticPasswordEnvName] = &corev1.EnvVarSource{
		SecretKeyRef: b.RepoPasswordSecretRef,
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

func (l *LocalSpec) EnvVars(vars map[string]*corev1.EnvVarSource) map[string]*corev1.EnvVarSource {
	return vars
}

type S3Spec struct {
	Endpoint                 string                    `json:"endpoint,omitempty"`
	Bucket                   string                    `json:"bucket,omitempty"`
	AccessKeyIDSecretRef     *corev1.SecretKeySelector `json:"accessKeyIDSecretRef,omitempty"`
	SecretAccessKeySecretRef *corev1.SecretKeySelector `json:"secretAccessKeySecretRef,omitempty"`
}

func (s *S3Spec) EnvVars(vars map[string]*corev1.EnvVarSource) map[string]*corev1.EnvVarSource {
	if s.AccessKeyIDSecretRef != nil {
		vars["AWS_ACCESS_KEY_ID"] = &corev1.EnvVarSource{
			SecretKeyRef: s.AccessKeyIDSecretRef,
		}
	}

	if s.SecretAccessKeySecretRef != nil {
		vars["AWS_SECRET_ACCESS_KEY"] = &corev1.EnvVarSource{
			SecretKeyRef: s.SecretAccessKeySecretRef,
		}
	}

	return vars
}

type GCSSpec struct {
	Bucket               string                    `json:"bucket,omitempty"`
	ProjectIDSecretRef   *corev1.SecretKeySelector `json:"projectIDSecretRef,omitempty"`
	AccessTokenSecretRef *corev1.SecretKeySelector `json:"accessTokenSecretRef,omitempty"`
}

func (g *GCSSpec) EnvVars(vars map[string]*corev1.EnvVarSource) map[string]*corev1.EnvVarSource {
	if g.ProjectIDSecretRef != nil {
		vars["GOOGLE_PROJECT_ID"] = &corev1.EnvVarSource{
			SecretKeyRef: g.ProjectIDSecretRef,
		}
	}

	if g.AccessTokenSecretRef != nil {
		vars["GOOGLE_ACCESS_TOKEN"] = &corev1.EnvVarSource{
			SecretKeyRef: g.AccessTokenSecretRef,
		}
	}

	return vars

}

type AzureSpec struct {
	Container            string                    `json:"container,omitempty"`
	AccountNameSecretRef *corev1.SecretKeySelector `json:"accountNameSecretRef,omitempty"`
	AccountKeySecreftRef *corev1.SecretKeySelector `json:"accountKeySecreftRef,omitempty"`
}

func (a *AzureSpec) EnvVars(vars map[string]*corev1.EnvVarSource) map[string]*corev1.EnvVarSource {
	if a.AccountKeySecreftRef != nil {
		vars["AZURE_ACCOUNT_KEY"] = &corev1.EnvVarSource{
			SecretKeyRef: a.AccountKeySecreftRef,
		}
	}

	if a.AccountNameSecretRef != nil {
		vars["AZURE_ACCOUNT_NAME"] = &corev1.EnvVarSource{
			SecretKeyRef: a.AccountNameSecretRef,
		}
	}

	return vars
}

type SwiftSpec struct {
	Container string `json:"container,omitempty"`
	Path      string `json:"path,omitempty"`
}

func (s *SwiftSpec) EnvVars(vars map[string]*corev1.EnvVarSource) map[string]*corev1.EnvVarSource {
	return vars
}

type B2Spec struct {
	Bucket              string                    `json:"bucket,omitempty"`
	Path                string                    `json:"path,omitempty"`
	AccountIDSecretRef  *corev1.SecretKeySelector `json:"accountIDSecretRef,omitempty"`
	AccountKeySecretRef *corev1.SecretKeySelector `json:"accountKeySecretRef,omitempty"`
}

func (b *B2Spec) EnvVars(vars map[string]*corev1.EnvVarSource) map[string]*corev1.EnvVarSource {
	if b.AccountIDSecretRef != nil {
		vars["B2_ACCOUNT_ID"] = &corev1.EnvVarSource{
			SecretKeyRef: b.AccountIDSecretRef,
		}
	}

	if b.AccountKeySecretRef != nil {
		vars["B2_ACCOUNT_KEY"] = &corev1.EnvVarSource{
			SecretKeyRef: b.AccountKeySecretRef,
		}
	}

	return vars
}

type RestServerSpec struct {
	URL               string                    `json:"url,omitempty"`
	UserSecretRef     *corev1.SecretKeySelector `json:"userSecretRef,omitempty"`
	PasswordSecretReg *corev1.SecretKeySelector `json:"passwordSecretReg,omitempty"`
}

func (r *RestServerSpec) EnvVars(vars map[string]*corev1.EnvVarSource) map[string]*corev1.EnvVarSource {
	if r.PasswordSecretReg != nil {
		vars["PASSWORD"] = &corev1.EnvVarSource{
			SecretKeyRef: r.PasswordSecretReg,
		}
	}

	if r.UserSecretRef != nil {
		vars["USER"] = &corev1.EnvVarSource{
			SecretKeyRef: r.UserSecretRef,
		}
	}

	return vars
}
