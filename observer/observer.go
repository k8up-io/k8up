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

	"github.com/vshn/k8up/api/v1alpha1"
)

const (
	Update   EventType = "update"
	Delete   EventType = "delete"
	Create   EventType = "create"
	Failed   EventType = "failed"
	Suceeded EventType = "suceeded"
	Running  EventType = "running"
)

var (
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
	JobType    v1alpha1.JobType
	Event      EventType
	Exclusive  bool
	Repository string
	callbacks  []ObservableJobCallback
}

// ObservableJobCallback is invoked on certain events of the ObservableJob.
// The related ObservableJob is passed as argument.
// Check the ObservableJob.Event property if you need to know the exact event on which
// your ObservableJobCallback was invoked.
type ObservableJobCallback func(ObservableJob)

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
		o.handleEvent(event)
	}
}

func (o *Observer) handleEvent(event ObservableJob) {
	jobName := o.getJobName(event.Job)

	o.mutex.Lock()
	defer o.mutex.Unlock()

	existingJob, exists := o.observedJobs[jobName]

	// we need to add the callbacks to the new event so they won't get lost
	if exists {
		event.callbacks = existingJob.callbacks
	}

	o.log.Info("new event on observed job", "event", event.Event, "jobName", jobName)

	switch event.Event {
	case Failed:
		incFailureCounters(event.Job.Namespace)
		invokeCallbacks(event)
	case Suceeded:
		// Only report succeeded jobs we've already seen to prevent
		// reporting succeeded jobs on operator restart
		if exists {
			o.log.Info("job succeeded", "jobName", jobName)
			o.observedJobs[jobName] = event
			incSuccessCounters(event.Job.Namespace)
			invokeCallbacks(event)
		}
	case Delete:
		o.log.Info("deleting job from observer", "jobName", jobName)
		delete(o.observedJobs, jobName)
		invokeCallbacks(event)
	default:
		o.observedJobs[jobName] = event
	}
}

func invokeCallbacks(event ObservableJob) {
	for _, callback := range event.callbacks {
		callback(event)
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

// IsConcurrentJobsLimitReached checks if the limit of concurrent jobs by type (backup, check, etc)
// has been reached
func (o *Observer) IsConcurrentJobsLimitReached(jobType v1alpha1.JobType, limit int) bool {
	if limit <= 0 {
		return false
	}

	o.mutex.Lock()
	listOfJobs := o.observedJobs
	o.mutex.Unlock()

	if len(listOfJobs) == 0 {
		return false
	}
	count := 0
	for _, job := range listOfJobs {
		if job.JobType == jobType {
			count++
			if count >= limit {
				return true
			}
		}
	}
	return false
}

// RegisterCallback will register a function to the given observed job.
// The callbacks will be executed if the job is successful, failed or deleted.
func (o *Observer) RegisterCallback(name string, callback ObservableJobCallback) {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	if event, exists := o.observedJobs[name]; !exists {
		o.observedJobs[name] = ObservableJob{callbacks: []ObservableJobCallback{callback}}
	} else {
		event.callbacks = append(event.callbacks, callback)
		o.observedJobs[name] = event
	}
}

func incFailureCounters(namespace string) {
	metricsFailureCounter.WithLabelValues(namespace).Inc()
	metricsTotalCounter.WithLabelValues(namespace).Inc()
}

func incSuccessCounters(namespace string) {
	metricsSuccessCounter.WithLabelValues(namespace).Inc()
	metricsTotalCounter.WithLabelValues(namespace).Inc()
}
