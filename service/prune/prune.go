package prune

import (
	"fmt"
	"time"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	"git.vshn.net/vshn/baas/config"
	"git.vshn.net/vshn/baas/service"
	"git.vshn.net/vshn/baas/service/observe"
	"k8s.io/apimachinery/pkg/runtime"
)

// Pruner holds the state of the pruner handler. It implements the ServiceHandler interface.
type Pruner struct {
	service.CommonObjects
	observer *observe.Observer
	config   config.Global
}

// NewPruner returns a new pruner handler
func NewPruner(common service.CommonObjects, observer *observe.Observer) *Pruner {
	return &Pruner{
		CommonObjects: common,
		observer:      observer,
		config:        config.New(),
	}
}

// Ensure satisfies service.Handler
func (p *Pruner) Ensure(obj runtime.Object) error {

	prune, err := p.checkObject(obj)
	if err != nil {
		return err
	}

	if prune.Status.Started {
		return nil
	}

	pruneCopy := prune.DeepCopy()

	pruneRunner := newPruneRunner(p.CommonObjects, p.config, pruneCopy, p.observer)

	// First increment prune semaphore, so no other jobs will start while waiting
	p.observer.GetLocker().Increment(service.GetRepository(pruneCopy), observe.PruneType)

	p.waitToRun(pruneCopy)

	err = pruneRunner.Start()

	// Decrement again as the pruneRunner.Start() also incremented the semaphore
	p.observer.GetLocker().Decrement(service.GetRepository(pruneCopy), observe.PruneType)

	return err

}

// Delete satisfies service.Handler
func (p *Pruner) Delete(name string) error {
	return nil
}

func (p *Pruner) checkObject(obj runtime.Object) (*backupv1alpha1.Prune, error) {
	prune, ok := obj.(*backupv1alpha1.Prune)
	if !ok {
		return nil, fmt.Errorf("%v is not a check", obj.GetObjectKind())
	}
	return prune, nil
}

// waitToRun for the prune is very simple; nothing else may run.
func (p *Pruner) waitToRun(pruneCopy *backupv1alpha1.Prune) {

	// TODO: this might be something to handle in the locker itself
	locker := p.observer.GetLocker()

	backend := pruneCopy.Spec.Backend

	jobs := []observe.JobType{observe.BackupType, observe.CheckType, observe.PruneType, observe.RestoreType}

	p.Logger.Infof("Prune job queued in namespace %v waiting for all other jobs to finish", pruneCopy.GetNamespace())

	waiting := true
	for waiting {
		for i := range jobs {
			if waiting || locker.IsLocked(service.GetRepository(backend), jobs[i]) {
				waiting = true
			} else {
				waiting = false
			}
		}
		time.Sleep(10 * time.Second)
	}
}
