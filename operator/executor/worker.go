// executor will execute all jobs in the queue by priority.

package executor

import (
	"time"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/locker"
	"k8s.io/apimachinery/pkg/api/errors"

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
			isExclusiveJobRunning := observer.GetObserver().IsExclusiveJobRunning(repository)
			switch jobType {
			case k8upv1.ArchiveType:
				fallthrough
			case k8upv1.RestoreType:
				fallthrough
			case k8upv1.CheckType:
				fallthrough
			case k8upv1.PruneType:
				fallthrough
			case k8upv1.BackupType:
				// only the backup type is currently implemented without the observer
				reached, err := qe.locker.IsConcurrentJobsLimitReached(jobType, jobLimit)
				if err != nil {
					job.Logger().Error(err, "cannot schedule job", "type", jobType, "repository", job.GetRepository())
				}
				shouldRun = !isExclusiveJobRunning && !reached
			default:
				shouldRun = !isExclusiveJobRunning &&
					!observer.GetObserver().IsConcurrentJobsLimitReached(jobType, jobLimit)
			}
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
