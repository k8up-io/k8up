package cleaner

import (
	"context"
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
)

// ObjectCleaner cleans old, finished job objects.
type ObjectCleaner struct {
	Client client.Client
	Limits GetJobsHistoryLimiter
}

// GetJobsHistoryLimiter provides the limits on how many jobs to clean.
type GetJobsHistoryLimiter interface {
	GetSuccessfulJobsHistoryLimit() *int
	GetFailedJobsHistoryLimit() *int
}

// NewObjectCleaner creates a new ObjectCleaner instance.
func NewObjectCleaner(clt client.Client, Limits GetJobsHistoryLimiter) *ObjectCleaner {
	return &ObjectCleaner{Client: clt, Limits: Limits}
}

// CleanOldObjects iterates over the given list and deletes them with the oldest object first until the amount returned from GetJobsHistoryLimiter remain.
// The function aborts early on errors.
// Returns the amount of deleted objects and possible errors.
func (c *ObjectCleaner) CleanOldObjects(ctx context.Context, jobObjects k8upv1.JobObjectList) (int, error) {
	maxSuccessfulObjects, maxFailedObjects := historyLimits(c.Limits)
	_, failedJobs, successfulJobs := groupByStatus(jobObjects)

	successDel, err := c.cleanOldObjects(ctx, successfulJobs, maxSuccessfulObjects)
	if err != nil {
		return successDel, err
	}
	failedDel, err := c.cleanOldObjects(ctx, failedJobs, maxFailedObjects)
	return successDel + failedDel, err
}

// cleanOldObjects deletes from the given objects until maxObjects remain.
// Returns the amount of deleted objects and possible errors.
func (c *ObjectCleaner) cleanOldObjects(ctx context.Context, jobObjects k8upv1.JobObjectList, maxObjects int) (int, error) {
	numToDelete := len(jobObjects) - maxObjects
	if numToDelete <= 0 {
		return 0, nil
	}

	log := controllerruntime.LoggerFrom(ctx)
	log.Info("Cleaning old jobs", "have", len(jobObjects), "max", maxObjects, "deleting", numToDelete)

	sort.Sort(jobObjects)
	for i := 0; i < numToDelete; i++ {
		if err := c.deleteJob(ctx, jobObjects[i]); err != nil {
			return i, fmt.Errorf("could not delete old %s: %w", jobObjects[i].GetType(), err)
		}
	}

	return numToDelete, nil
}

func (c *ObjectCleaner) deleteJob(ctx context.Context, job k8upv1.JobObject) error {
	log := controllerruntime.LoggerFrom(ctx)
	log.V(1).Info("Cleaning old job", "namespace", job.GetNamespace(), "name", job.GetName())
	option := metav1.DeletePropagationForeground
	err := c.Client.Delete(ctx, job, &client.DeleteOptions{
		PropagationPolicy: &option,
	})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}

func historyLimits(l GetJobsHistoryLimiter) (successful, failed int) {
	successful = cfg.Config.GetGlobalSuccessfulJobsHistoryLimit()
	failed = cfg.Config.GetGlobalFailedJobsHistoryLimit()
	if l != nil {
		successful = getOrDefault(l.GetSuccessfulJobsHistoryLimit(), successful)
		failed = getOrDefault(l.GetFailedJobsHistoryLimit(), failed)
	}
	return
}

func getOrDefault(n *int, defaultN int) int {
	if n == nil {
		return defaultN
	}
	if *n < 0 {
		return 0
	}
	return *n
}

// groupByStatus groups jobs by the running state
func groupByStatus(jobs []k8upv1.JobObject) (running []k8upv1.JobObject, failed []k8upv1.JobObject, successful []k8upv1.JobObject) {
	running = make([]k8upv1.JobObject, 0, len(jobs))
	successful = make([]k8upv1.JobObject, 0, len(jobs))
	failed = make([]k8upv1.JobObject, 0, len(jobs))
	for _, job := range jobs {
		if job.GetStatus().HasSucceeded() {
			successful = append(successful, job)
			continue
		}
		if job.GetStatus().HasFailed() {
			failed = append(failed, job)
			continue
		}
		running = append(running, job)
	}
	return
}
