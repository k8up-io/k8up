package restorecontroller

import (
	"context"
	"errors"
	"fmt"

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
	restorePath  = "/restore"
	_dataDirName = "k8up-dir"
)

type RestoreExecutor struct {
	executor.Generic
	restore *k8upv1.Restore
}

// NewRestoreExecutor will return a new executor for Restore jobs.
func NewRestoreExecutor(config job.Config) *RestoreExecutor {
	return &RestoreExecutor{
		Generic: executor.Generic{Config: config},
		restore: config.Obj.(*k8upv1.Restore),
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

func (r *RestoreExecutor) createRestoreObject(
	ctx context.Context,
	restore *k8upv1.Restore,
) (*batchv1.Job, error) {
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
		batchJob.Spec.Template.Spec.Volumes = append(volumes, r.attachMoreVolumes()...)
		batchJob.Spec.Template.Spec.Containers[0].VolumeMounts = append(volumeMounts, r.attachMoreVolumeMounts()...)

		args, argsErr := r.setupArgs(restore)
		batchJob.Spec.Template.Spec.Containers[0].Args = args
		return argsErr
	})

	return batchJob, err
}

func (r *RestoreExecutor) jobName() string {
	return k8upv1.RestoreType.String() + "-" + r.Obj.GetName()
}

func (r *RestoreExecutor) setupArgs(restore *k8upv1.Restore) ([]string, error) {
	args := []string{"-varDir", cfg.Config.PodVarDir, "-restore"}
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

	args = append(args, r.appendTLSOptionsArgs()...)

	return args, nil
}

func (r *RestoreExecutor) volumeConfig(restore *k8upv1.Restore) (
	[]corev1.Volume,
	[]corev1.VolumeMount,
) {
	volumes := make([]corev1.Volume, 0)
	if restore.Spec.RestoreMethod.S3 == nil {
		addVolume := corev1.Volume{
			Name: restore.Spec.RestoreMethod.Folder.ClaimName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: restore.Spec.RestoreMethod.Folder.PersistentVolumeClaimVolumeSource,
			},
		}
		volumes = append(volumes, addVolume)
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

func (r *RestoreExecutor) appendTLSOptionsArgs() []string {
	var args []string

	if r.restore.Spec.Backend != nil && r.restore.Spec.Backend.TLSOptions != nil {
		if r.restore.Spec.Backend.TLSOptions.CACert != "" {
			args = append(args, []string{"--caCert", r.restore.Spec.Backend.TLSOptions.CACert}...)
		}
		if r.restore.Spec.Backend.TLSOptions.ClientCert != "" && r.restore.Spec.Backend.TLSOptions.ClientKey != "" {
			addMoreArgs := []string{
				"--clientCert",
				r.restore.Spec.Backend.TLSOptions.ClientCert,
				"--clientKey",
				r.restore.Spec.Backend.TLSOptions.ClientKey,
			}
			args = append(args, addMoreArgs...)
		}
	}

	if r.restore.Spec.RestoreMethod != nil && r.restore.Spec.RestoreMethod.TLSOptions != nil {
		if r.restore.Spec.RestoreMethod.TLSOptions.CACert != "" {
			args = append(args, []string{"--restoreCaCert", r.restore.Spec.RestoreMethod.TLSOptions.CACert}...)
		}
		if r.restore.Spec.RestoreMethod.TLSOptions.ClientCert != "" && r.restore.Spec.RestoreMethod.TLSOptions.ClientKey != "" {
			addMoreArgs := []string{
				"--restoreClientCert",
				r.restore.Spec.RestoreMethod.TLSOptions.ClientCert,
				"--restoreClientKey",
				r.restore.Spec.RestoreMethod.TLSOptions.ClientKey,
			}
			args = append(args, addMoreArgs...)
		}
	}

	return args
}

func (r *RestoreExecutor) attachMoreVolumes() []corev1.Volume {
	ku8pVolume := corev1.Volume{
		Name:         _dataDirName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}

	if utils.ZeroLen(r.restore.Spec.Volumes) {
		return []corev1.Volume{ku8pVolume}
	}

	moreVolumes := make([]corev1.Volume, 0, len(*r.restore.Spec.Volumes)+1)
	moreVolumes = append(moreVolumes, ku8pVolume)
	for _, v := range *r.restore.Spec.Volumes {
		vol := v

		var volumeSource corev1.VolumeSource
		if vol.PersistentVolumeClaim != nil {
			volumeSource.PersistentVolumeClaim = vol.PersistentVolumeClaim
		} else if vol.Secret != nil {
			volumeSource.Secret = vol.Secret
		} else if vol.ConfigMap != nil {
			volumeSource.ConfigMap = vol.ConfigMap
		} else {
			continue
		}

		addVolume := corev1.Volume{
			Name:         vol.Name,
			VolumeSource: volumeSource,
		}
		moreVolumes = append(moreVolumes, addVolume)
	}

	return moreVolumes
}

func (r *RestoreExecutor) attachMoreVolumeMounts() []corev1.VolumeMount {
	var volumeMount []corev1.VolumeMount

	if r.restore.Spec.Backend != nil && !utils.ZeroLen(r.restore.Spec.Backend.VolumeMounts) {
		volumeMount = append(volumeMount, *r.restore.Spec.Backend.VolumeMounts...)
	}
	if r.restore.Spec.RestoreMethod != nil && !utils.ZeroLen(r.restore.Spec.RestoreMethod.VolumeMounts) {
		for _, v1 := range *r.restore.Spec.RestoreMethod.VolumeMounts {
			vm1 := v1
			var isExist bool

			for _, v2 := range volumeMount {
				vm2 := v2
				if vm1.Name == vm2.Name && vm1.MountPath == vm2.MountPath {
					isExist = true
					break
				}
			}

			if isExist {
				continue
			}

			volumeMount = append(volumeMount, vm1)
		}
	}

	addVolumeMount := corev1.VolumeMount{
		Name:      _dataDirName,
		MountPath: cfg.Config.PodVarDir,
	}
	volumeMount = append(volumeMount, addVolumeMount)

	return volumeMount
}
