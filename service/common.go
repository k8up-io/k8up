package service

import (
	"crypto/rand"
	"fmt"
	"time"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	baas8scli "git.vshn.net/vshn/baas/client/k8s/clientset/versioned"
	"git.vshn.net/vshn/baas/log"
	"github.com/spf13/viper"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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
	RestoreS3Endpoint        = "RESTORE_S3ENDPOINT"
	RestoreS3AccessKeyID     = "RESTORE_ACCESSKEYID"
	RestoreS3SecretAccessKey = "RESTORE_SECRETACCESSKEY"
	PromURL                  = "PROM_URL"
)

// GlobalConfig contains configuration that is the same for all services
type GlobalConfig struct {
	Image                          string
	GlobalAccessKeyID              string
	GlobalSecretAccessKey          string
	GlobalRepoPassword             string
	GlobalS3Endpoint               string
	GlobalS3Bucket                 string
	GlobalStatsURL                 string
	GlobalRestoreS3Endpoint        string
	GlobalRestoreS3Bucket          string
	GlobalRestoreS3AccessKeyID     string
	GlobalRestoreS3SecretAccessKey string
	GlobalArchiveS3Endpoint        string
	GlobalArchiveS3Bucket          string
	GlobalArchiveS3AccessKeyID     string
	GlobalArchiveS3SecretAccessKey string
	Label                          string
	Identifier                     string
	RestartPolicy                  string
	GlobalPromURL                  string
	GlobalKeepJobs                 int
}

// CommonObjects contains objects that every service needs at some point.
type CommonObjects struct {
	K8sCli  kubernetes.Interface
	BaasCLI baas8scli.Interface
	Logger  log.Logger
}

// MergeGlobalBackendConfig merges together the
func MergeGlobalBackendConfig(backend *backupv1alpha1.Backend, globalConfig GlobalConfig) *backupv1alpha1.Backend {
	var registerBackend = new(backupv1alpha1.Backend)

	registerBackend.S3 = &backupv1alpha1.S3Spec{}

	registerBackend.S3.Bucket = globalConfig.GlobalS3Bucket
	registerBackend.S3.Endpoint = globalConfig.GlobalS3Endpoint

	if backend != nil {
		if backend.S3 != nil && backend.S3.Bucket != "" && backend.S3.Endpoint != "" {
			registerBackend.S3 = backend.S3
		}
	}

	return registerBackend
}

// NewGlobalConfig returns an instance of GlobalConfig with the fields set to
// the approriate env variables.
func NewGlobalConfig() GlobalConfig {
	initDefaults()
	return GlobalConfig{
		GlobalAccessKeyID:              viper.GetString("GlobalAccessKeyID"),
		GlobalSecretAccessKey:          viper.GetString("GlobalSecretAccessKey"),
		GlobalRepoPassword:             viper.GetString("GlobalRepoPassword"),
		GlobalS3Endpoint:               viper.GetString("GlobalS3Endpoint"),
		GlobalS3Bucket:                 viper.GetString("GlobalS3Bucket"),
		GlobalStatsURL:                 viper.GetString("GlobalStatsURL"),
		GlobalRestoreS3Bucket:          viper.GetString("GlobalRestoreS3Bucket"),
		GlobalRestoreS3Endpoint:        viper.GetString("GlobalRestoreS3Endpoint"),
		GlobalRestoreS3AccessKeyID:     viper.GetString("GlobalRestoreS3AccessKeyID"),
		GlobalRestoreS3SecretAccessKey: viper.GetString("GlobalRestoreS3SecretAccessKey"),
		GlobalArchiveS3Bucket:          viper.GetString("GlobalArchiveS3Bucket"),
		GlobalArchiveS3Endpoint:        viper.GetString("GlobalArchiveS3Endpoint"),
		GlobalArchiveS3AccessKeyID:     viper.GetString("GlobalArchiveS3AccessKeyID"),
		GlobalArchiveS3SecretAccessKey: viper.GetString("GlobalArchiveS3SecretAccessKey"),
		Image:                          viper.GetString("image"),
		Label:                          viper.GetString("label"),
		Identifier:                     viper.GetString("identifier"),
		RestartPolicy:                  viper.GetString("restartPolicy"),
		GlobalPromURL:                  viper.GetString("PromURL"),
		GlobalKeepJobs:                 viper.GetInt("GlobalKeepJobs"),
	}
}

