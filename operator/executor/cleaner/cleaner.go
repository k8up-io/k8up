package cleaner

import (
	"context"
	"fmt"
	"sort"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/job"
)

// ObjectCleaner cleans old, finished job objects.
type ObjectCleaner struct {
	Client client.Client
	Limits GetJobsHistoryLimiter
	Log    logr.Logger
}

// GetJobsHistoryLimiter provides the limits on how many jobs to clean.
type GetJobsHistoryLimiter interface {
	GetSuccessfulJobsHistoryLimit() *int
	GetFailedJobsHistoryLimit() *int
}

// CleanOldObjects iterates over the given list and deletes them with the oldest object first until the amount returned from GetJobsHistoryLimiter remain.
// The function aborts early on errors.
// Returns the amount of deleted objects and possible errors.
func (c *ObjectCleaner) CleanOldObjects(ctx context.Context, jobObjects k8upv1.JobObjectList) (int, error) {
	maxSuccessfulObjects, maxFailedObjects := historyLimits(c.Limits)
	_, failedJobs, successfulJobs := job.GroupByStatus(jobObjects)

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

	c.Log.Info("cleaning old jobs", "have", len(jobObjects), "want", maxObjects, "deleting", numToDelete)

	if numToDelete <= 0 {
		return 0, nil
	}

	sort.Sort(jobObjects)
	for i := 0; i < numToDelete; i++ {
		if err := c.deleteJob(ctx, jobObjects[i]); err != nil {
			c.Log.Error(err, "could not delete old job", "namespace", jobObjects[i].GetMetaObject().GetNamespace())
			return i, fmt.Errorf("could not delete old %s: %w", jobObjects[i].GetType(), err)
		}
	}

	return numToDelete, nil
}

func (c *ObjectCleaner) deleteJob(ctx context.Context, job k8upv1.JobObject) error {
	name := job.GetMetaObject().GetName()
	ns := job.GetMetaObject().GetNamespace()
	c.Log.Info("cleaning old job", "namespace", ns, "name", name)
	option := metav1.DeletePropagationForeground
	err := c.Client.Delete(ctx, job.GetRuntimeObject().(client.Object), &client.DeleteOptions{
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
