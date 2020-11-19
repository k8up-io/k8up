// observer keeps track of the currently running jobs.

package observer

import (
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	batchv1 "k8s.io/api/batch/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	Update   EventType = "update"
	Delete   EventType = "delete"
	Create   EventType = "create"
	Failed   EventType = "failed"
	Suceeded EventType = "suceeded"
	Running  EventType = "running"
	observer *Observer

	promLabels = []string{
		"namespace",
	}

	metricsFailureCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "k8up_jobs_failed_counter",
		Help: "The total number of backups that failed",
	}, promLabels)

	metricsSuccessCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "k8up_jobs_successful_counter",
		Help: "The total number of backups that went through cleanly",
	}, promLabels)

	metricsTotalCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "k8up_jobs_total",
		Help: "The total amount of all jobs run",
	}, promLabels)
)

// Observer handles the internal state of the observed batchv1.job objects.
type Observer struct {
	// events is used to send updates to the observer
	events chan ObservableJob
	// observedJobs keeps track of all the jobs that are being observed
	observedJobs map[string]ObservableJob
	mutex        sync.Mutex
	log          logr.Logger
}

// ObservableJob defines a batchv1.job that is being observed by the Observer.
type ObservableJob struct {
	Job        *batchv1.Job
	Event      EventType
	Exclusive  bool
	Repository string
	callback   func()
}

// EventType describes an event for the observer
type EventType string

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(metricsFailureCounter, metricsSuccessCounter, metricsTotalCounter)
}

// GetObserver returns the currently active observer
func GetObserver() *Observer {
	if observer == nil {
		observer = &Observer{
			events:       make(chan ObservableJob, 10),
			observedJobs: make(map[string]ObservableJob),
			log:          ctrl.Log.WithName("observer"),
			mutex:        sync.Mutex{},
		}
		go observer.run()
	}

	return observer
}

func (o *Observer) run() {
	for event := range o.events {

		jobName := o.getJobName(event.Job)

		o.mutex.Lock()

		existingJob, exists := o.observedJobs[jobName]

		// we need to add the callbacks to the new event so they won't get lost
		if exists {
			event.callback = existingJob.callback
		}

		o.log.Info("new event on observed job", "event", event.Event, "jobName", jobName)

		switch event.Event {
		case Failed:
			if event.callback != nil {
				event.callback()
			}
			incFailure(event.Job.Namespace)
		case Suceeded:
			// only report back succeeded jobs we've already seen. Will prevent
			// reporting succeeded jobs on operator restart
			if exists {
				o.log.Info("job succeeded", "jobName", jobName)
				o.observedJobs[jobName] = event
				incSuccess(event.Job.Namespace)
				if event.callback != nil {
					event.callback()
				}
			}
		case Delete:
			o.log.Info("deleting job from observer", "jobName", jobName)
			delete(o.observedJobs, jobName)
			if event.callback != nil {
				event.callback()
			}
		default:
			o.observedJobs[jobName] = event
		}

		o.mutex.Unlock()

	}
}

// GetUpdateChannel returns a chan ObservableJob. This channel allows for adding
// or updating jobs that are observed.
func (o *Observer) GetUpdateChannel() chan ObservableJob {
	return o.events
}

func (o *Observer) getJobName(job *batchv1.Job) string {
	return fmt.Sprintf("%s/%s", job.Namespace, job.Name)
}

// GetJobByName will return the requested job if it exists. JobName needs to be a string
// in the form of `jobname/namespace`
func (o *Observer) GetJobByName(jobName string) ObservableJob {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	return o.observedJobs[jobName]
}

// GetJobsByRepository will return a list of all the jobs currently being observed
// in a given repository.
func (o *Observer) GetJobsByRepository(repository string) []ObservableJob {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	repoJobs := make([]ObservableJob, 0)
	for _, job := range o.observedJobs {
		if job.Repository == repository {
			repoJobs = append(repoJobs, job)
		}
	}
	return repoJobs
}

// IsExclusiveJobRunning will return true if there's currently an exclusive job
// running on the repository.
func (o *Observer) IsExclusiveJobRunning(repository string) bool {
	listOfJobs := o.GetJobsByRepository(repository)

	if len(listOfJobs) == 0 {
		return false
	}

	for _, job := range listOfJobs {
		if job.Exclusive && job.Event == Running {
			return true
		}
	}

	return false
}

// IsAnyJobRunning will return true if there's any job running for the given
// repository.
func (o *Observer) IsAnyJobRunning(repository string) bool {
	listOfJobs := o.GetJobsByRepository(repository)

	if len(listOfJobs) == 0 {
		return false
	}

	for _, job := range listOfJobs {
		if job.Event == Running {
			return true
		}
	}

	return false
}

// RegisterCallback will register a function to the given observed job.
// The callback will be executed if the job is, successful, failed or deleted.
func (o *Observer) RegisterCallback(name string, callback func()) {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	if event, exists := o.observedJobs[name]; !exists {
		o.observedJobs[name] = ObservableJob{callback: callback}
	} else {
		event.callback = callback
		o.observedJobs[name] = event
	}
}

func incFailure(namespace string) {
	metricsFailureCounter.WithLabelValues(namespace).Inc()
	metricsTotalCounter.WithLabelValues(namespace).Inc()
}

func incSuccess(namespace string) {
	metricsSuccessCounter.WithLabelValues(namespace).Inc()
	metricsTotalCounter.WithLabelValues(namespace).Inc()
}
