package check

import (
	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	"git.vshn.net/vshn/baas/service"
	"git.vshn.net/vshn/baas/service/observe"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var test service.Runner = &checkRunner{}

type checkRunner struct {
	service.CommonObjects
	config   config
	check    *backupv1alpha1.Check
	observer *observe.Observer
}

func newCheckRunner(common service.CommonObjects, config config, check *backupv1alpha1.Check, observer *observe.Observer) *checkRunner {
	return &checkRunner{
		CommonObjects: common,
		config:        config,
		check:         check,
		observer:      observer,
	}
}

func (c *checkRunner) Start() error {
	c.Logger.Infof("New check job received %v in namespace %v", c.check.Name, c.check.Namespace)
	c.check.Status.Started = true
	c.updateCheckStatus()

	checkJob := newCheckJob(c.check, c.config)

	go c.watchState(checkJob)

	_, err := c.K8sCli.Batch().Jobs(c.check.Namespace).Create(checkJob)
	if err != nil {
		return err
	}

	return nil
}

func (c *checkRunner) SameSpec(object runtime.Object) bool { return true }
func (c *checkRunner) Stop() error                         { return nil }

func (c *checkRunner) updateCheckStatus() {
	// Just overwrite the resource
	result, err := c.BaasCLI.AppuioV1alpha1().Checks(c.check.Namespace).Get(c.check.Name, metav1.GetOptions{})
	if err != nil {
		c.Logger.Errorf("Cannot get baas object: %v", err)
	}

	result.Status = c.check.Status
	_, updateErr := c.BaasCLI.AppuioV1alpha1().Checks(c.check.Namespace).Update(result)
	if updateErr != nil {
		c.Logger.Errorf("Coud not update backup resource: %v", updateErr)
	}
}

func (c *checkRunner) watchState(job *batchv1.Job) {
	subscription, err := c.observer.GetBroker().Subscribe(job.Labels[c.config.Identifier])
	if err != nil {
		c.Logger.Errorf("Cannot watch state of backup %v", c.check.Name)
	}

	watch := observe.WatchObjects{
		Job:     job,
		JobType: observe.CheckType,
		Locker:  c.observer.GetLocker(),
		Logger:  c.Logger,
		Failedfunc: func(message observe.PodState) {
			c.check.Status.Failed = true
			c.check.Status.Finished = true
			c.updateCheckStatus()
		},
		Successfunc: func(message observe.PodState) {
			c.check.Status.Finished = true
			c.updateCheckStatus()
		},
	}

	subscription.WatchLoop(watch)
}
