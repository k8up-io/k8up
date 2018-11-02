package archive

import (
	"fmt"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	"git.vshn.net/vshn/baas/config"
	"git.vshn.net/vshn/baas/service"
	"git.vshn.net/vshn/baas/service/observe"
	"k8s.io/apimachinery/pkg/runtime"
)

// Archive holds the state of the archive handler. It satisfies the ServiceHandler Interface.
type Archive struct {
	service.CommonObjects
	config   config.Global
	observer *observe.Observer
}

// NewArchive returns a new archive handler.
func NewArchive(common service.CommonObjects, observer *observe.Observer) *Archive {
	return &Archive{
		CommonObjects: common,
		config:        config.New(),
		observer:      observer,
	}
}

// Ensure is part of the ServiceHandler interface
func (a *Archive) Ensure(obj runtime.Object) error {
	archiver, err := a.checkObject(obj)
	if err != nil {
		return err
	}

	if archiver.Status.Started {
		return nil
	}

	archiverCopy := archiver.DeepCopy()

	newArchiver := newArchiveRunner(archiverCopy, a.CommonObjects, a.observer)
	return newArchiver.Start()
}

// Delete is part of the ServiceHandler interface. It's needed for permanent
// services, like the scheduler.
func (a *Archive) Delete(name string) error {
	return nil
}

// checkObject checks if the received object is indeed the correct type.
func (a *Archive) checkObject(obj runtime.Object) (*backupv1alpha1.Archive, error) {
	archive, ok := obj.(*backupv1alpha1.Archive)
	if !ok {
		return nil, fmt.Errorf("%v is not an archive", obj.GetObjectKind())
	}
	return archive, nil
}
