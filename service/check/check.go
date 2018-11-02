package check

import (
	"fmt"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	"git.vshn.net/vshn/baas/config"
	"git.vshn.net/vshn/baas/service"
	"git.vshn.net/vshn/baas/service/observe"
	"k8s.io/apimachinery/pkg/runtime"
)

// Check holds the state of the check handler. It implements ServiceHandler.
type Check struct {
	service.CommonObjects
	observer *observe.Observer
	config   config.Global
}

// NewCheck returns a new check handler.
func NewCheck(common service.CommonObjects, observer *observe.Observer) *Check {
	return &Check{
		CommonObjects: common,
		observer:      observer,
		config:        config.New(),
	}
}

// Ensure is part of the ServiceHandler interface
func (c *Check) Ensure(obj runtime.Object) error {
	check, err := c.checkObject(obj)
	if err != nil {
		return err
	}

	if check.Status.Started {
		return nil
	}

	checkCopy := check.DeepCopy()

	checkRunner := newCheckRunner(c.CommonObjects, c.config, checkCopy, c.observer)

	return checkRunner.Start()
}

// Delete is part of the ServiceHandler interface. It's needed for permanent
// services, like the scheduler.
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
