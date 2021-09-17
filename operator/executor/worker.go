// executor will execute all jobs in the queue by priority.

package executor

import (
	"time"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/k8up-io/k8up/operator/observer"
	"github.com/k8up-io/k8up/operator/queue"
)

var (
	worker *QueueWorker
)

// QueueWorker is the object responsible for iterating the job queue and triggering
// the execution of the jobs.
type QueueWorker struct {
	// trigger is used to trigger an execution loop. So we don't need to poll
	// the whole time.
	trigger chan bool
}

// GetExecutor will return the singleton instance for the executor.
func GetExecutor() *QueueWorker {
	if worker == nil {
		worker = &QueueWorker{trigger: make(chan bool)}
		go worker.executeQueue()
	}
	return worker
}

func (qe *QueueWorker) executeQueue() {
	for {
		time.Sleep(1 * time.Second)

		repositories := queue.GetExecQueue().GetRepositories()
		for _, repository := range repositories {
			qe.loopRepositoryJobs(repository)
		}
	}
}

func (qe *QueueWorker) loopRepositoryJobs(repository string) {
	for !queue.GetExecQueue().IsEmpty(repository) {
		job := queue.GetExecQueue().Get(repository)
		jobType := job.GetJobType()
		jobLimit := job.GetConcurrencyLimit()

		var shouldRun bool
		if job.Exclusive() {
			// TODO: discard an exclusive job if there's any other exclusive job running
			// and mark that in the status. So it is skippable.
			shouldRun = !observer.GetObserver().IsAnyJobRunning(repository)
		} else {
			shouldRun = !observer.GetObserver().IsExclusiveJobRunning(repository) &&
				!observer.GetObserver().IsConcurrentJobsLimitReached(jobType, jobLimit)
		}

		if !shouldRun {
			job.Logger().Info("skipping job due to exclusivity", "exclusive", job.Exclusive(), "repository", job.GetRepository())
			continue
		}

		err := job.Execute()
		if err != nil {
			if !errors.IsAlreadyExists(err) {
				job.Logger().Error(err, "cannot create job", "repository", repository)
			}
		}

		// Skip the rest for this repository if we just started an exclusive job.
		if job.Exclusive() {
			return
		}
	}
}
