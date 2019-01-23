// Package observe contains a very special "operator". It is actually a
// controller, which uses dummy pod and job CRDs to satisfy kooper. It's job is
// to observe what happens with the pods running the actual jobs.
//
// There are three components to this:
// * locker: handles the semaphores and locks
// * observer: actually observes the pods and triggers state updates
// * subscription: notifies consumers on state updates
package observe

import (
	"fmt"
	"sync"

	"github.com/vshn/k8up/config"
	"github.com/vshn/k8up/service"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var instance *Observer
var once sync.Once

// Observer will listen for jobs and pods in all namespaces that belong to
// the baas infrastructure. It'll hold the state of all these jobs and pods
// and triggers corresponding events.
type Observer struct {
	broker *Broker
	config config.Global
	locker Locker
	service.CommonObjects
}

// GetInstance returns a singleton of the observer
func GetInstance() *Observer {
	once.Do(func() {
		if instance == nil {
			instance = &Observer{
				broker: newBroker(),
				config: config.New(),
				locker: newLocker(),
			}
		}
	})
	return instance
}

// SetCommonObjects sets the common objects for this operator
func (o *Observer) SetCommonObjects(common service.CommonObjects) {
	o.CommonObjects = common
}

// Ensure will be triggered when a pod or job gets created.
func (o *Observer) Ensure(obj runtime.Object) error {
	switch obj.(type) {
	case *batchv1.Job:
		job, _ := obj.(*batchv1.Job)
		o.jobObserver(job.DeepCopy())
		return nil
	case *corev1.Pod:
		pod, _ := obj.(*corev1.Pod)
		o.podObserver(pod.DeepCopy())
		return nil
	default:
		return fmt.Errorf("Neither pod nor job: %T", obj)
	}
}

// Delete will be triggered if a pod or job gets deleted.
func (o *Observer) Delete(name string) error {
	// TODO:
	return nil
}

// podObserver checks the status of the given pod. It will then trigger
// a notification in the broker and all registered consumers will be notified
// about the change.
func (o *Observer) podObserver(pod *corev1.Pod) {

}

// jobObserver will observer the job status in the future
func (o *Observer) jobObserver(job *batchv1.Job) {
	baasJob := false
	baasID := ""
	for key, value := range job.GetLabels() {
		if fmt.Sprintf("%v=%v", key, value) == o.config.Label+"=true" {
			baasJob = true
		}
		if key == o.config.Identifier {
			baasID = value
		}
	}
	if baasJob {
		conditions := job.Status.Conditions
		message := batchv1.JobConditionType("")

		// Check for conditions
		if len(conditions) > 0 {
			latestCondition := conditions[len(conditions)-1]

			message = latestCondition.Type

			reason := latestCondition.Reason

			if message != "Complete" && reason != "BackoffLimitExceeded" {
				message = jobRunning
			}

		} else { // it's just running
			message = jobRunning
		}

		repository := service.GetRepository(&corev1.Pod{Spec: job.Spec.Template.Spec})
		err := o.broker.Notify(job.Labels[o.config.Identifier], PodState{
			State:      message,
			Repository: repository,
			BaasID:     baasID,
		})
		if err != nil {
			// TODO: here would be the point to re-register lost pods.
			return
		}
	}
}

// GetBroker returns the broker.
func (o *Observer) GetBroker() *Broker {
	return o.broker
}

// GetLocker returns the locker.
func (o *Observer) GetLocker() Locker {
	return o.locker
}
