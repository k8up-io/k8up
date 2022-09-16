package executor

import (
	stderrors "errors"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/job"
	"github.com/k8up-io/k8up/v2/operator/observer"
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
	archive, ok := a.Obj.(*k8upv1.Archive)
	if !ok {
		return stderrors.New("object is not a archive")
	}

	if a.Obj.GetStatus().Started {
		a.RegisterJobSucceededConditionCallback() // ensure that completed jobs can complete backups between operator restarts.
		return nil
	}

	jobObj, err := job.GenerateGenericJob(archive, a.Config)
	if err != nil {
		a.SetConditionFalseWithMessage(k8upv1.ConditionReady, k8upv1.ReasonCreationFailed, "could not get job template: %v", err)
		return err
	}
	jobObj.GetLabels()[job.K8upExclusive] = "true"

	a.startArchive(jobObj, archive)

	return nil
}

func (a *ArchiveExecutor) startArchive(archiveJob *batchv1.Job, archive *k8upv1.Archive) {
	a.registerArchiveCallback(archive)
	a.RegisterJobSucceededConditionCallback()

	archiveJob.Spec.Template.Spec.Containers[0].Env = a.setupEnvVars(archive)
	archive.Spec.AppendEnvFromToContainer(&archiveJob.Spec.Template.Spec.Containers[0])
	archiveJob.Spec.Template.Spec.Containers[0].Args = a.setupArgs(archive)

	err := a.Client.Create(a.CTX, archiveJob)
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			a.Log.Error(err, "could not create job")
			a.SetConditionFalseWithMessage(k8upv1.ConditionReady,
				k8upv1.ReasonCreationFailed,
				"could not create archive job '%v/%v': %v",
				archiveJob.Namespace, archiveJob.Name, err)
			return
		}
	}

	a.SetStarted("the job '%v/%v' was created", archiveJob.Namespace, archiveJob.Name)
}

func (a *ArchiveExecutor) registerArchiveCallback(archive *k8upv1.Archive) {
	name := a.GetJobNamespacedName()
	observer.GetObserver().RegisterCallback(name.String(), func(_ observer.ObservableJob) {
		a.cleanupOldArchives(name, archive)
	})
}

func (a *ArchiveExecutor) setupArgs(archive *k8upv1.Archive) []string {
	args := []string{"-archive", "-restoreType", "s3"}

	if archive.Spec.RestoreSpec != nil {
		if len(archive.Spec.RestoreSpec.Tags) > 0 {
			args = append(args, BuildTagArgs(archive.Spec.RestoreSpec.Tags)...)
		}
	}

	return args
}

func (a *ArchiveExecutor) setupEnvVars(archive *k8upv1.Archive) []corev1.EnvVar {
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

func (a *ArchiveExecutor) cleanupOldArchives(name types.NamespacedName, archive *k8upv1.Archive) {
	a.cleanupOldResources(&k8upv1.ArchiveList{}, name, archive)
}
