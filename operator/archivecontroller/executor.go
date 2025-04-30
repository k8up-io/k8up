package archivecontroller

import (
	"context"

	"github.com/k8up-io/k8up/v2/operator/executor"
	"github.com/k8up-io/k8up/v2/operator/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/job"
)

const (
	archivePath    = "/archive"
	certPrefixName = "restore"
)

// ArchiveExecutor will execute the batch.job for archive.
type ArchiveExecutor struct {
	executor.Generic
	archive *k8upv1.Archive
}

// NewArchiveExecutor will return a new executor for archive jobs.
func NewArchiveExecutor(config job.Config) *ArchiveExecutor {
	return &ArchiveExecutor{
		Generic: executor.Generic{Config: config},
		archive: config.Obj.(*k8upv1.Archive),
	}
}

// GetConcurrencyLimit returns the concurrent jobs limit
func (a *ArchiveExecutor) GetConcurrencyLimit() int {
	return cfg.Config.GlobalConcurrentArchiveJobsLimit
}

// Execute creates the actual batch.job on the k8s api.
func (a *ArchiveExecutor) Execute(ctx context.Context) error {
	log := controllerruntime.LoggerFrom(ctx)

	batchJob := &batchv1.Job{}
	batchJob.Name = a.jobName()
	batchJob.Namespace = a.archive.Namespace

	_, err := controllerutil.CreateOrUpdate(ctx, a.Client, batchJob, func() error {
		mutateErr := job.MutateBatchJob(ctx, batchJob, a.archive, a.Config, a.Client)
		if mutateErr != nil {
			return mutateErr
		}

		batchJob.Spec.Template.Spec.Containers[0].Env = append(batchJob.Spec.Template.Spec.Containers[0].Env, a.setupEnvVars(ctx, a.archive)...)
		a.archive.Spec.AppendEnvFromToContainer(&batchJob.Spec.Template.Spec.Containers[0])
		batchJob.Spec.Template.Spec.Containers[0].VolumeMounts = a.attachTLSVolumeMounts()
		batchJob.Spec.Template.Spec.Volumes = utils.AttachEmptyDirVolumes(a.archive.Spec.Volumes)

		batchJob.Spec.Template.Spec.Containers[0].Args = a.setupArgs()

		return nil
	})
	if err != nil {
		log.Error(err, "could not create job")
		a.SetConditionFalseWithMessage(ctx, k8upv1.ConditionReady, k8upv1.ReasonCreationFailed, "could not create job: %v", err)
		return err
	}

	a.SetStarted(ctx, "the job '%v/%v' was created", batchJob.Namespace, batchJob.Name)
	return nil
}

func (a *ArchiveExecutor) jobName() string {
	return k8upv1.ArchiveType.String() + "-" + a.Obj.GetName()
}

func (a *ArchiveExecutor) setupArgs() []string {
	args := []string{"-varDir", cfg.Config.PodVarDir, "-archive", "-restoreType", "s3"}
	if a.archive.Spec.RestoreSpec != nil && len(a.archive.Spec.Tags) > 0 {
		args = append(args, executor.BuildTagArgs(a.archive.Spec.Tags)...)
	}
	if a.archive.Spec.Backend != nil {
		args = append(args, utils.AppendTLSOptionsArgs(a.archive.Spec.Backend.TLSOptions)...)
	}
	if a.archive.Spec.RestoreSpec != nil && a.archive.Spec.RestoreMethod != nil {
		args = append(args, utils.AppendTLSOptionsArgs(a.archive.Spec.RestoreMethod.TLSOptions, certPrefixName)...)
	}

	return args
}

func (a *ArchiveExecutor) setupEnvVars(ctx context.Context, archive *k8upv1.Archive) []corev1.EnvVar {
	log := controllerruntime.LoggerFrom(ctx)
	vars := executor.NewEnvVarConverter()

	if archive.Spec.RestoreSpec != nil && archive.Spec.RestoreMethod != nil {
		for key, value := range archive.Spec.RestoreMethod.S3.RestoreEnvVars() {
			// FIXME(mw): ugly, due to EnvVarConverter()
			if value.Value != "" {
				vars.SetString(key, value.Value)
			} else {
				vars.SetEnvVarSource(key, value.ValueFrom)
			}
		}
	}

	if archive.Spec.RestoreSpec != nil && archive.Spec.RestoreMethod != nil {
		if archive.Spec.RestoreMethod.Folder != nil {
			vars.SetString("RESTORE_DIR", archivePath)
		}
	}

	if archive.Spec.Backend != nil {
		for key, value := range archive.Spec.Backend.GetCredentialEnv() {
			vars.SetEnvVarSource(key, value)
		}
		vars.SetString(cfg.ResticRepositoryEnvName, archive.Spec.Backend.String())
	}

	err := vars.Merge(executor.DefaultEnv(a.Obj.GetNamespace()))
	if err != nil {
		log.Error(err, "error while merging the environment variables", "name", a.Obj.GetName(), "namespace", a.Obj.GetNamespace())
	}

	return vars.Convert()
}

func (a *ArchiveExecutor) cleanupOldArchives(ctx context.Context, archive *k8upv1.Archive) {
	a.CleanupOldResources(ctx, &k8upv1.ArchiveList{}, archive.Namespace, archive)
}

func (a *ArchiveExecutor) attachTLSVolumeMounts() []corev1.VolumeMount {
	var tlsVolumeMounts []corev1.VolumeMount
	if a.archive.Spec.Backend != nil && !utils.ZeroLen(a.archive.Spec.Backend.VolumeMounts) {
		tlsVolumeMounts = append(tlsVolumeMounts, *a.archive.Spec.Backend.VolumeMounts...)
	}
	if a.archive.Spec.RestoreSpec != nil && a.archive.Spec.RestoreMethod != nil && !utils.ZeroLen(a.archive.Spec.RestoreMethod.VolumeMounts) {
		tlsVolumeMounts = append(tlsVolumeMounts, *a.archive.Spec.RestoreMethod.VolumeMounts...)
	}

	return utils.AttachEmptyDirVolumeMounts(cfg.Config.PodVarDir, &tlsVolumeMounts)
}
