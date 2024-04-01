package v1

import (
	"fmt"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/k8up-io/k8up/v2/operator/cfg"
)

type (
	// Backend allows configuring several backend implementations.
	// It is expected that users only configure one storage type.
	Backend struct {
		// RepoPasswordSecretRef references a secret key to look up the restic repository password
		RepoPasswordSecretRef *corev1.SecretKeySelector `json:"repoPasswordSecretRef,omitempty"`
		// EnvFrom adds all environment variables from a an external source to the Restic job.
		EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`
		Local   *LocalSpec             `json:"local,omitempty"`
		S3      *S3Spec                `json:"s3,omitempty"`
		GCS     *GCSSpec               `json:"gcs,omitempty"`
		Azure   *AzureSpec             `json:"azure,omitempty"`
		Swift   *SwiftSpec             `json:"swift,omitempty"`
		B2      *B2Spec                `json:"b2,omitempty"`
		Rest    *RestServerSpec        `json:"rest,omitempty"`
	}

	// +k8s:deepcopy-gen=false

	// BackendInterface represents a Backend for internal use.
	BackendInterface interface {
		fmt.Stringer
		EnvVars(vars map[string]*corev1.EnvVarSource) map[string]*corev1.EnvVarSource
	}
)

// GetCredentialEnv will return a map containing the credentials for the given backend.
func (in *Backend) GetCredentialEnv() map[string]*corev1.EnvVarSource {
	vars := make(map[string]*corev1.EnvVarSource)

	if in.RepoPasswordSecretRef != nil {
		vars[cfg.ResticPasswordEnvName] = &corev1.EnvVarSource{
			SecretKeyRef: in.RepoPasswordSecretRef,
		}
	}

	for _, backend := range in.getSupportedBackends() {
		if IsNil(backend) {
			continue
		}
		return backend.EnvVars(vars)
	}

	return nil
}

// String returns the string representation of the repository. If no repo is
// defined it'll return empty string.
func (in *Backend) String() string {

	for _, backend := range in.getSupportedBackends() {
		if IsNil(backend) {
			continue
		}
		return backend.String()
	}
	return ""

}

// IsBackendEqualTo returns true if the restic repository string is equal to the other's string.
// If other is nil, it returns false.
func (in *Backend) IsBackendEqualTo(other *Backend) bool {
	if other == nil {
		return false
	}
	return in.String() == other.String()
}

func (in *Backend) getSupportedBackends() []BackendInterface {
	return []BackendInterface{in.Azure, in.B2, in.GCS, in.Local, in.Rest, in.S3, in.Swift}
}

// IsNil returns true if the given value is nil using reflect.
func IsNil(v interface{}) bool {
	// Unfortunately "v == nil" doesn't work with Interfaces, since they are tuples containing type and value.
	return v == nil || (reflect.ValueOf(v).Kind() == reflect.Ptr && reflect.ValueOf(v).IsNil())
}

func addEnvVarFromSecret(vars map[string]*corev1.EnvVarSource, key string, ref *corev1.SecretKeySelector) {
	if ref != nil {
		vars[key] = &corev1.EnvVarSource{
			SecretKeyRef: ref,
		}
	}
}

type LocalSpec struct {
	MountPath string `json:"mountPath,omitempty"`
}

// EnvVars returns the env vars for this backend.
func (in *LocalSpec) EnvVars(vars map[string]*corev1.EnvVarSource) map[string]*corev1.EnvVarSource {
	return vars
}

// String returns the mountpath.
func (in *LocalSpec) String() string {
	return in.MountPath
}

type S3Spec struct {
	Endpoint                 string                    `json:"endpoint,omitempty"`
	Bucket                   string                    `json:"bucket,omitempty"`
	AccessKeyIDSecretRef     *corev1.SecretKeySelector `json:"accessKeyIDSecretRef,omitempty"`
	SecretAccessKeySecretRef *corev1.SecretKeySelector `json:"secretAccessKeySecretRef,omitempty"`
}

// EnvVars returns the env vars for this backend.
func (in *S3Spec) EnvVars(vars map[string]*corev1.EnvVarSource) map[string]*corev1.EnvVarSource {
	addEnvVarFromSecret(vars, cfg.AwsAccessKeyIDEnvName, in.AccessKeyIDSecretRef)
	addEnvVarFromSecret(vars, cfg.AwsSecretAccessKeyEnvName, in.SecretAccessKeySecretRef)
	return vars
}

// String returns "s3:endpoint/bucket".
// If endpoint or bucket are empty, it uses their global setting accordingly.
func (in *S3Spec) String() string {
	endpoint := cfg.Config.GlobalS3Endpoint
	if in.Endpoint != "" {
		endpoint = in.Endpoint
	}

	bucket := cfg.Config.GlobalS3Bucket
	if in.Bucket != "" {
		bucket = in.Bucket
	}

	return fmt.Sprintf("s3:%v/%v", endpoint, bucket)
}

// RestoreEnvVars returns the env vars for this backend when using Restore jobs.
func (in *S3Spec) RestoreEnvVars() map[string]*corev1.EnvVar {
	vars := make(map[string]*corev1.EnvVar)
	if in.AccessKeyIDSecretRef != nil {
		vars[cfg.RestoreS3AccessKeyIDEnvName] = &corev1.EnvVar{
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: in.AccessKeyIDSecretRef,
			},
		}
	} else {
		vars[cfg.RestoreS3AccessKeyIDEnvName] = &corev1.EnvVar{
			Value: cfg.Config.GlobalRestoreS3AccessKey,
		}
	}

	if in.SecretAccessKeySecretRef != nil {
		vars[cfg.RestoreS3SecretAccessKeyEnvName] = &corev1.EnvVar{
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: in.SecretAccessKeySecretRef,
			},
		}
	} else {
		vars[cfg.RestoreS3SecretAccessKeyEnvName] = &corev1.EnvVar{
			Value: cfg.Config.GlobalRestoreS3SecretAccessKey,
		}
	}

	bucket := in.Bucket
	endpoint := in.Endpoint
	if bucket == "" {
		bucket = cfg.Config.GlobalRestoreS3Bucket
	}
	if endpoint == "" {
		endpoint = cfg.Config.GlobalRestoreS3Endpoint
	}

	vars[cfg.RestoreS3EndpointEnvName] = &corev1.EnvVar{
		Value: fmt.Sprintf("%v/%v", endpoint, bucket),
	}

	return vars
}

type GCSSpec struct {
	Bucket               string                    `json:"bucket,omitempty"`
	ProjectIDSecretRef   *corev1.SecretKeySelector `json:"projectIDSecretRef,omitempty"`
	AccessTokenSecretRef *corev1.SecretKeySelector `json:"accessTokenSecretRef,omitempty"`
}

// EnvVars returns the env vars for this backend.
func (in *GCSSpec) EnvVars(vars map[string]*corev1.EnvVarSource) map[string]*corev1.EnvVarSource {
	addEnvVarFromSecret(vars, cfg.GcsProjectIDEnvName, in.ProjectIDSecretRef)
	addEnvVarFromSecret(vars, cfg.GcsAccessTokenEnvName, in.AccessTokenSecretRef)
	return vars

}

// String returns "gs:bucket:/"
func (in *GCSSpec) String() string {
	return fmt.Sprintf("gs:%s:/", in.Bucket)
}

type AzureSpec struct {
	Container            string                    `json:"container,omitempty"`
	Path                 string                    `json:"path,omitempty"`
	AccountNameSecretRef *corev1.SecretKeySelector `json:"accountNameSecretRef,omitempty"`
	AccountKeySecretRef  *corev1.SecretKeySelector `json:"accountKeySecretRef,omitempty"`
}

// EnvVars returns the env vars for this backend.
func (in *AzureSpec) EnvVars(vars map[string]*corev1.EnvVarSource) map[string]*corev1.EnvVarSource {
	addEnvVarFromSecret(vars, cfg.AzureAccountKeyEnvName, in.AccountKeySecretRef)
	addEnvVarFromSecret(vars, cfg.AzureAccountEnvName, in.AccountNameSecretRef)
	return vars
}

// String returns "azure:container:path"
// If Path is empty, the default value "/" will be used as path
func (in *AzureSpec) String() string {
	path := "/"
	if in.Path != "" {
		path = in.Path
	}
	return fmt.Sprintf("azure:%s:%s", in.Container, path)
}

type SwiftSpec struct {
	Container string `json:"container,omitempty"`
	Path      string `json:"path,omitempty"`
}

// EnvVars returns the env vars for this backend.
func (in *SwiftSpec) EnvVars(vars map[string]*corev1.EnvVarSource) map[string]*corev1.EnvVarSource {
	return vars
}

// String returns "swift:container:path"
func (in *SwiftSpec) String() string {
	return fmt.Sprintf("swift:%s:%s", in.Container, in.Path)
}

type B2Spec struct {
	Bucket              string                    `json:"bucket,omitempty"`
	Path                string                    `json:"path,omitempty"`
	AccountIDSecretRef  *corev1.SecretKeySelector `json:"accountIDSecretRef,omitempty"`
	AccountKeySecretRef *corev1.SecretKeySelector `json:"accountKeySecretRef,omitempty"`
}

// EnvVars returns the env vars for this backend.
func (in *B2Spec) EnvVars(vars map[string]*corev1.EnvVarSource) map[string]*corev1.EnvVarSource {
	addEnvVarFromSecret(vars, cfg.B2AccountIDEnvName, in.AccountIDSecretRef)
	addEnvVarFromSecret(vars, cfg.B2AccountKeyEnvName, in.AccountKeySecretRef)
	return vars
}

// String returns "b2:bucket:path"
func (in *B2Spec) String() string {
	return fmt.Sprintf("b2:%s:%s", in.Bucket, in.Path)
}

type RestServerSpec struct {
	URL               string                    `json:"url,omitempty"`
	UserSecretRef     *corev1.SecretKeySelector `json:"userSecretRef,omitempty"`
	PasswordSecretReg *corev1.SecretKeySelector `json:"passwordSecretReg,omitempty"`
}

// EnvVars returns the env vars for this backend.
func (in *RestServerSpec) EnvVars(vars map[string]*corev1.EnvVarSource) map[string]*corev1.EnvVarSource {
	addEnvVarFromSecret(vars, cfg.RestPasswordEnvName, in.PasswordSecretReg)
	addEnvVarFromSecret(vars, cfg.RestUserEnvName, in.UserSecretRef)
	return vars
}

// String returns "rest:URL"
func (in *RestServerSpec) String() string {
	protocol, url, _ := strings.Cut(in.URL, "://")
	return fmt.Sprintf("rest:%s://%s:%s@%s", protocol, "$(USER)", "$(PASSWORD)", url)
}
