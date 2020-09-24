package restore

import (
	"fmt"
	"sort"

	backupv1alpha1 "github.com/vshn/k8up/apis/backup/v1alpha1"
	"github.com/vshn/k8up/service"
	"github.com/vshn/k8up/service/observe"
	"github.com/vshn/k8up/service/schedule"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type restoreRunner struct {
	restore *backupv1alpha1.Restore
	service.CommonObjects
	config   config
	observer *observe.Observer
}

// newRestoreRunner returns a new restore runner
func newRestoreRunner(restore *backupv1alpha1.Restore, common service.CommonObjects, observer *observe.Observer) *restoreRunner {
	return &restoreRunner{
		restore:       restore,
		CommonObjects: common,
		config:        newConfig(),
		observer:      observer,
	}
}

// Start is part of the ServiceRunner interface.
func (r *restoreRunner) Start() error {

	r.Logger.Infof("Received restore job %v in namespace %v", r.restore.Name, r.restore.Namespace)

	if r.restore.Spec.Backend == nil {
		r.Logger.Infof("Restore %v doesn't have a backend configured, skipping...", r.restore.Name)
		return nil
	}

	restoreJob := newRestoreJob(r.restore, r.config)

	go r.watchState(restoreJob)

	_, err := r.K8sCli.BatchV1().Jobs(r.restore.Namespace).Create(restoreJob)
	if err != nil {
		return err
	}

	r.restore.Status.Started = true

	err = r.updateStatus()
	if err != nil {
		return fmt.Errorf("Cannot get baas object: %v", err)
	}

	return nil
}

// Stop is part of the ServiceRunner interface, it's needed for permanent
// services like the scheduler.
func (r *restoreRunner) Stop() error {
	// Currently noop
	return nil
}

// SameSpec checks if the CRD instance changed. This is is only viable for
// permanent services like the scheduler, that may change.
func (r *restoreRunner) SameSpec(object runtime.Object) bool {
	return false
}

func (r *restoreRunner) updateStatus() error {
	// Just overwrite the resource
	result, err := r.BaasCLI.AppuioV1alpha1().Restores(r.restore.Namespace).Get(r.restore.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	result.Status = r.restore.Status
	_, updateErr := r.BaasCLI.AppuioV1alpha1().Restores(r.restore.Namespace).Update(result)
	if updateErr != nil {
		r.Logger.Errorf("Coud not update restore resource: %v", updateErr)
	}
	return nil
}

func (r *restoreRunner) getScheduledCRDsInNameSpace() *backupv1alpha1.RestoreList {
	opts := metav1.ListOptions{
		LabelSelector: schedule.ScheduledLabelFilter(),
	}
	restores, err := r.BaasCLI.AppuioV1alpha1().Restores(r.restore.Namespace).List(opts)
	if err != nil {
		r.Logger.Errorf("%v", err)
		return nil
	}

	return restores
}

func (r *restoreRunner) cleanupRestore(restore *backupv1alpha1.Restore) error {
	r.Logger.Infof("Cleanup restore %v", restore.Name)
	option := metav1.DeletePropagationForeground
	return r.BaasCLI.AppuioV1alpha1().Restores(restore.Namespace).Delete(restore.Name, &metav1.DeleteOptions{
		PropagationPolicy: &option,
	})
}

func (r *restoreRunner) removeOldestRestores(restores *backupv1alpha1.RestoreList, maxJobs int) {
	if maxJobs == 0 {
		maxJobs = r.config.GlobalKeepJobs
	}
	numToDelete := len(restores.Items) - maxJobs
	if numToDelete <= 0 {
		return
	}

	r.Logger.Infof("Cleaning up %d/%d jobs", numToDelete, len(restores.Items))

	sort.Sort(restores)
	for i := 0; i < numToDelete; i++ {
		r.Logger.Infof("Removing job %v limit reached", restores.Items[i].Name)
		r.cleanupRestore(&restores.Items[i])
	}
}

func (r *restoreRunner) watchState(restoreJob *batchv1.Job) {
	subscription, err := r.observer.GetBroker().Subscribe(restoreJob.Labels[r.config.Identifier])
	if err != nil {
		r.Logger.Errorf("Cannot watch state of backup %v", r.restore.Name)
	}

	watch := observe.WatchObjects{
		Job:     restoreJob,
		JobName: observe.RestoreName,
		Locker:  r.observer.GetLocker(),
		Logger:  r.Logger,
		Failedfunc: func(message observe.PodState) {
			r.restore.Status.Failed = true
			r.restore.Status.Finished = true
			r.updateStatus()
		},
		Successfunc: func(message observe.PodState) {
			r.restore.Status.Finished = true
			r.updateStatus()
		},
	}

	subscription.WatchLoop(watch)

	r.removeOldestRestores(r.getScheduledCRDsInNameSpace(), r.restore.Spec.KeepJobs)
}
