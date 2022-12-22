// executor will execute all jobs in the queue by priority.

package executor

import (
	"time"

	"github.com/k8up-io/k8up/v2/operator/locker"
	"github.com/k8up-io/k8up/v2/operator/observer"
	"github.com/k8up-io/k8up/v2/operator/queue"
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

	// locker is used to query Kubernetes API about job concurrency and exclusivity.
	locker *locker.Locker
}

// StartExecutor will start the singleton instance of the Executor.
// If already started, it will no-op.
func StartExecutor(locker *locker.Locker) {
	if worker == nil {
		worker = &QueueWorker{trigger: make(chan bool), locker: locker}
		go worker.executeQueue()
	}
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

		shouldRun := false
		if job.Exclusive() {
			// TODO: discard an exclusive job if there's any other exclusive job running
			// and mark that in the status. So it is skippable.
			shouldRun = !observer.GetObserver().IsAnyJobRunning(repository)
		} else {
			reached, err := qe.locker.IsConcurrentJobsLimitReached(jobType, jobLimit)
			if err != nil {
				job.Logger().Error(err, "cannot schedule job", "type", jobType, "repository", job.GetRepository())
			}
			isExclusiveJobRunning := observer.GetObserver().IsExclusiveJobRunning(repository)
			shouldRun = !isExclusiveJobRunning && !reached

		}

		if !shouldRun {
			job.Logger().Info("skipping job due to exclusivity", "exclusive", job.Exclusive(), "repository", job.GetRepository())
			continue
		}

		err := job.Execute()
		if err != nil {
			job.Logger().Error(err, "failed to execute", "repository", repository)
		}

		// Skip the rest for this repository if we just started an exclusive job.
		if job.Exclusive() {
			return
		}
	}
}
