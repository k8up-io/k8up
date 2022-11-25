package executor

import (
	"context"
	"errors"
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/job"
	"github.com/k8up-io/k8up/v2/operator/observer"
)

const restorePath = "/restore"

type RestoreExecutor struct {
	generic
}

// NewRestoreExecutor will return a new executor for Restore jobs.
func NewRestoreExecutor(config job.Config) *RestoreExecutor {
	return &RestoreExecutor{
		generic: generic{config},
	}
}

// GetConcurrencyLimit returns the concurrent jobs limit
func (r *RestoreExecutor) GetConcurrencyLimit() int {
	return cfg.Config.GlobalConcurrentRestoreJobsLimit
}

// Execute creates the actual batch.job on the k8s api.
func (r *RestoreExecutor) Execute() error {
	restore, ok := r.Obj.(*k8upv1.Restore)
	if !ok {
		return errors.New("object is not a restore")
	}

	if restore.GetStatus().Started {
		r.RegisterJobSucceededConditionCallback() // ensure that completed jobs can complete backups between operator restarts.
		return nil
	}

	return r.startRestore(restore)

	//return nil
}

func (r *RestoreExecutor) startRestore(restore *k8upv1.Restore) error {
	node := NewCITANode(r.CTX, r.Client, restore.Namespace, restore.Spec.Node)
	stopped, err := node.Stop()
	if err != nil {
		return err
	}
	if !stopped {
		return nil
	}

	//r.registerRestoreCallback(restore)
	r.registerCITANodeCallback(restore)
	r.RegisterJobSucceededConditionCallback()

	restoreJob, err := r.buildRestoreObject(restore)
	if err != nil {
		r.Log.Error(err, "unable to build restore object")
		r.SetConditionFalseWithMessage(k8upv1.ConditionReady, k8upv1.ReasonCreationFailed, "unable to build restore object: %v", err)
		return err
	}

	if err := r.Client.Create(r.CTX, restoreJob); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			r.Log.Error(err, "could not create job")
			r.SetConditionFalseWithMessage(k8upv1.ConditionReady, k8upv1.ReasonCreationFailed, "could not create job: %v", err)
			return err
		}
	}

	r.SetStarted("the job '%v/%v' was created", restoreJob.Namespace, restoreJob.Name)
	return nil
}

func (r *RestoreExecutor) registerRestoreCallback(restore *k8upv1.Restore) {
	name := r.GetJobNamespacedName()
	observer.GetObserver().RegisterCallback(name.String(), func(_ observer.ObservableJob) {
		r.cleanupOldRestores(name, restore)
	})
}

func (r *RestoreExecutor) registerCITANodeCallback(restore *k8upv1.Restore) {
	name := r.GetJobNamespacedName()
	observer.GetObserver().RegisterCallback(name.String(), func(_ observer.ObservableJob) {
		//b.StopPreBackupDeployments()
		//b.cleanupOldBackups(name)
		r.startCITANode(r.CTX, r.Client, restore.Namespace, restore.Spec.Node)
	})
}

func (r *RestoreExecutor) startCITANode(ctx context.Context, client client.Client, namespace, name string) {
	NewCITANode(ctx, client, namespace, name).Start()
}

func (r *RestoreExecutor) cleanupOldRestores(name types.NamespacedName, restore *k8upv1.Restore) {
	r.cleanupOldResources(&k8upv1.RestoreList{}, name, restore)

}

func (r *RestoreExecutor) buildRestoreObject(restore *k8upv1.Restore) (*batchv1.Job, error) {
	j, err := job.GenerateGenericJob(restore, r.Config)
	if err != nil {
		return nil, err
	}

	j.GetLabels()[job.K8upExclusive] = strconv.FormatBool(r.Exclusive())

	j.Spec.Template.Spec.Containers[0].Env = r.setupEnvVars(restore)
	restore.Spec.AppendEnvFromToContainer(&j.Spec.Template.Spec.Containers[0])

	volumes, volumeMounts := r.volumeConfig(restore)
	j.Spec.Template.Spec.Volumes = volumes
	j.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts

	args, err := r.args(restore)
	if err != nil {
		return nil, err
	}
	j.Spec.Template.Spec.Containers[0].Args = args

	return j, nil
}

func (r *RestoreExecutor) args(restore *k8upv1.Restore) ([]string, error) {
	args := []string{"-restore"}

	if len(restore.Spec.Tags) > 0 {
		args = append(args, BuildTagArgs(restore.Spec.Tags)...)
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

func (r *RestoreExecutor) setupEnvVars(restore *k8upv1.Restore) []corev1.EnvVar {
	vars := NewEnvVarConverter()

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

	err := vars.Merge(DefaultEnv(r.Obj.GetMetaObject().GetNamespace()))
	if err != nil {
		r.Log.Error(err, "error while merging the environment variables", "name", r.Obj.GetMetaObject().GetName(), "namespace", r.Obj.GetMetaObject().GetNamespace())
	}

	return vars.Convert()
}
