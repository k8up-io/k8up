package executor

import (
	stderrors "errors"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/job"
	"github.com/vshn/k8up/observer"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ArchiveExecutor will execute the batch.job for archive.
type ArchiveExecutor struct {
	generic
}

// NewArchiveExecutor will return a new executor for archive jobs.
func NewArchiveExecutor(config job.Config) *ArchiveExecutor {
	return &ArchiveExecutor{
		generic: generic{config},
	}
}

// Execute creates the actual batch.job on the k8s api.
func (a *ArchiveExecutor) Execute() error {
	archive, ok := a.Obj.(*k8upv1alpha1.Archive)
	if !ok {
		return stderrors.New("object is not a archive")
	}

	if a.Obj.GetK8upStatus().Started {
		return nil
	}

	jobObj, err := job.GetGenericJob(archive, a.Config)
	jobObj.GetLabels()[job.K8upExclusive] = "true"
	if err != nil {
		return err
	}

	a.startArchive(jobObj, archive)

	return nil
}

// Exclusive should return true for jobs that can't run while other jobs run.
func (*ArchiveExecutor) Exclusive() bool {
	return true
}

func (a *ArchiveExecutor) startArchive(job *batchv1.Job, archive *k8upv1alpha1.Archive) {
	name := types.NamespacedName{Namespace: a.Obj.GetMetaObject().GetNamespace(), Name: a.Obj.GetMetaObject().GetName()}
	a.setArchiveCallback(name, archive)

	job.Spec.Template.Spec.Containers[0].Env = a.setupEnvVars(archive)
	job.Spec.Template.Spec.Containers[0].Args = a.setupArgs(archive)

	err := a.Client.Create(a.CTX, job)
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			a.Log.Error(err, "could not create job")
			return
		}
	}

	a.Obj.GetK8upStatus().Started = true

	err = a.Client.Status().Update(a.CTX, a.Obj.GetRuntimeObject().DeepCopyObject())
	if err != nil {
		a.Config.Log.Error(err, "could not update archive status")
	}
}

func (a *ArchiveExecutor) setArchiveCallback(name types.NamespacedName, archive *k8upv1alpha1.Archive) {
	observer.GetObserver().RegisterCallback(name.String(), func() {
		a.cleanupOldArchives(name, archive)
	})
}

func (a *ArchiveExecutor) setupArgs(archive *k8upv1alpha1.Archive) []string {
	args := []string{"-archive", "-restoreType", "s3"}

	if archive.Spec.RestoreSpec != nil {
		if len(archive.Spec.RestoreSpec.Tags) > 0 {
			args = append(args, BuildTagArgs(archive.Spec.RestoreSpec.Tags)...)
		}
	}

	return args
}

func (a *ArchiveExecutor) setupEnvVars(archive *k8upv1alpha1.Archive) []corev1.EnvVar {
	vars := NewEnvVarConverter()

	if archive.Spec.RestoreSpec != nil && archive.Spec.RestoreSpec.RestoreMethod != nil {
		if archive.Spec.RestoreSpec.RestoreMethod.S3 != nil {
			rVars := archive.Spec.RestoreSpec.RestoreMethod.S3.EnvVars(archive.Spec.RestoreSpec.Backend.GetCredentialEnv())
			for key, value := range rVars {
				vars.SetEnvVarSource(key, value)
			}
		}
	}

	err := vars.Merge(DefaultEnv(a.Obj.GetMetaObject().GetNamespace()))
	if err != nil {
		a.Log.Error(err, "error while merging the environment variables", "name", a.Obj.GetMetaObject().GetName(), "namespace", a.Obj.GetMetaObject().GetNamespace())
	}

	return vars.Convert()
}

func (a *ArchiveExecutor) cleanupOldArchives(name types.NamespacedName, archive *k8upv1alpha1.Archive) {
	archiveList := &k8upv1alpha1.ArchiveList{}
	err := a.Client.List(a.CTX, archiveList, &client.ListOptions{
		Namespace: name.Namespace,
	})
	if err != nil {
		a.Log.Error(err, "could not list objects to cleanup old archives", "Namespace", name.Namespace)
	}

	jobs := make(jobObjectList, len(archiveList.Items))
	for i, aItem := range archiveList.Items {
		jobs[i] = &aItem
	}

	var keepJobs *int = archive.Spec.KeepJobs

	err = cleanOldObjects(jobs, getKeepJobs(keepJobs), a.Config)
	if err != nil {
		a.Log.Error(err, "could not delete old archives", "namespace", name.Namespace)
	}
}
