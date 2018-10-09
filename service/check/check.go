package check

import (
	"fmt"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	"git.vshn.net/vshn/baas/service"
	"git.vshn.net/vshn/baas/service/observe"
	"k8s.io/apimachinery/pkg/runtime"
)

type Check struct {
	service.CommonObjects
	observer *observe.Observer
	config   config
}

func NewCheck(common service.CommonObjects, observer *observe.Observer) *Check {
	return &Check{
		CommonObjects: common,
		observer:      observer,
		config:        newConfig(),
	}
}

func (c *Check) Ensure(obj runtime.Object) error {
	check, err := c.checkObject(obj)
	if err != nil {
		return err
	}

	if check.Status.Started {
		return nil
	}

	checkCopy := check.DeepCopy()

	checkCopy.GlobalOverrides = &backupv1alpha1.GlobalOverrides{}
	checkCopy.GlobalOverrides.RegisteredBackend = service.MergeGlobalBackendConfig(checkCopy.Spec.Backend, c.config.GlobalConfig)

	checkRunner := newCheckRunner(c.CommonObjects, c.config, checkCopy, c.observer)

	return checkRunner.Start()
}

func (c *Check) Delete(name string) error {
	return nil
}

func (c *Check) checkObject(obj runtime.Object) (*backupv1alpha1.Check, error) {
	check, ok := obj.(*backupv1alpha1.Check)
	if !ok {
		return nil, fmt.Errorf("%v is not a check", obj.GetObjectKind())
	}
	return check, nil
}
