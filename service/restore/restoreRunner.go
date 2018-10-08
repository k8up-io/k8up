package restore

import (
	"fmt"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	baas8scli "git.vshn.net/vshn/baas/client/k8s/clientset/versioned"
	"git.vshn.net/vshn/baas/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

type RestoreRunner struct {
	restore *backupv1alpha1.Restore
	k8sCLI  kubernetes.Interface
	baasCLI baas8scli.Interface
	log     log.Logger
	config  config
}

// NewRestoreRunner returns a new restore runner
func NewRestoreRunner(restore *backupv1alpha1.Restore, k8sCLI kubernetes.Interface, baasCLI baas8scli.Interface, log log.Logger) *RestoreRunner {
	return &RestoreRunner{
		restore: restore,
		k8sCLI:  k8sCLI,
		baasCLI: baasCLI,
		log:     log,
		config:  newConfig(),
	}
}

func (r *RestoreRunner) Start() error {

	r.log.Infof("Received restore job %v in namespace %v", r.restore.Name, r.restore.Namespace)

	if r.restore.Spec.Backend == nil {
		r.log.Infof("Restore %v doesn't have a backend configured, skipping...", r.restore.Name)
		return nil
	}

	volumes := []corev1.Volume{}
	if r.restore.Spec.RestoreMethod.S3 == nil {
		volumes = append(volumes,
			corev1.Volume{
				Name: r.restore.Spec.RestoreMethod.Folder.ClaimName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: r.restore.Spec.RestoreMethod.Folder.PersistentVolumeClaimVolumeSource,
				},
			})
	}

	restoreJob := newRestoreJob(r.restore, volumes, r.config)

	_, err := r.k8sCLI.Batch().Jobs(r.restore.Namespace).Create(restoreJob)
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
	result, err := r.baasCLI.AppuioV1alpha1().Restores(r.restore.Namespace).Get(r.restore.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	result.Status = r.restore.Status
	_, updateErr := r.baasCLI.AppuioV1alpha1().Restores(r.restore.Namespace).Update(result)
	if updateErr != nil {
		r.log.Errorf("Coud not update restore resource: %v", updateErr)
	}
	return nil
}
