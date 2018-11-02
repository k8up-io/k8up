package restore

import (
	"fmt"
	"sort"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	"git.vshn.net/vshn/baas/service"
	"git.vshn.net/vshn/baas/service/schedule"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type RestoreRunner struct {
	restore *backupv1alpha1.Restore
	service.CommonObjects
	config config
}

// NewRestoreRunner returns a new restore runner
func NewRestoreRunner(restore *backupv1alpha1.Restore, common service.CommonObjects) *RestoreRunner {
	return &RestoreRunner{
		restore:       restore,
		CommonObjects: common,
		config:        newConfig(),
	}
}

// Start is part of the ServiceRunner interface.
func (r *RestoreRunner) Start() error {

	r.Logger.Infof("Received restore job %v in namespace %v", r.restore.Name, r.restore.Namespace)

	if r.restore.Spec.Backend == nil {
		r.Logger.Infof("Restore %v doesn't have a backend configured, skipping...", r.restore.Name)
		return nil
	}

	restoreJob := newRestoreJob(r.restore, r.config)

	_, err := r.K8sCli.Batch().Jobs(r.restore.Namespace).Create(restoreJob)
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
func (r *RestoreRunner) Stop() error {
	// Currently noop
	return nil
}

// SameSpec checks if the CRD instance changed. This is is only viable for
// permanent services like the scheduler, that may change.
func (r *RestoreRunner) SameSpec(object runtime.Object) bool {
	return false
}

func (r *RestoreRunner) updateStatus() error {
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

func (r *RestoreRunner) getScheduledCRDsInNameSpace() []backupv1alpha1.Restore {
	opts := metav1.ListOptions{
		LabelSelector: schedule.ScheduledLabelFilter(),
	}
	restores, err := r.BaasCLI.AppuioV1alpha1().Restores(r.restore.Namespace).List(opts)
	if err != nil {
		r.Logger.Errorf("%v", err)
		return nil
	}

	return restores.Items
}

func (r *RestoreRunner) cleanupRestore(restore *backupv1alpha1.Restore) error {
	r.Logger.Infof("Cleanup restore %v", restore.Name)
	option := metav1.DeletePropagationForeground
	return r.BaasCLI.AppuioV1alpha1().Restores(restore.Namespace).Delete(restore.Name, &metav1.DeleteOptions{
		PropagationPolicy: &option,
	})
}

func (r *RestoreRunner) removeOldestPrunes(restores []backupv1alpha1.Restore, maxJobs int) {
	if maxJobs == 0 {
		maxJobs = r.config.GlobalKeepJobs
	}
	numToDelete := len(restores) - maxJobs
	if numToDelete <= 0 {
		return
	}

	r.Logger.Infof("Cleaning up %d/%d jobs", numToDelete, len(restores))

	sort.Sort(byCreationTime(restores))
	for i := 0; i < numToDelete; i++ {
		r.Logger.Infof("Removing job %v limit reached", restores[i].Name)
		r.cleanupRestore(&restores[i])
	}
}
