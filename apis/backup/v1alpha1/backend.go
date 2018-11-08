package v1alpha1

import (
	"fmt"

	"git.vshn.net/vshn/baas/config"
	corev1 "k8s.io/api/core/v1"
)

const (
	RestoreS3Endpoint        = "RESTORE_S3ENDPOINT"
	RestoreS3AccessKeyID     = "RESTORE_ACCESSKEYID"
	RestoreS3SecretAccessKey = "RESTORE_SECRETACCESSKEY"
	ResticRepository         = "RESTIC_REPOSITORY"
	ResticPassword           = "RESTIC_PASSWORD"
	AwsAccessKeyID           = "AWS_ACCESS_KEY_ID"
	AwsSecretAccessKey       = "AWS_SECRET_ACCESS_KEY"
)

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

// String returns the string representation of the repository. If no repo is
// defined it'll return empty string.
func (b *Backend) String() string {

	if b.Azure != nil {
		return b.Azure.Container
	}

	if b.B2 != nil {
		return b.B2.Bucket
	}

	if b.GCS != nil {
		return b.GCS.Bucket
	}

	if b.Local != nil {
		return b.Local.MountPath
	}

	if b.Rest != nil {
		return b.Rest.URL
	}

	if b.S3 != nil {
		return fmt.Sprintf("s3:%v/%v", b.S3.Endpoint, b.S3.Bucket)
	}

	if b.Swift != nil {
		return b.Swift.Container
	}

	return ""

}

func (b *Backend) Merge(config config.Global) {

	if b == nil {
		b = &Backend{}
	}

	// Currently only S3 is implemented
	if b.S3 == nil {
		b.S3 = &S3Spec{
			Endpoint: config.GlobalS3Endpoint,
			Bucket:   config.GlobalS3Bucket,
		}
	} else {
		if b.S3.Endpoint == "" {
			b.S3.Endpoint = config.GlobalS3Endpoint
		}
		if b.S3.Bucket == "" {
			b.S3.Bucket = config.GlobalS3Bucket
		}
	}
}

func (b *Backend) PasswordEnvVar(config config.Global) corev1.EnvVar {
	repoPasswordEnv := corev1.EnvVar{
		Name:  ResticPassword,
		Value: config.GlobalRepoPassword,
	}

	selector := b.RepoPasswordSecretRef

	if selector != nil {
		repoPasswordEnv = corev1.EnvVar{
			Name: ResticPassword,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: selector.LocalObjectReference,
					Key:                  selector.Key,
				},
			},
		}
	}
	return repoPasswordEnv
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

func (s *S3Spec) BackupEnvs(config config.Global) []corev1.EnvVar {
	return s.repoEnvs(AwsAccessKeyID, AwsSecretAccessKey, ResticRepository, false, config)
}

// TODO: test if merging still works correctly
func (s *S3Spec) repoEnvs(awsAccessKeyIDName, awsSecretAccessKeyName, repoName string, restore bool, config config.Global) []corev1.EnvVar {
	var tmpEnvs = []corev1.EnvVar{}

	var accessKeyValue string
	var secretAccessKeyValue string

	if restore {
		accessKeyValue = config.GlobalRestoreS3AccessKeyID
		secretAccessKeyValue = config.GlobalRestoreS3SecretAccessKey
	} else {
		accessKeyValue = config.GlobalAccessKeyID
		secretAccessKeyValue = config.GlobalSecretAccessKey
	}

	accessKeyID := corev1.EnvVar{
		Name:  awsAccessKeyIDName,
		Value: accessKeyValue,
	}
	secretKeyID := corev1.EnvVar{
		Name:  awsSecretAccessKeyName,
		Value: secretAccessKeyValue,
	}

	if s.AccessKeyIDSecretRef != nil && s.SecretAccessKeySecretRef != nil {
		accessKeyID = corev1.EnvVar{
			Name: awsAccessKeyIDName,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: s.AccessKeyIDSecretRef.LocalObjectReference,
					Key:                  s.AccessKeyIDSecretRef.Key,
				},
			},
		}
		secretKeyID = corev1.EnvVar{
			Name: awsSecretAccessKeyName,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: s.SecretAccessKeySecretRef.LocalObjectReference,
					Key:                  s.SecretAccessKeySecretRef.Key,
				},
			},
		}
	}

	var endpoint string
	if s.Endpoint != "" && s.Bucket != "" {
		endpoint = fmt.Sprintf("%v/%v", s.Endpoint, s.Bucket)
		if !restore {
			endpoint = "s3:" + endpoint
		}
	} else if restore {
		endpoint = fmt.Sprintf("%v/%v", config.GlobalRestoreS3Endpoint, config.GlobalRestoreS3Bucket)
	} else {
		endpoint = fmt.Sprintf("s3:%v/%v", config.GlobalS3Endpoint, config.GlobalS3Bucket)
	}

	tmpEnvs = append(tmpEnvs, []corev1.EnvVar{
		accessKeyID,
		secretKeyID,
		{
			Name:  repoName,
			Value: endpoint,
		},
	}...)

	return tmpEnvs
}

func (s *S3Spec) RestoreEnvs(config config.Global) []corev1.EnvVar {
	return s.repoEnvs(RestoreS3AccessKeyID, RestoreS3SecretAccessKey, RestoreS3Endpoint, true, config)
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
