package archive

import (
	backupv1alpha1 "github.com/vshn/k8up/apis/backup/v1alpha1"
	"github.com/vshn/k8up/config"
	"github.com/vshn/k8up/service"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

func newArchiveJob(archive *backupv1alpha1.Archive, config config.Global) *batchv1.Job {

	args := []string{"-archive", "-restoreType", "s3"}

	job := service.GetBasicJob(backupv1alpha1.ArchiveKind, config, &archive.ObjectMeta)
	job.Spec.Template.Spec.Containers[0].Args = args
	finalEnv := append(job.Spec.Template.Spec.Containers[0].Env, setUpEnvVariables(archive, config)...)
	job.Spec.Template.Spec.Containers[0].Env = finalEnv

	return job
}

func setUpEnvVariables(archive *backupv1alpha1.Archive, config config.Global) []corev1.EnvVar {
	vars := service.DefaultEnvs(archive.Spec.Backend, config)

	if archive.Spec.RestoreMethod.S3 != nil {
		vars = append(vars, archive.Spec.RestoreMethod.S3.RestoreEnvs(config)...)
	}

	return vars
}
