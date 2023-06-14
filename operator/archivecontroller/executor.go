package archivecontroller

import (
	"context"

	"github.com/k8up-io/k8up/v2/operator/executor"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/job"
)

const archivePath = "/archive"

// ArchiveExecutor will execute the batch.job for archive.
type ArchiveExecutor struct {
	executor.Generic
}

// NewArchiveExecutor will return a new executor for archive jobs.
func NewArchiveExecutor(config job.Config) *ArchiveExecutor {
	return &ArchiveExecutor{
		Generic: executor.Generic{Config: config},
	}
}

// GetConcurrencyLimit returns the concurrent jobs limit
func (a *ArchiveExecutor) GetConcurrencyLimit() int {
	return cfg.Config.GlobalConcurrentArchiveJobsLimit
}

// Execute creates the actual batch.job on the k8s api.
func (a *ArchiveExecutor) Execute(ctx context.Context) error {
	log := controllerruntime.LoggerFrom(ctx)
	archive := a.Obj.(*k8upv1.Archive)

	batchJob := &batchv1.Job{}
	batchJob.Name = a.jobName()
	batchJob.Namespace = archive.Namespace

	_, err := controllerutil.CreateOrUpdate(ctx, a.Client, batchJob, func() error {
		mutateErr := job.MutateBatchJob(batchJob, archive, a.Config)
		if mutateErr != nil {
			return mutateErr
		}

		batchJob.Spec.Template.Spec.Containers[0].Env = a.setupEnvVars(ctx, archive)
		archive.Spec.AppendEnvFromToContainer(&batchJob.Spec.Template.Spec.Containers[0])
		batchJob.Spec.Template.Spec.Containers[0].Args = a.setupArgs(archive)
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

func (a *ArchiveExecutor) setupArgs(archive *k8upv1.Archive) []string {
	args := []string{"-archive", "-restoreType", "s3"}

	if archive.Spec.RestoreSpec != nil {
		if len(archive.Spec.RestoreSpec.Tags) > 0 {
			args = append(args, executor.BuildTagArgs(archive.Spec.RestoreSpec.Tags)...)
		}
	}

	return args
}

func (a *ArchiveExecutor) setupEnvVars(ctx context.Context, archive *k8upv1.Archive) []corev1.EnvVar {
	log := controllerruntime.LoggerFrom(ctx)
	vars := executor.NewEnvVarConverter()

	if archive.Spec.RestoreSpec != nil && archive.Spec.RestoreSpec.RestoreMethod != nil {
		for key, value := range archive.Spec.RestoreMethod.S3.RestoreEnvVars() {
			// FIXME(mw): ugly, due to EnvVarConverter()
			if value.Value != "" {
				vars.SetString(key, value.Value)
			} else {
				vars.SetEnvVarSource(key, value.ValueFrom)
			}
		}
	}

	if archive.Spec.RestoreSpec != nil && archive.Spec.RestoreSpec.RestoreMethod != nil {
		if archive.Spec.RestoreSpec.RestoreMethod.Folder != nil {
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
