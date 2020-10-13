// executor will execute all jobs in the queue by priority.

package executor

import "github.com/vshn/k8up/queue"

func executeQueue() {
	for {
		for _, repository := range queue.GetRepositories() {
			// TODO: add locker check before executing
			// TODO: check if the job is running/completed -> observer?
			job := queue.GetExecQueue().Get(repository)
			err := job.Execute()
			if err != nil {
				job.Logger().Error(err, "cannot create job", "repository", repository)
			}
		}
	}
}
