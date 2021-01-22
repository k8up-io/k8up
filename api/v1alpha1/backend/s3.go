package backend

import (
	"fmt"

	"k8s.io/api/core/v1"

	"github.com/vshn/k8up/cfg"
)

type (
	S3Spec struct {
		Endpoint                 string                `json:"endpoint,omitempty"`
		Bucket                   string                `json:"bucket,omitempty"`
		AccessKeyIDSecretRef     *v1.SecretKeySelector `json:"accessKeyIDSecretRef,omitempty"`
		SecretAccessKeySecretRef *v1.SecretKeySelector `json:"secretAccessKeySecretRef,omitempty"`
	}
)

func (s *S3Spec) EnvVars(vars map[string]*v1.EnvVarSource) map[string]*v1.EnvVarSource {
	if s.AccessKeyIDSecretRef != nil {
		vars["AWS_ACCESS_KEY_ID"] = &v1.EnvVarSource{
			SecretKeyRef: s.AccessKeyIDSecretRef,
		}
	}

	if s.SecretAccessKeySecretRef != nil {
		vars["AWS_SECRET_ACCESS_KEY"] = &v1.EnvVarSource{
			SecretKeyRef: s.SecretAccessKeySecretRef,
		}
	}

	return vars
}

func (s *S3Spec) RestoreEnvVars() map[string]*v1.EnvVar {
	vars := make(map[string]*v1.EnvVar)
	if s.AccessKeyIDSecretRef != nil {
		vars[cfg.RestoreS3AccessKeyIDEnvName] = &v1.EnvVar{
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: s.AccessKeyIDSecretRef,
			},
		}
	} else {
		vars[cfg.RestoreS3AccessKeyIDEnvName] = &v1.EnvVar{
			Value: cfg.Config.GlobalRestoreS3AccessKey,
		}
	}

	if s.SecretAccessKeySecretRef != nil {
		vars[cfg.RestoreS3SecretAccessKeyEnvName] = &v1.EnvVar{
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: s.SecretAccessKeySecretRef,
			},
		}
	} else {
		vars[cfg.RestoreS3SecretAccessKeyEnvName] = &v1.EnvVar{
			Value: cfg.Config.GlobalRestoreS3SecretAccessKey,
		}
	}

	bucket := s.Bucket
	endpoint := s.Endpoint
	if bucket == "" {
		bucket = cfg.Config.GlobalRestoreS3Bucket
	}
	if endpoint == "" {
		endpoint = cfg.Config.GlobalRestoreS3Endpoint
	}

	vars[cfg.RestoreS3EndpointEnvName] = &v1.EnvVar{
		Value: fmt.Sprintf("%v/%v", endpoint, bucket),
	}

	return vars
}

func (s *S3Spec) IsEqualTo(other BackendInterface) bool {
	if o, ok := other.(*S3Spec); ok {
		return o.Endpoint == s.Endpoint && o.Bucket == s.Bucket
	}
	return false
}
