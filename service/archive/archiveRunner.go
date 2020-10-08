package archive

import (
	"sort"

	backupv1alpha1 "github.com/vshn/k8up/apis/backup/v1alpha1"
	"github.com/vshn/k8up/config"
	"github.com/vshn/k8up/service"
	"github.com/vshn/k8up/service/observe"
	"github.com/vshn/k8up/service/schedule"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// archiveRunner contains the state of this running archive job
type archiveRunner struct {
	service.CommonObjects
	archiver *backupv1alpha1.Archive
	config   config.Global
	observer *observe.Observer
}

func newArchiveRunner(archiver *backupv1alpha1.Archive, common service.CommonObjects, observer *observe.Observer) *archiveRunner {
	return &archiveRunner{
		CommonObjects: common,
		archiver:      archiver,
		config:        config.New(),
		observer:      observer,
	}
}

// Start is part of the ServiceRunner interface.
func (a *archiveRunner) Start() error {
	a.Logger.Infof("New archive job received %v in namespace %v", a.archiver.Name, a.archiver.Namespace)
	a.archiver.Status.Started = true
	a.updateArchiveStatus()

	archiveJob := newArchiveJob(a.archiver, a.config)

	go a.watchState(archiveJob)

	_, err := a.K8sCli.BatchV1().Jobs(a.archiver.Namespace).Create(archiveJob)
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

// Stop is part of the ServiceRunner interface, it's needed for permanent
// services like the scheduler.
func (a *archiveRunner) Stop() error { return nil }

// SameSpec checks if the CRD instance changed. This is is only viable for
// permanent services like the scheduler, that may change.
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
		JobName: observe.RestoreName,
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

	a.removeOldestArchives(a.getScheduledCRDsInNameSpace(), a.archiver.Spec.KeepJobs)

}

func (a *archiveRunner) getScheduledCRDsInNameSpace() *backupv1alpha1.ArchiveList {
	opts := metav1.ListOptions{
		LabelSelector: schedule.ScheduledLabelFilter(),
	}
	archives, err := a.BaasCLI.AppuioV1alpha1().Archives(a.archiver.Namespace).List(opts)
	if err != nil {
		a.Logger.Errorf("%v", err)
		return nil
	}

	return archives
}

func (a *archiveRunner) cleanupArchive(archive *backupv1alpha1.Archive) error {
	a.Logger.Infof("Cleanup archive %v", archive.Name)
	option := metav1.DeletePropagationForeground
	return a.BaasCLI.AppuioV1alpha1().Archives(archive.Namespace).Delete(archive.Name, &metav1.DeleteOptions{
		PropagationPolicy: &option,
	})
}

func (a *archiveRunner) removeOldestArchives(archives *backupv1alpha1.ArchiveList, maxJobs int) {
	if maxJobs == 0 {
		maxJobs = a.config.GlobalKeepJobs
	}
	numToDelete := len(archives.Items) - maxJobs
	if numToDelete <= 0 {
		return
	}

	a.Logger.Infof("Cleaning up %d/%d jobs", numToDelete, len(archives.Items))

	sort.Sort(archives)
	for i := 0; i < numToDelete; i++ {
		a.Logger.Infof("Removing job %v limit reached", archives.Items[i].Name)
		a.cleanupArchive(&archives.Items[i])
	}
}