func initDefaults() {
	viper.SetDefault("image", "172.30.1.1:5000/myproject/restic")
	viper.SetDefault("label", "baasresource")
	viper.SetDefault("identifier", "baasid")
	viper.SetDefault("restartPolicy", "OnFailure")
	viper.SetDefault("PromURL", "http://127.0.0.1/")
	viper.SetDefault("GlobalKeepJobs", 10)
}

// BuildS3EnvVars constructs the environment variables for an S3 backup.
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

func BuildRestoreS3Env(s3 *backupv1alpha1.S3Spec, config GlobalConfig) []corev1.EnvVar {
	vars := []corev1.EnvVar{}

	var endpoint string
	if s3.Endpoint != "" && s3.Bucket != "" {
		endpoint = fmt.Sprintf("%v/%v", s3.Endpoint, s3.Bucket)
	} else {
		endpoint = fmt.Sprintf("%v/%v", config.GlobalRestoreS3Endpoint, config.GlobalRestoreS3Bucket)
	}

	restoreEndpoint := corev1.EnvVar{
		Name:  RestoreS3Endpoint,
		Value: endpoint,
	}

	restoreAccessKeyID := corev1.EnvVar{}
	restoreSecretAccessKey := corev1.EnvVar{}

	if config.GlobalRestoreS3AccessKeyID != "" {
		restoreAccessKeyID = corev1.EnvVar{
			Name:  RestoreS3AccessKeyID,
			Value: config.GlobalRestoreS3AccessKeyID,
		}
	}

	if config.GlobalRestoreS3SecretAccessKey != "" {
		restoreSecretAccessKey = corev1.EnvVar{
			Name:  RestoreS3SecretAccessKey,
			Value: config.GlobalRestoreS3SecretAccessKey,
		}
	}

	if s3.AccessKeyIDSecretRef != nil {
		restoreAccessKeyID = corev1.EnvVar{
			Name: RestoreS3AccessKeyID,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: s3.AccessKeyIDSecretRef.LocalObjectReference,
					Key:                  s3.AccessKeyIDSecretRef.Key,
				},
			},
		}
	}

	if s3.SecretAccessKeySecretRef != nil {
		restoreSecretAccessKey = corev1.EnvVar{
			Name: RestoreS3SecretAccessKey,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: s3.SecretAccessKeySecretRef.LocalObjectReference,
					Key:                  s3.SecretAccessKeySecretRef.Key,
				},
			},
		}
	}

	vars = append(vars, restoreAccessKeyID, restoreEndpoint, restoreSecretAccessKey)

	return vars
}

func NewOwnerReference(object metav1.Object) metav1.OwnerReference {
	return metav1.OwnerReference{
		UID:        object.GetUID(),
		APIVersion: backupv1alpha1.SchemeGroupVersion.String(),
		Kind:       backupv1alpha1.RestoreKind,
		Name:       object.GetName(),
	}
}

// PseudoUUID is used to generate IDs for baas related pods/jobs
func PseudoUUID() string {

	b := make([]byte, 16)
	rand.Read(b)

	return fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func GetRepository(pod *corev1.Pod) string {
	// baas pods only have one container
	for _, env := range pod.Spec.Containers[0].Env {
		if env.Name == ResticRepository {
			return env.Value
		}
	}
	return ""
}

func GetBasicJob(namePrefix string, config GlobalConfig, object metav1.Object) *batchv1.Job {

	nameJob := fmt.Sprintf("%vjob-%d", namePrefix, time.Now().Unix())
	namePod := fmt.Sprintf("%vpod-%d", namePrefix, time.Now().Unix())

	labels := map[string]string{
		config.Label:      "true",
		config.Identifier: PseudoUUID(),
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: nameJob,
			OwnerReferences: []metav1.OwnerReference{
				NewOwnerReference(object),
			},
			Labels: labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      namePod,
					Namespace: object.GetNamespace(),
					Labels:    labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicy(config.RestartPolicy),
					Containers: []corev1.Container{
						{
							Name:  namePod,
							Image: config.Image,
							Env: []corev1.EnvVar{
								{
									Name:  Hostname,
									Value: object.GetNamespace(),
								},
							},
							ImagePullPolicy: corev1.PullAlways,
							TTY:             true,
							Stdin:           true,
						},
					},
				},
			},
		},
	}
}
