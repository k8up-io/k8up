package restore

import (
	backupv1alpha1 "github.com/vshn/k8up/apis/backup/v1alpha1"
	"github.com/vshn/k8up/service"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

func newRestoreJob(restore *backupv1alpha1.Restore, config config) *batchv1.Job {

	volumes := []corev1.Volume{}
	if restore.Spec.RestoreMethod.S3 == nil {
		volumes = append(volumes,
			corev1.Volume{
				Name: restore.Spec.RestoreMethod.Folder.ClaimName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: restore.Spec.RestoreMethod.Folder.PersistentVolumeClaimVolumeSource,
				},
			})
	}

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

	restoreJob := service.GetBasicJob("restore", config.Global, &restore.ObjectMeta)

	restoreJob.Spec.Template.Spec.Containers[0].Args = args

	restoreJob.Spec.Template.Spec.Containers[0].Env = setUpEnvVariables(restore, config)

	return restoreJob
}

func setUpEnvVariables(restore *backupv1alpha1.Restore, config config) []corev1.EnvVar {
	vars := service.DefaultEnvs(restore.Spec.Backend, config.Global)

	vars = append(vars, restore.Spec.Backend.S3.RestoreEnvs(config.Global)...)

	return vars
}
