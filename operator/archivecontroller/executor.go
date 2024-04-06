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
	archivePath  = "/archive"
	_dataDirName = "k8up-dir"
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
		mutateErr := job.MutateBatchJob(batchJob, a.archive, a.Config)
		if mutateErr != nil {
			return mutateErr
		}

		batchJob.Spec.Template.Spec.Containers[0].Env = a.setupEnvVars(ctx, a.archive)
		a.archive.Spec.AppendEnvFromToContainer(&batchJob.Spec.Template.Spec.Containers[0])
		batchJob.Spec.Template.Spec.Containers[0].VolumeMounts = a.attachMoreVolumeMounts()
		batchJob.Spec.Template.Spec.Volumes = a.attachMoreVolumes()

		args, argsErr := a.setupArgs()
		batchJob.Spec.Template.Spec.Containers[0].Args = args

		return argsErr
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

func (a *ArchiveExecutor) setupArgs() ([]string, error) {
	args := []string{"-varDir", cfg.Config.PodVarDir, "-archive", "-restoreType", "s3"}
	if a.archive.Spec.RestoreSpec != nil && len(a.archive.Spec.RestoreSpec.Tags) > 0 {
		args = append(args, executor.BuildTagArgs(a.archive.Spec.RestoreSpec.Tags)...)
	}
	args = append(args, a.appendOptionsArgs()...)

	return args, nil
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

func (a *ArchiveExecutor) appendOptionsArgs() []string {
	var args []string

	if a.archive.Spec.Backend != nil && a.archive.Spec.Backend.Options != nil {
		if a.archive.Spec.Backend.Options.CACert != "" {
			args = append(args, []string{"-caCert", a.archive.Spec.Backend.Options.CACert}...)
		}
		if a.archive.Spec.Backend.Options.ClientCert != "" && a.archive.Spec.Backend.Options.ClientKey != "" {
			addMoreArgs := []string{
				"-clientCert",
				a.archive.Spec.Backend.Options.ClientCert,
				"-clientKey",
				a.archive.Spec.Backend.Options.ClientKey,
			}
			args = append(args, addMoreArgs...)
		}
	}

	if a.archive.Spec.RestoreSpec != nil && a.archive.Spec.RestoreMethod.Options != nil {
		if a.archive.Spec.RestoreMethod.Options.CACert != "" {
			args = append(args, []string{"-restoreCaCert", a.archive.Spec.RestoreMethod.Options.CACert}...)
		}
		if a.archive.Spec.RestoreMethod.Options.ClientCert != "" && a.archive.Spec.RestoreMethod.Options.ClientKey != "" {
			addMoreArgs := []string{
				"-restoreClientCert",
				a.archive.Spec.RestoreMethod.Options.ClientCert,
				"-restoreClientKey",
				a.archive.Spec.RestoreMethod.Options.ClientKey,
			}
			args = append(args, addMoreArgs...)
		}
	}

	return args
}

func (a *ArchiveExecutor) attachMoreVolumes() []corev1.Volume {
	ku8pVolume := corev1.Volume{
		Name:         _dataDirName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}

	if utils.ZeroLen(a.archive.Spec.Volumes) {
		return []corev1.Volume{ku8pVolume}
	}

	moreVolumes := make([]corev1.Volume, 0, len(*a.archive.Spec.Volumes)+1)
	moreVolumes = append(moreVolumes, ku8pVolume)
	for _, v := range *a.archive.Spec.Volumes {
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

func (a *ArchiveExecutor) attachMoreVolumeMounts() []corev1.VolumeMount {
	var volumeMount []corev1.VolumeMount

	if a.archive.Spec.Backend != nil && !utils.ZeroLen(a.archive.Spec.Backend.VolumeMounts) {
		volumeMount = append(volumeMount, *a.archive.Spec.Backend.VolumeMounts...)
	}
	if a.archive.Spec.RestoreMethod != nil && !utils.ZeroLen(a.archive.Spec.RestoreMethod.VolumeMounts) {
		for _, v1 := range *a.archive.Spec.RestoreMethod.VolumeMounts {
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
