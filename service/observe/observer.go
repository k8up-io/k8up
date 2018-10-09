// observe contains a very special "operator". It is actually a controller,
// which uses dummy pod and job CRDs to satisfy kooper. It's job is to observe
// what happens with the pods running the actual jobs.
package observe

import (
	"fmt"
	"sync"

	"git.vshn.net/vshn/baas/log"
	"git.vshn.net/vshn/baas/service"
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
	config config
	locker Locker
	Logger log.Logger
}

// GetInstance returns a singleton of the observer
func GetInstance(log log.Logger) *Observer {
	once.Do(func() {
		if instance == nil {
			instance = &Observer{
				broker: newBroker(),
				config: newConfig(),
				locker: newLocker(),
				Logger: log,
			}
		}
	})
	return instance
}

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

func (o *Observer) Delete(name string) error {
	return nil
}

func (o *Observer) podObserver(pod *corev1.Pod) {
	baasPod := false
	baasID := ""
	for key, value := range pod.GetLabels() {
		if fmt.Sprintf("%v=%v", key, value) == o.config.Label+"=true" {
			baasPod = true
		}
		if key == o.config.Identifier {
			baasID = value
		}
	}
	if baasPod {
		var message string
		// first check the pod phase for a general overview
		message = string(pod.Status.Phase)
		// then check if the container actually restarted and report failure
		// FIXME: After backoff limit is reached the pods get removed by the
		// Kubernete job controller. Need to check status of jobs, too.
		if len(pod.Status.ContainerStatuses) > 0 {
			state := pod.Status.ContainerStatuses[0]
			if state.RestartCount > 0 {
				message = string(corev1.PodFailed)
			}
		}
		repository := service.GetRepository(pod)
		err := o.broker.Notify(pod.Labels[o.config.Identifier], PodState{
			State:      string(message),
			Repository: repository,
			BaasID:     baasID,
		})
		if err != nil {
			o.Logger.Warningf("%v", err)
		}
	}
}

func (o *Observer) jobObserver(job *batchv1.Job) {
	// for key, value := range job.GetLabels() {
	// 	if key == "backupPod" && value == "true" {
	// 		o.Logger.Infof("job %v", job.Name)
	// 	}
	// }
}

func (o *Observer) GetBroker() *Broker {
	return instance.broker
}

func (o *Observer) GetLocker() Locker {
	return o.locker
}
