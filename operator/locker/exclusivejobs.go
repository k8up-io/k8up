package locker

import (
	"fmt"

	"github.com/k8up-io/k8up/v2/operator/job"
	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IsAnyJobRunningForRepository will return true if there's any job running for the given repository.
func (l *Locker) IsAnyJobRunningForRepository(repository string) (bool, error) {
	listOfJobs, err := l.GetJobsByRepository(repository, false)
	if err != nil {
		return false, fmt.Errorf("cannot filter jobs for repository: %w", err)
	}
	if len(listOfJobs) == 0 {
		return false, nil
	}

	for _, batchJob := range listOfJobs {
		if batchJob.Status.Active >= 0 {
			return true, nil
		}
	}
	return false, nil
}

// IsExclusiveJobRunning will return true if there's currently an exclusive job running on the repository.
func (l *Locker) IsExclusiveJobRunning(repository string) (bool, error) {
	listOfJobs, err := l.GetJobsByRepository(repository, true)
	if err != nil {
		return false, fmt.Errorf("cannot filter jobs for repository: %w", err)
	}

	if len(listOfJobs) == 0 {
		return false, nil
	}

	for _, batchJob := range listOfJobs {
		if batchJob.Status.Active >= 0 && batchJob.Labels[job.K8upExclusive] == "true" {
			return true, nil
		}
	}

	return false, nil
}

// GetJobsByRepository will return a list of all the jobs currently existing for the given repository.
func (l *Locker) GetJobsByRepository(repository string, exclusive bool) ([]batchv1.Job, error) {
	matchLabels := client.MatchingLabels{
		job.K8uplabel: "true",
	}
	if exclusive {
		matchLabels[job.K8upExclusive] = "true"
	}
	list, err := jobListFn(l, matchLabels)
	if err != nil {
		return []batchv1.Job{}, fmt.Errorf("cannot get list of jobs: %w", err)
	}
	filtered := make([]batchv1.Job, 0)

	for _, batchJob := range list.Items {
		if batchJob.Annotations[job.K8upRepositoryAnnotation] == repository {
			filtered = append(filtered, batchJob)
		}
	}
	return filtered, nil
}
