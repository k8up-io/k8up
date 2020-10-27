// executor will execute all jobs in the queue by priority.

package executor

import (
	"time"

	"github.com/vshn/k8up/observer"
	"github.com/vshn/k8up/queue"
	"k8s.io/apimachinery/pkg/api/errors"
)

var (
	worker *QueueWorker
)

type QueueWorker struct {
	// trigger is used to trigger an execution loop. So we don't need to poll
	// the whole time.
	trigger chan bool
}

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

		var shouldRun bool
		if job.Exclusive() {
			// TODO: discard an exclusive job if there's any other exclusive job running
			// and mark that in the status. So it is skippable.
			shouldRun = !observer.GetObserver().IsAnyJobRunning(repository)
		} else {
			shouldRun = !observer.GetObserver().IsExclusiveJobRunning(repository)
		}

		if shouldRun {
			err := job.Execute()
			if err != nil {
				if !errors.IsAlreadyExists(err) {
					job.Logger().Error(err, "cannot create job", "repository", repository)
				}
			}
		}
	}
}
