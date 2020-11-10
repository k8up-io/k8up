package executor

import (
	"errors"
	"strconv"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/constants"
	"github.com/vshn/k8up/job"
	"github.com/vshn/k8up/observer"
)

const restorePath = "/restore"

type RestoreExecutor struct {
	generic
}

func NewRestoreExecutor(config job.Config) *RestoreExecutor {
	return &RestoreExecutor{
		generic: generic{config},
	}
}

func (r *RestoreExecutor) Execute() error {
	restore, ok := r.Obj.(*k8upv1alpha1.Restore)
	if !ok {
		return errors.New("object is not a prune")
	}

	if restore.GetK8upStatus().Started {
		return nil
	}

	r.startRestore(restore)

	return nil
}

func (r *RestoreExecutor) Exclusive() bool {
	return true
}

func (r *RestoreExecutor) startRestore(restore *k8upv1alpha1.Restore) {
	name := types.NamespacedName{Namespace: r.Obj.GetMetaObject().GetNamespace(), Name: r.Obj.GetMetaObject().GetName()}
	r.setRestoreCallback(name, restore)

	j, err := r.buildRestoreObject(restore)
	if err != nil {
		r.Log.Error(err, "unable to build restore object")
		return
	}

	if err := r.Client.Create(r.CTX, j); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			r.Log.Error(err, "could not create job")
			return
		}
	}

	if err := r.setJobStatusStarted(); err != nil {
		r.Log.Error(err, "could not update prune status")
	}
}

func (r *RestoreExecutor) setJobStatusStarted() error {
	original := r.Obj
	new := r.Obj.GetRuntimeObject().DeepCopyObject().(*k8upv1alpha1.Restore)
	new.GetK8upStatus().Started = true

	return r.Client.Status().Patch(r.CTX, new, client.MergeFrom(original.GetRuntimeObject()))
}

func (r *RestoreExecutor) setRestoreCallback(name types.NamespacedName, restore *k8upv1alpha1.Restore) {
	observer.GetObserver().RegisterCallback(name.String(), func() {
		r.cleanupOldRestores(name, restore)
	})
}

func (r *RestoreExecutor) cleanupOldRestores(name types.NamespacedName, restore *k8upv1alpha1.Restore) {
	restoreList := &k8upv1alpha1.RestoreList{}
	err := r.Client.List(r.CTX, restoreList, &client.ListOptions{
		Namespace: name.Namespace,
	})
	if err != nil {
		r.Log.Error(err, "could not list objects to cleanup old restores", "Namespace", name.Namespace)
	}

	jobs := make(jobObjectList, len(restoreList.Items))
	for i, restore := range restoreList.Items {
		jobs[i] = &restore
	}

	var keepJobs *int = restore.Spec.KeepJobs

	err = cleanOldObjects(jobs, getKeepJobs(keepJobs), r.Config)
	if err != nil {
		r.Log.Error(err, "could not delete old restores", "namespace", name.Namespace)
	}
}

func (r *RestoreExecutor) buildRestoreObject(restore *k8upv1alpha1.Restore) (*batchv1.Job, error) {
	j, err := job.GetGenericJob(restore, r.Config)
	if err != nil {
		return nil, err
	}

	j.GetLabels()[job.K8upExclusive] = strconv.FormatBool(r.Exclusive())

	j.Spec.Template.Spec.Containers[0].Env = r.setupEnvVars(restore)

	volumes, volumeMounts := r.volumeConfig(restore)
	j.Spec.Template.Spec.Volumes = volumes
	j.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts

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

	methodDefined := false
	if restore.Spec.RestoreMethod.Folder != nil {
		args = append(args, "-restoreType", "folder")
		methodDefined = true
	}

	if !methodDefined && restore.Spec.RestoreMethod.S3 != nil {
		args = append(args, "-restoreType", "s3")
		methodDefined = true
	}

	j.Spec.Template.Spec.Containers[0].Args = args

	return j, nil
}

func (r *RestoreExecutor) volumeConfig(restore *k8upv1alpha1.Restore) ([]corev1.Volume, []corev1.VolumeMount) {
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
			MountPath: restorePath,
		}
		mounts = append(mounts, tmpMount)
	}

	return volumes, mounts
}

func (r *RestoreExecutor) setupEnvVars(restore *k8upv1alpha1.Restore) []corev1.EnvVar {
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
		vars.SetString(constants.ResticRepositoryEnvName, restore.Spec.Backend.String())
	}

	err := vars.Merge(DefaultEnv(r.Obj.GetMetaObject().GetNamespace()))
	if err != nil {
		r.Log.Error(err, "error while merging the environment variables", "name", r.Obj.GetMetaObject().GetName(), "namespace", r.Obj.GetMetaObject().GetNamespace())
	}

	return vars.Convert()
}
