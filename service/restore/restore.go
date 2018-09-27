package restore

import (
	"fmt"

	"git.vshn.net/vshn/baas/service"

	"git.vshn.net/vshn/baas/log"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	baas8scli "git.vshn.net/vshn/baas/client/k8s/clientset/versioned"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

// Restore will ensure that the backups are running accordingly.
type Restore struct {
	k8sCli  kubernetes.Interface
	baasCLI baas8scli.Interface
	logger  log.Logger
	config  config
}

// NewRestore returns a new restore.
func NewRestore(k8sCli kubernetes.Interface, baasCLI baas8scli.Interface, logger log.Logger) *Restore {
	return &Restore{
		k8sCli:  k8sCli,
		baasCLI: baasCLI,
		logger:  logger,
		config:  newConfig(),
	}
}

func (r *Restore) checkObject(obj runtime.Object) (*backupv1alpha1.Restore, error) {
	backup, ok := obj.(*backupv1alpha1.Restore)
	if !ok {
		return nil, fmt.Errorf("%v is not a restore", obj.GetObjectKind())
	}
	return backup, nil
}

// Ensure satisfies CRDEnsurer interface.
func (r *Restore) Ensure(obj runtime.Object) error {
	restore, err := r.checkObject(obj)
	if err != nil {
		return err
	}
	var rst service.Runner

	if restore.Status.Started {
		// ignore restores that have the started status set
		return nil
	}

	// Create a restore.
	restoreCopy := restore.DeepCopy()

	restoreCopy.GlobalOverrides.RegisteredBackend = service.MergeGlobalBackendConfig(restore.Spec.Backend, r.config.GlobalConfig)

	rst = NewRestoreRunner(restoreCopy, r.k8sCli, r.baasCLI, r.logger)

	return rst.Start()
}

// Delete satisfies CRDEnsurer interface.
func (r *Restore) Delete(name string) error {
	// Currently noop
	return nil
}
