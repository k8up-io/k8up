package prune

import (
	"fmt"

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

	return pruneRunner.Start()

}

// Delete satisfies service.Handler
func (p *Pruner) Delete(name string) error {
	return nil
}

func (p *Pruner) checkObject(obj runtime.Object) (*backupv1alpha1.Prune, error) {
	prune, ok := obj.(*backupv1alpha1.Prune)
	if !ok {
		return nil, fmt.Errorf("%v is not a prune", obj.GetObjectKind())
	}
	return prune, nil
}
