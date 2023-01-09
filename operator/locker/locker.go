package locker

import (
	"context"
	"fmt"
	"sync"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/job"
	batchv1 "k8s.io/api/batch/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var repositoryLockers = sync.Map{}

// Locker is used to synchronize different controllers that operate on the same Restic repository.
type Locker interface {
	// TryRun attempts to run a non-exclusive Restic job.
	// The given runnable is executed if:
	//   - concurrency limit for the same type of job is not yet reached
	//   - and no other exclusive job is running (batch Job with Active pods > 0)
	//
	// The given runnable runs synchronously within a locked mutex, so it should be as short as possible as it blocks other runnables.
	// It returns false with nil error if the preconditions aren't met, false with an error if preconditions cannot be determined, or true with the error returned by the runnable.
	TryRun(ctx context.Context, config job.Config, concurrencyLimit int, runnable func(ctx context.Context) error) (bool, error)

	// TryRunExclusively attempts to run an exclusive Restic job.
	// The given runnable is executed if there are no other jobs running (batch Job with Active pods == 0).
	//
	// The given runnable runs synchronously within a locked mutex, so it should be as short as possible as it blocks other runnables.
	// It returns false with nil error if the precondition isn't met, false with an error if precondition cannot be determined, or true with the error returned by the runnable.
	TryRunExclusively(ctx context.Context, runnable func(ctx context.Context) error) (bool, error)
}

type lockerImpl struct {
	kube       client.Client
	m          sync.Mutex
	repository string
}

// GetForRepository returns a Locker scoped for the given repository.
func GetForRepository(clt client.Client, repository string) Locker {
	locker, _ := repositoryLockers.LoadOrStore(repository, &lockerImpl{kube: clt, repository: repository})
	return locker.(*lockerImpl)
}

func (l *lockerImpl) TryRun(ctx context.Context, config job.Config, concurrencyLimit int, runnable func(ctx context.Context) error) (bool, error) {
	log := controllerruntime.LoggerFrom(ctx).WithName("locker").WithValues("repository", l.repository)
	l.m.Lock()
	defer func() {
		l.m.Unlock()
		log.V(1).Info("Unlocked")
	}()
	log.V(1).Info("Locked")

	runningJobs, err := l.fetchRunningJobs(ctx)
	if err != nil {
		return false, fmt.Errorf("cannot determine if jobs are running: %w", err)
	}

	reached := isConcurrencyLimitReached(runningJobs, config.Obj.GetType(), concurrencyLimit)
	shouldRun := !isExclusiveJobRunning(runningJobs) && !reached

	if shouldRun {
		return true, runnable(ctx)
	}
	return false, nil
}

func (l *lockerImpl) TryRunExclusively(ctx context.Context, runnable func(ctx context.Context) error) (bool, error) {
	log := controllerruntime.LoggerFrom(ctx).WithName("locker").WithValues("repository", l.repository)
	l.m.Lock()
	defer func() {
		l.m.Unlock()
		log.V(1).Info("Unlocked")
	}()
	log.V(1).Info("Locked")

	running, err := l.fetchRunningJobs(ctx)
	if err != nil {
		return false, err
	}

	if len(running) == 0 {
		return true, runnable(ctx)
	}
	return false, nil
}

// isExclusiveJobRunning will return true if there's currently an exclusive job running.
func isExclusiveJobRunning(jobs []batchv1.Job) bool {
	for _, batchJob := range jobs {
		if batchJob.Status.Active > 0 && batchJob.Labels[job.K8upExclusive] == "true" {
			return true
		}
	}
	return false
}

// isConcurrencyLimitReached returns true if the cluster-wide amount of jobs by type is greater or equal the given jobLimit.
// It does this by counting all batchv1.Jobs that satisfy label constraints and have active Pods.
// Suspended jobs (if any) are not counted.
// The intention is to avoid overloading the cluster if many jobs are spawned at the same time.
// A jobLimit of 0 returns false.
func isConcurrencyLimitReached(jobs []batchv1.Job, jobType k8upv1.JobType, jobLimit int) bool {
	if jobLimit == 0 {
		return false
	}
	count := 0
	for _, batchJob := range jobs {
		if batchJob.Labels[k8upv1.LabelK8upType] != jobType.String() {
			continue
		}
		count += int(batchJob.Status.Active)
		if count >= jobLimit {
			return true
		}
	}
	return false
}

// jobListFn is a function that by default lists job with the Kubernetes Client, but allows unit testing without the client.
var jobListFn = func(locker *lockerImpl, ctx context.Context, listOptions ...client.ListOption) (batchv1.JobList, error) {
	// list all jobs that match labels.
	// controller-runtime by default caches GET and LIST requests, so performance-wise all the results should be in the cache already.
	list := batchv1.JobList{}
	err := locker.kube.List(ctx, &list, listOptions...)
	return list, err
}

func (l *lockerImpl) fetchRunningJobs(ctx context.Context) ([]batchv1.Job, error) {
	matchLabels := client.MatchingLabels{
		job.K8uplabel:              "true",
		k8upv1.LabelRepositoryHash: job.Sha256Hash(l.repository),
	}
	list, err := jobListFn(l, ctx, matchLabels)
	if err != nil {
		return []batchv1.Job{}, fmt.Errorf("cannot get list of jobs: %w", err)
	}
	filtered := make([]batchv1.Job, 0)
	for _, batchJob := range list.Items {
		if batchJob.Status.Active > 0 {
			filtered = append(filtered, batchJob)
		}
	}
	return filtered, nil
}
