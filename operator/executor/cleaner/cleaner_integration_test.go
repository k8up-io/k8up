//go:build integration

package cleaner

import (
	"testing"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/envtest"

	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CleanerTestSuite struct {
	envtest.Suite
}

func Test_Cleaner(t *testing.T) {
	suite.Run(t, new(CleanerTestSuite))
}

func (ts *CleanerTestSuite) TestCleanup() {
	ts.withJobs()
	ts.runCleanup()
	ts.assertJobsDeleted()
}

func newLimiter(maxFailed, maxSuccessful int) limiter {
	return limiter{maxFailed: &maxFailed, maxSuccessful: &maxSuccessful}
}

type limiter struct {
	maxFailed, maxSuccessful *int
}

func (l limiter) GetSuccessfulJobsHistoryLimit() *int {
	return l.maxSuccessful
}

func (l limiter) GetFailedJobsHistoryLimit() *int {
	return l.maxFailed
}

func (ts *CleanerTestSuite) withJobs() {
	jobs := jobList(ts.NS, 3, 2, 2)
	ts.EnsureJobs(jobs)
}

func (ts *CleanerTestSuite) runCleanup() {
	objCleaner := NewObjectCleaner(ts.Client, newLimiter(1, 1))
	deleted, err := objCleaner.CleanOldObjects(ts.Ctx, ts.fetchJobs().GetJobObjects())
	ts.Assertions.NoError(err)
	ts.Assertions.Equal(2, deleted)
}

func (ts *CleanerTestSuite) assertJobsDeleted() {
	afterClean := filterDeleted(ts.fetchJobs())
	runningJobs, failedJobs, successfulJobs := groupByStatus(afterClean.GetJobObjects())
	ts.Assertions.Equal(3, len(runningJobs))
	ts.Assertions.Equal(1, len(failedJobs))
	ts.Assertions.Equal(1, len(successfulJobs))
}

func (ts *CleanerTestSuite) fetchJobs() *k8upv1.RestoreList {
	jobs := &k8upv1.RestoreList{}
	ts.Assertions.NoError(ts.Client.List(ts.Ctx, jobs, client.InNamespace(ts.NS)))
	return jobs
}

func jobList(ns string, running, failed, successful int) *k8upv1.RestoreList {
	runningJobs := make([]k8upv1.Restore, running)
	for i := range runningJobs {
		runningJobs[i] = createJobInNS(ns)
	}
	failedJobs := make([]k8upv1.Restore, failed)
	for i := range failedJobs {
		failedJobs[i] = createJobInNS(ns)
		markJobFailed(&failedJobs[i])
	}
	successfulJobs := make([]k8upv1.Restore, successful)
	for i := range successfulJobs {
		successfulJobs[i] = createJobInNS(ns)
		markJobSuccessful(&successfulJobs[i])
	}

	return &k8upv1.RestoreList{
		Items: append(runningJobs, append(failedJobs, successfulJobs...)...),
	}
}

func (ts *CleanerTestSuite) EnsureJobs(jobs *k8upv1.RestoreList) {
	for _, jobItem := range jobs.Items {
		item := &jobItem
		deepCopy := jobItem.DeepCopy()
		ts.EnsureResources(item)
		// client.Create nullifies the status (because of Subresource) so we'll set it again.
		item.Status = deepCopy.Status
		ts.UpdateStatus(item)
	}
}

func createJobInNS(ns string) k8upv1.Restore {
	return k8upv1.Restore{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "job-" + string(uuid.NewUUID())},
		Spec:       k8upv1.RestoreSpec{},
	}
}

func markJobSuccessful(job k8upv1.JobObject) {
	job.SetStatus(k8upv1.Status{
		Conditions: []metav1.Condition{
			{
				Type:               k8upv1.ConditionCompleted.String(),
				Status:             metav1.ConditionTrue,
				Reason:             k8upv1.ReasonSucceeded.String(),
				LastTransitionTime: metav1.Now(),
			},
		},
	})
}

func markJobFailed(job k8upv1.JobObject) {
	job.SetStatus(k8upv1.Status{
		Conditions: []metav1.Condition{
			{
				Type:               k8upv1.ConditionCompleted.String(),
				Status:             metav1.ConditionTrue,
				Reason:             k8upv1.ReasonFailed.String(),
				LastTransitionTime: metav1.Now(),
			},
		},
	})
}

func filterDeleted(list *k8upv1.RestoreList) *k8upv1.RestoreList {
	out := make([]k8upv1.Restore, 0, len(list.Items))
	for _, obj := range list.Items {
		if obj.DeletionTimestamp == nil {
			out = append(out, obj)
		}
	}
	list.Items = out
	return list
}
