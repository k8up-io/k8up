package check

import (
	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	"git.vshn.net/vshn/baas/service"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

type byCreationTime []backupv1alpha1.Check

func (b byCreationTime) Len() int      { return len(b) }
func (b byCreationTime) Swap(i, j int) { b[i], b[j] = b[j], b[i] }

func (b byCreationTime) Less(i, j int) bool {

	if b[i].CreationTimestamp.Equal(&b[j].CreationTimestamp) {
		return b[i].Name < b[j].Name
	}

	return b[i].CreationTimestamp.Before(&b[j].CreationTimestamp)
}

func newCheckJob(check *backupv1alpha1.Check, config config) *batchv1.Job {

	job := service.GetBasicJob("check", config.GlobalConfig, &check.ObjectMeta)

	finalEnv := append(job.Spec.Template.Spec.Containers[0].Env, setUpEnvVariables(check, config)...)

	job.Spec.Template.Spec.Containers[0].Env = finalEnv
	job.Spec.Template.Spec.Containers[0].Args = []string{"-check"}

	return job
}

func setUpEnvVariables(check *backupv1alpha1.Check, config config) []corev1.EnvVar {
	envVars := make([]corev1.EnvVar, 0)

	envVars = append(envVars, service.BuildS3EnvVars(check.GlobalOverrides.RegisteredBackend.S3, config.GlobalConfig)...)

	envVars = append(envVars, service.BuildRepoPasswordVar(check.GlobalOverrides.RegisteredBackend.RepoPasswordSecretRef, config.GlobalConfig))

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
