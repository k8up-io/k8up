// observer keeps track of the currently running jobs.

package observer

import (
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	Update   EventType = "update"
	Delete   EventType = "delete"
	Create   EventType = "create"
	Failed   EventType = "failed"
	Suceeded EventType = "suceeded"
	Running  EventType = "running"
	observer *Observer
)

type Observer struct {
	// events is used to send updates to the observer
	events chan ObservableJob
	// observedJobs keeps track of all the jobs that are being observed
	observedJobs map[string]ObservableJob
	mutex        sync.Mutex
	log          logr.Logger
}

type ObservableJob struct {
	Job       *batchv1.Job
	Event     EventType
	Exclusive bool
}

// Event describes an event for the observer
type EventType string

// GetObserver returns the currently active observer
func GetObserver() *Observer {
	if observer != nil {
		observer = &Observer{
			events:       make(chan ObservableJob, 10),
			observedJobs: make(map[string]ObservableJob),
			log:          ctrl.Log.WithName("observer"),
		}
		observer.run()
	}

	return observer
}

func (o *Observer) run() {
	for event := range o.events {

		jobName := o.getJobName(event.Job)

		switch event.Event {
		case Update:
			o.log.Info("updating job in observer", "jobName", jobName)
			o.observedJobs[jobName] = event
		case Delete:
			o.log.Info("deleting job from observer", "jobName", jobName)
			delete(o.observedJobs, jobName)
		case Create:
			o.log.Info("creating job in observer", "jobName", jobName)
			o.observedJobs[jobName] = event
		}
	}
}

func (o *Observer) GetUpdateChannel() chan ObservableJob {
	return o.events
}

func (o *Observer) getJobName(job *batchv1.Job) string {
	return fmt.Sprintf("%s/%s", job.Name, job.Namespace)
}

// GetJob will return the requested job if it exists. JobName needs to be a string
// in the form of `jobname/namespace`
func (o *Observer) GetJob(jobName string) ObservableJob {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	return observer.observedJobs[jobName]
}
