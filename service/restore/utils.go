package restore

import (
	"fmt"
	"time"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	"git.vshn.net/vshn/baas/service"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newRestoreJob(restore *backupv1alpha1.Restore, volumes []corev1.Volume, config config) *batchv1.Job {

	mounts := make([]corev1.VolumeMount, 0)
	for _, volume := range volumes {
		tmpMount := corev1.VolumeMount{
			Name:      volume.Name,
			MountPath: service.RestorePath,
		}
		mounts = append(mounts, tmpMount)
	}

	args := []string{"-restore"}

	if restore.Spec.RestoreFilter != "" {
		args = append(args, "-restoreFilter", restore.Spec.RestoreFilter)
	}

	if restore.Spec.Snapshot != "" {
		args = append(args, "-restoreSnap", restore.Spec.Snapshot)
	}

	methodDefined := false
	if restore.Spec.RestoreMethod.Folder != nil {
		args = append(args, "-restoreType", "folder")
		methodDefined = true
	}

	if !methodDefined && restore.Spec.RestoreMethod.S3 != nil {
		args = append(args, "-restoreType", "s3")
		methodDefined = true
	}

	jobName := fmt.Sprintf("restorejob-%v", time.Now().Unix())
	podName := fmt.Sprintf("restorepod-%v", time.Now().Unix())

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: restore.Namespace,
			Labels: map[string]string{
				"restorePod": "true",
			},
			OwnerReferences: []metav1.OwnerReference{
				newOwnerReference(restore),
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: restore.Namespace,
					Labels: map[string]string{
						"restorePod": "true",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: "OnFailure",
					Volumes:       volumes,
					Containers: []corev1.Container{
						{
							Name:            podName,
							Image:           config.image,
							VolumeMounts:    mounts,
							Env:             setUpEnvVariables(restore, config),
							ImagePullPolicy: corev1.PullAlways,
							TTY:             true,
							Stdin:           true,
							Args:            args,
						},
					},
				},
			},
		},
	}
}

func setUpEnvVariables(restore *backupv1alpha1.Restore, config config) []corev1.EnvVar {
	vars := make([]corev1.EnvVar, 0)

	if restore.Spec.Backend.S3 != nil {

		s3repoEnv := service.BuildS3EnvVars(restore.GlobalOverrides.RegisteredBackend.S3, config.GlobalConfig)

		vars = append(vars, s3repoEnv...)

		repoPasswordEnv := service.BuildRepoPasswordVar(restore.Spec.RepoPasswordSecretRef, config.GlobalConfig)

		if restore.Spec.RestoreMethod.S3 != nil {

			endpoint := fmt.Sprintf("%v/%v", restore.Spec.RestoreMethod.S3.Endpoint, restore.Spec.RestoreMethod.S3.Bucket)

			restoreEndpoint := corev1.EnvVar{
				Name:  service.RestoreS3EndpointEnv,
				Value: endpoint,
			}

			restoreAccessKeyRef := corev1.EnvVar{
				Name: service.RestoreS3AccessKeyIDEnv,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: restore.Spec.RestoreMethod.S3.AccessKeyIDSecretRef.LocalObjectReference,
						Key:                  restore.Spec.RestoreMethod.S3.AccessKeyIDSecretRef.Key,
					},
				},
			}

			restoreSecretAccessKeyRef := corev1.EnvVar{
				Name: service.RestoreS3SecretAccessKey,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: restore.Spec.RestoreMethod.S3.SecretAccessKeySecretRef.LocalObjectReference,
						Key:                  restore.Spec.RestoreMethod.S3.SecretAccessKeySecretRef.Key,
					},
				},
			}

			vars = append(vars, restoreEndpoint, restoreAccessKeyRef, restoreSecretAccessKeyRef)
		}

		vars = append(vars, []corev1.EnvVar{
			repoPasswordEnv,
		}...)
	}

	return vars
}

func newOwnerReference(restore *backupv1alpha1.Restore) metav1.OwnerReference {
	return metav1.OwnerReference{
		UID:        restore.GetUID(),
		APIVersion: backupv1alpha1.SchemeGroupVersion.String(),
		Kind:       backupv1alpha1.RestoreKind,
		Name:       restore.Name,
	}
}
