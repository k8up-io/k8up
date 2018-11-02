package service

import (
	"crypto/rand"
	"fmt"
	"time"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	baas8scli "git.vshn.net/vshn/baas/client/k8s/clientset/versioned"
	"git.vshn.net/vshn/baas/config"
	"git.vshn.net/vshn/baas/log"
	"github.com/spotahome/kooper/client/crd"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Shared constants between the various services
const (
	Hostname    = "HOSTNAME"
	KeepLast    = "KEEP_LAST"
	KeepHourly  = "KEEP_HOURLY"
	KeepDaily   = "KEEP_DAILY"
	KeepWeekly  = "KEEP_WEEKLY"
	KeepMonthly = "KEEP_MONTHLY"
	KeepYearly  = "KEEP_YEARLY"
	KeepTag     = "KEEP_TAG"
	StatsURL    = "STATS_URL"
	RestorePath = "/restore"
	PromURL     = "PROM_URL"
)

type CommonObjects struct {
	BaasCLI baas8scli.Interface
	CrdCli  crd.Interface
	K8sCli  kubernetes.Interface
	Logger  log.Logger
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
		if env.Name == backupv1alpha1.ResticRepository {
			return env.Value
		}
	}
	return ""
}

func GetBasicJob(namePrefix string, config config.Global, object metav1.Object) *batchv1.Job {

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

func DefaultEnvs(backend *backupv1alpha1.Backend, config config.Global) []corev1.EnvVar {

	if backend == nil {
		backend = &backupv1alpha1.Backend{S3: &backupv1alpha1.S3Spec{}}
	}

	backendTmp := &backupv1alpha1.Backend{S3: &backupv1alpha1.S3Spec{}}
	if backend.S3 != nil {
		backendTmp = backend
	}

	vars := backendTmp.S3.BackupEnvs(config)

	password := &backupv1alpha1.Backend{}
	if backend.RepoPasswordSecretRef != nil {
		password.RepoPasswordSecretRef = backend.RepoPasswordSecretRef
	}

	vars = append(vars, password.PasswordEnvVar(config))

	vars = append(vars, []corev1.EnvVar{
		{
			Name:  StatsURL,
			Value: config.GlobalStatsURL,
		},
	}...)
	return vars
}
