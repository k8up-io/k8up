package service

import (
	"fmt"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
)

// Shared constants between the various services
const (
	Hostname                 = "HOSTNAME"
	ResticRepository         = "RESTIC_REPOSITORY"
	ResticPassword           = "RESTIC_PASSWORD"
	AwsAccessKeyID           = "AWS_ACCESS_KEY_ID"
	AwsSecretAccessKey       = "AWS_SECRET_ACCESS_KEY"
	KeepLast                 = "KEEP_LAST"
	KeepHourly               = "KEEP_HOURLY"
	KeepDaily                = "KEEP_DAILY"
	KeepWeekly               = "KEEP_WEEKLY"
	KeepMonthly              = "KEEP_MONTHLY"
	KeepYearly               = "KEEP_YEARLY"
	KeepTag                  = "KEEP_TAG"
	StatsURL                 = "STATS_URL"
	RestorePath              = "/restore"
	RestoreS3EndpointEnv     = "RESTORE_S3ENDPOINT"
	RestoreS3AccessKeyIDEnv  = "RESTORE_ACCESSKEYID"
	RestoreS3SecretAccessKey = "RESTORE_SECRETACCESSKEY"
)

// GlobalConfig contains configuration that is the same for all services
type GlobalConfig struct {
	GlobalAccessKeyID     string
	GlobalSecretAccessKey string
	GlobalRepoPassword    string
	GlobalS3Endpoint      string
	GlobalS3Bucket        string
	GlobalStatsURL        string
}

// MergeGlobalBackendConfig merges together the
func MergeGlobalBackendConfig(backend *backupv1alpha1.Backend, globalConfig GlobalConfig) *backupv1alpha1.Backend {
	var registerBackend = new(backupv1alpha1.Backend)

	registerBackend.S3 = &backupv1alpha1.S3Spec{}
	if backend != nil {
		if backend.S3 != nil {
			registerBackend.S3.Bucket = backend.S3.Bucket
			registerBackend.S3.Endpoint = backend.S3.Endpoint
		} else {
			registerBackend.S3.Bucket = globalConfig.GlobalS3Bucket
			registerBackend.S3.Endpoint = globalConfig.GlobalS3Endpoint
		}
	}

	return registerBackend
}

// NewGlobalConfig returns an instance of GlobalConfig with the fields set to
// the approriate env variables.
func NewGlobalConfig() GlobalConfig {
	return GlobalConfig{
		GlobalAccessKeyID:     viper.GetString("GlobalAccessKeyID"),
		GlobalSecretAccessKey: viper.GetString("GlobalSecretAccessKey"),
		GlobalRepoPassword:    viper.GetString("GlobalRepoPassword"),
		GlobalS3Endpoint:      viper.GetString("GlobalS3Endpoint"),
		GlobalS3Bucket:        viper.GetString("GlobalS3Bucket"),
		GlobalStatsURL:        viper.GetString("GlobalStatsURL"),
	}
}

// BuildS3EnvVars constructs the environment variables for an S3 backup
func BuildS3EnvVars(s3Backend *backupv1alpha1.S3Spec, config GlobalConfig) []corev1.EnvVar {
	var tmpEnvs = []corev1.EnvVar{}

	if s3Backend != nil {
		s3Endpoint := s3Backend.Endpoint
		s3Bucket := s3Backend.Bucket

		accessKeyID := corev1.EnvVar{
			Name:  AwsAccessKeyID,
			Value: config.GlobalAccessKeyID,
		}
		secretKeyID := corev1.EnvVar{
			Name:  AwsSecretAccessKey,
			Value: config.GlobalSecretAccessKey,
		}

		if s3Backend.AccessKeyIDSecretRef != nil && s3Backend.SecretAccessKeySecretRef != nil {
			accessKeyID = corev1.EnvVar{
				Name: AwsAccessKeyID,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: s3Backend.AccessKeyIDSecretRef.LocalObjectReference,
						Key:                  s3Backend.AccessKeyIDSecretRef.Key,
					},
				},
			}
			secretKeyID = corev1.EnvVar{
				Name: AwsSecretAccessKey,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: s3Backend.SecretAccessKeySecretRef.LocalObjectReference,
						Key:                  s3Backend.SecretAccessKeySecretRef.Key,
					},
				},
			}
		}

		r := fmt.Sprintf("s3:%s/%s", s3Endpoint, s3Bucket)

		tmpEnvs = append(tmpEnvs, []corev1.EnvVar{
			accessKeyID,
			secretKeyID,
			{
				Name:  ResticRepository,
				Value: r,
			},
		}...)

	}
	return tmpEnvs
}

func BuildRepoPasswordVar(selector *backupv1alpha1.SecretKeySelector, config GlobalConfig) corev1.EnvVar {
	repoPasswordEnv := corev1.EnvVar{
		Name:  ResticPassword,
		Value: config.GlobalRepoPassword,
	}

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
