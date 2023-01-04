package locker

import (
	"context"
	"fmt"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/job"
	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Locker struct {
	Kube client.Client
}

// jobListFn is a function that by default lists job with the Kubernetes Client, but allows unit testing without the client.
var jobListFn = func(locker *Locker, listOptions ...client.ListOption) (batchv1.JobList, error) {
	// list all jobs that match labels.
	// controller-runtime by default caches GET and LIST requests, so performance-wise all the results should be in the cache already.
	list := batchv1.JobList{}
	err := locker.Kube.List(context.Background(), &list, listOptions...)
	return list, err
}

// IsConcurrentJobsLimitReached returns true if the cluster-wide amount of jobs by type is greater or equal the given jobLimit.
// It does this by listing and counting all batchv1.Jobs that satisfy label constraints and have active Pods.
// Suspended jobs (if any) are not counted.
// The intention is to avoid overloading the cluster if many jobs are spawned at the same time.
// Returns an error if the listing failed.
func (l *Locker) IsConcurrentJobsLimitReached(jobType k8upv1.JobType, jobLimit int) (bool, error) {
	if jobLimit <= 0 {
		// no limit set
		return false, nil
	}
	list, err := jobListFn(l, client.MatchingLabels{
		job.K8uplabel:        "true",
		k8upv1.LabelK8upType: jobType.String(),
	})
	if err != nil {
		return false, fmt.Errorf("cannot determine job concurrency: %w", err)
	}
	count := 0
	for _, batchJob := range list.Items {
		count += int(batchJob.Status.Active)
		if count >= jobLimit {
			return true, nil
		}
	}

	return false, nil
}
