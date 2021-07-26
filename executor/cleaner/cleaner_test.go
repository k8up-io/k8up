// +build integration

package cleaner_test

import (
	"context"
	"testing"

	k8upv1a1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/executor/cleaner"
	"github.com/vshn/k8up/job"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCleanup(t *testing.T) {
	jobs := jobList(3, 2, 2)
	client := newMockClient(jobs)

	objCleaner := &cleaner.ObjectCleaner{Client: client, Limits: newLimiter(1, 1), Log: logr.DiscardLogger{}}
	deleted, err := objCleaner.CleanOldObjects(context.TODO(), jobs.GetJobObjects())
	assert.NoError(t, err)
	assert.Equal(t, 2, deleted)

	afterClean := &k8upv1a1.RestoreList{}
	assert.NoError(t, client.List(context.TODO(), afterClean))
	runningJobs, failedJobs, successfulJobs := job.GroupByStatus(afterClean.GetJobObjects())
	assert.Equal(t, 3, len(runningJobs))
	assert.Equal(t, 1, len(failedJobs))
	assert.Equal(t, 1, len(successfulJobs))
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

func newMockClient(jobs *k8upv1a1.RestoreList) client.Client {
	objs := make([]client.Object, len(jobs.Items))
	for i := range jobs.Items {
		objs[i] = &jobs.Items[i]
	}

	scheme := runtime.NewScheme()
	k8upv1a1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func jobList(running, failed, successful int) *k8upv1a1.RestoreList {
	runningJobs := make([]k8upv1a1.Restore, running)
	for i := range runningJobs {
		runningJobs[i] = createJob()
	}
	failedJobs := make([]k8upv1a1.Restore, failed)
	for i := range failedJobs {
		failedJobs[i] = createJob()
		markJobFailed(&failedJobs[i])
	}
	successfulJobs := make([]k8upv1a1.Restore, successful)
	for i := range successfulJobs {
		successfulJobs[i] = createJob()
		markJobSuccessful(&successfulJobs[i])
	}

	return &k8upv1a1.RestoreList{
		Items: append(runningJobs, append(failedJobs, successfulJobs...)...),
	}
}

func createJob() k8upv1a1.Restore {
	return k8upv1a1.Restore{
		ObjectMeta: metav1.ObjectMeta{Name: "job-" + string(uuid.NewUUID())},
		Spec:       k8upv1a1.RestoreSpec{},
	}
}

func markJobSuccessful(job k8upv1a1.JobObject) {
	job.SetStatus(k8upv1a1.Status{
		Conditions: []metav1.Condition{
			{
				Type:   k8upv1a1.ConditionCompleted.String(),
				Status: metav1.ConditionTrue,
				Reason: k8upv1a1.ReasonSucceeded.String(),
			},
		},
	})
}

func markJobFailed(job k8upv1a1.JobObject) {
	job.SetStatus(k8upv1a1.Status{
		Conditions: []metav1.Condition{
			{
				Type:   k8upv1a1.ConditionCompleted.String(),
				Status: metav1.ConditionTrue,
				Reason: k8upv1a1.ReasonFailed.String(),
			},
		},
	})
}
