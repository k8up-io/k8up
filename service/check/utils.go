package check

import (
	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	"git.vshn.net/vshn/baas/config"
	"git.vshn.net/vshn/baas/service"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

func newCheckJob(check *backupv1alpha1.Check, config config.Global) *batchv1.Job {

	job := service.GetBasicJob(backupv1alpha1.CheckKind, config, &check.ObjectMeta)

	finalEnv := append(job.Spec.Template.Spec.Containers[0].Env, setUpEnvVariables(check, config)...)

	job.Spec.Template.Spec.Containers[0].Env = finalEnv
	job.Spec.Template.Spec.Containers[0].Args = []string{"-check"}

	return job
}

func setUpEnvVariables(check *backupv1alpha1.Check, config config.Global) []corev1.EnvVar {
	envVars := service.DefaultEnvs(check.Spec.Backend, config)

	promURL := config.GlobalPromURL
	if check.Spec.PromURL != "" {
		promURL = check.Spec.PromURL
	}

	envVars = append(envVars, corev1.EnvVar{
		Name:  service.PromURL,
		Value: promURL,
	})

	return envVars
}
