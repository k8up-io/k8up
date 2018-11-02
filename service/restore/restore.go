package restore

import (
	"fmt"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	"git.vshn.net/vshn/baas/service"
	"k8s.io/apimachinery/pkg/runtime"
)

// Restore holds the state of the restore handler. It implements ServiceHandler intrface.
type Restore struct {
	service.CommonObjects
	config config
}

// NewRestore returns a new restore.
func NewRestore(common service.CommonObjects) *Restore {
	return &Restore{
		CommonObjects: common,
		config:        newConfig(),
	}
}

func (r *Restore) checkObject(obj runtime.Object) (*backupv1alpha1.Restore, error) {
	backup, ok := obj.(*backupv1alpha1.Restore)
	if !ok {
		return nil, fmt.Errorf("%v is not a restore", obj.GetObjectKind())
	}
	return backup, nil
}

// Ensure satisfies ServiceHandler interface.
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

	rst = NewRestoreRunner(restoreCopy, r.CommonObjects)

	return rst.Start()
}

// Delete satisfies ServiceHandler interface.
func (r *Restore) Delete(name string) error {
	// Currently noop
	return nil
}
