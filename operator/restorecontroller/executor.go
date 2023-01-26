package restorecontroller

import (
	"context"
	"errors"
	"fmt"

	"github.com/k8up-io/k8up/v2/operator/executor"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/job"
)

const restorePath = "/restore"

type RestoreExecutor struct {
	executor.Generic
}

// NewRestoreExecutor will return a new executor for Restore jobs.
func NewRestoreExecutor(config job.Config) *RestoreExecutor {
	return &RestoreExecutor{
		Generic: executor.Generic{Config: config},
	}
}

// GetConcurrencyLimit returns the concurrent jobs limit
func (r *RestoreExecutor) GetConcurrencyLimit() int {
	return cfg.Config.GlobalConcurrentRestoreJobsLimit
}

// Execute creates the actual batch.job on the k8s api.
func (r *RestoreExecutor) Execute(ctx context.Context) error {
	log := controllerruntime.LoggerFrom(ctx)
	restore, ok := r.Obj.(*k8upv1.Restore)
	if !ok {
		return errors.New("object is not a restore")
	}

	restoreJob, err := r.createRestoreObject(ctx, restore)
	if err != nil {
		log.Error(err, "unable to create or update restore object")
		r.SetConditionFalseWithMessage(ctx, k8upv1.ConditionReady, k8upv1.ReasonCreationFailed, "unable to create restore object: %v", err)
		return nil
	}

	r.SetStarted(ctx, "the job '%v/%v' was created", restoreJob.Namespace, restoreJob.Name)

	return nil
}

func (r *RestoreExecutor) cleanupOldRestores(ctx context.Context, restore *k8upv1.Restore) {
	r.CleanupOldResources(ctx, &k8upv1.RestoreList{}, restore.Namespace, restore)
}

func (r *RestoreExecutor) createRestoreObject(ctx context.Context, restore *k8upv1.Restore) (*batchv1.Job, error) {
	batchJob := &batchv1.Job{}
	batchJob.Name = r.jobName()
	batchJob.Namespace = restore.Namespace
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, batchJob, func() error {
		mutateErr := job.MutateBatchJob(batchJob, restore, r.Config)
		if mutateErr != nil {
			return mutateErr
		}
		batchJob.Labels[job.K8upExclusive] = "true"
		batchJob.Spec.Template.Spec.Containers[0].Env = r.setupEnvVars(ctx, restore)
		restore.Spec.AppendEnvFromToContainer(&batchJob.Spec.Template.Spec.Containers[0])

		volumes, volumeMounts := r.volumeConfig(restore)
		batchJob.Spec.Template.Spec.Volumes = volumes
		batchJob.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts

		args, argsErr := r.args(restore)
		batchJob.Spec.Template.Spec.Containers[0].Args = args
		return argsErr
	})

	return batchJob, err
}

func (r *RestoreExecutor) jobName() string {
	return k8upv1.RestoreType.String() + "-" + r.Obj.GetName()
}

func (r *RestoreExecutor) args(restore *k8upv1.Restore) ([]string, error) {
	args := []string{"-restore"}

	if len(restore.Spec.Tags) > 0 {
		args = append(args, executor.BuildTagArgs(restore.Spec.Tags)...)
	}

	if restore.Spec.RestoreFilter != "" {
		args = append(args, "-restoreFilter", restore.Spec.RestoreFilter)
	}

	if restore.Spec.Snapshot != "" {
		args = append(args, "-restoreSnap", restore.Spec.Snapshot)
	}

	switch {
	case restore.Spec.RestoreMethod.Folder != nil:
		args = append(args, "-restoreType", "folder")
	case restore.Spec.RestoreMethod.S3 != nil:
		args = append(args, "-restoreType", "s3")
	default:
		return nil, fmt.Errorf("undefined restore method (-restoreType) on '%v/%v'", restore.Namespace, restore.Name)
	}
	return args, nil
}

func (r *RestoreExecutor) volumeConfig(restore *k8upv1.Restore) ([]corev1.Volume, []corev1.VolumeMount) {
	volumes := make([]corev1.Volume, 0)
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
			MountPath: restorePath,
		}
		mounts = append(mounts, tmpMount)
	}

	return volumes, mounts
}

func (r *RestoreExecutor) setupEnvVars(ctx context.Context, restore *k8upv1.Restore) []corev1.EnvVar {
	log := controllerruntime.LoggerFrom(ctx)
	vars := executor.NewEnvVarConverter()

	if restore.Spec.RestoreMethod.S3 != nil {
		for key, value := range restore.Spec.RestoreMethod.S3.RestoreEnvVars() {
			// FIXME(mw): ugly, due to EnvVarConverter()
			if value.Value != "" {
				vars.SetString(key, value.Value)
			} else {
				vars.SetEnvVarSource(key, value.ValueFrom)
			}
		}
	}
	if restore.Spec.RestoreMethod.Folder != nil {
		vars.SetString("RESTORE_DIR", restorePath)
	}
	if restore.Spec.Backend != nil {
		for key, value := range restore.Spec.Backend.GetCredentialEnv() {
			vars.SetEnvVarSource(key, value)
		}
		vars.SetString(cfg.ResticRepositoryEnvName, restore.Spec.Backend.String())
	}

	err := vars.Merge(executor.DefaultEnv(r.Obj.GetNamespace()))
	if err != nil {
		log.Error(err, "error while merging the environment variables", "name", r.Obj.GetName(), "namespace", r.Obj.GetNamespace())
	}

	return vars.Convert()
}
