package restore

import (
	"fmt"

	"git.vshn.net/vshn/baas/service"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
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

func (r *RestoreRunner) Stop() error {
	// Currently noop
	return nil
}

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
