package executor

import (
	stderrors "errors"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/cfg"
	"github.com/vshn/k8up/job"
	"github.com/vshn/k8up/observer"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const archivePath = "/archive"

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

// GetConcurrencyLimit returns the concurrent jobs limit
func (a *ArchiveExecutor) GetConcurrencyLimit() int {
	return cfg.Config.GlobalConcurrentArchiveJobsLimit
}

// Execute creates the actual batch.job on the k8s api.
func (a *ArchiveExecutor) Execute() error {
	archive, ok := a.Obj.(*k8upv1alpha1.Archive)
	if !ok {
		return stderrors.New("object is not a archive")
	}

	if a.Obj.GetStatus().Started {
		return nil
	}

	jobObj, err := job.GetGenericJob(archive, a.Config)
	if err != nil {
		a.SetConditionFalse(ConditionJobCreated, "could not get job template: %v", err)
		return err
	}
	jobObj.GetLabels()[job.K8upExclusive] = "true"

	a.startArchive(jobObj, archive)

	return nil
}

func (a *ArchiveExecutor) startArchive(archiveJob *batchv1.Job, archive *k8upv1alpha1.Archive) {
	a.registerArchiveCallback(archive)
	a.RegisterJobSucceededConditionCallback()

	archiveJob.Spec.Template.Spec.Containers[0].Env = a.setupEnvVars(archive)
	archiveJob.Spec.Template.Spec.Containers[0].Args = a.setupArgs(archive)

	err := a.Client.Create(a.CTX, archiveJob)
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			a.Log.Error(err, "could not create job")
			a.SetConditionFalse(ConditionJobCreated, "could not create job: %v", err)
			return
		}
	}

	a.SetStarted(ConditionJobCreated, "the job '%v/%v' was created", archiveJob.Namespace, archiveJob.Name)
}

func (a *ArchiveExecutor) registerArchiveCallback(archive *k8upv1alpha1.Archive) {
	name := a.GetJobNamespacedName()
	observer.GetObserver().RegisterCallback(name.String(), func(_ observer.ObservableJob) {
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
		a.SetConditionFalse(ConditionCleanupSucceeded, "cloud not list objects to cleanup old archives: %v", err)
		return
	}

	jobs := make(jobObjectList, len(archiveList.Items))
	for i, aItem := range archiveList.Items {
		jobs[i] = &aItem
	}

	var keepJobs = archive.Spec.KeepJobs

	err = cleanOldObjects(jobs, getKeepJobs(keepJobs), a.Config)
	if err != nil {
		a.Log.Error(err, "could not delete old archives", "namespace", name.Namespace)
		a.SetConditionFalse(ConditionCleanupSucceeded, "could not delete old archives: %v", err)
	}

	a.SetConditionTrue(ConditionCleanupSucceeded)
}
