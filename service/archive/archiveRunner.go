package archive

import (
	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	"git.vshn.net/vshn/baas/service"
	"git.vshn.net/vshn/baas/service/observe"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// archiveRunner contains the state of this running archive job
type archiveRunner struct {
	service.CommonObjects
	archiver *backupv1alpha1.Archive
	config   config
	observer *observe.Observer
}

func newArchiveRunner(archiver *backupv1alpha1.Archive, common service.CommonObjects, observer *observe.Observer) *archiveRunner {
	return &archiveRunner{
		CommonObjects: common,
		archiver:      archiver,
		config:        newConfig(),
		observer:      observer,
	}
}

func (a *archiveRunner) Start() error {
	a.Logger.Infof("New archive job received %v in namespace %v", a.archiver.Name, a.archiver.Namespace)
	a.archiver.Status.Started = true
	a.updateArchiveStatus()

	archiveJob := newArchiveJob(a.archiver, a.config)

	go a.watchState(archiveJob)

	_, err := a.K8sCli.Batch().Jobs(a.archiver.Namespace).Create(archiveJob)
	if err != nil {
		return err
	}

	return nil
}

func (a *archiveRunner) updateArchiveStatus() error {
	// Just overwrite the resource
	result, err := a.BaasCLI.AppuioV1alpha1().Archives(a.archiver.Namespace).Get(a.archiver.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	result.Status = a.archiver.Status
	_, updateErr := a.BaasCLI.AppuioV1alpha1().Archives(a.archiver.Namespace).Update(result)
	if updateErr != nil {
		a.Logger.Errorf("Could not update archive resource: %v", updateErr)
	}
	return nil
}

func (a *archiveRunner) Stop() error                         { return nil }
func (a *archiveRunner) SameSpec(object runtime.Object) bool { return true }

// TODO: make exported so running jobs can be picked up if the operator does
// a failover
func (a *archiveRunner) watchState(archiveJob *batchv1.Job) {
	subscription, err := a.observer.GetBroker().Subscribe(archiveJob.Labels[a.config.Identifier])
	if err != nil {
		a.Logger.Errorf("Cannot watch state of %v: %v", archiveJob.GetName(), err)
	}

	watch := observe.WatchObjects{
		Logger:  a.Logger,
		Job:     archiveJob,
		JobType: observe.RestoreType,
		Locker:  a.observer.GetLocker(),
		Successfunc: func(message observe.PodState) {
			a.archiver.Status.Finished = true
			a.updateArchiveStatus()
		},
		Failedfunc: func(message observe.PodState) {
			a.archiver.Status.Failed = true
			a.updateArchiveStatus()
		},
	}

	subscription.WatchLoop(watch)

	defer func() {
		a.observer.GetBroker().Unsubscribe(archiveJob.Labels[a.config.Identifier], subscription)
	}()

}
