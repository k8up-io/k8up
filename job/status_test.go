package job_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	k8upv1a1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/job"
)

func TestGroupByStatus(t *testing.T) {
	successJob := createJob(completedStatusWithReason(k8upv1a1.ReasonSucceeded))
	failedJob := createJob(completedStatusWithReason(k8upv1a1.ReasonFailed))
	runningJob := createJob(k8upv1a1.Status{})

	runningJobs, failedJobs, successfulJobs := job.GroupByStatus([]k8upv1a1.JobObject{&successJob, &failedJob, &runningJob})
	assert.Len(t, runningJobs, 1)
	assert.True(t, runningJobs[0] == &runningJob)
	assert.Len(t, failedJobs, 1)
	assert.True(t, failedJobs[0] == &failedJob)
	assert.Len(t, successfulJobs, 1)
	assert.True(t, successfulJobs[0] == &successJob)

}

func createJob(status k8upv1a1.Status) k8upv1a1.Restore {
	return k8upv1a1.Restore{
		ObjectMeta: metav1.ObjectMeta{Name: "job-" + string(uuid.NewUUID())},
		Spec:       k8upv1a1.RestoreSpec{},
		Status:     status,
	}
}

func completedStatusWithReason(r k8upv1a1.ConditionReason) k8upv1a1.Status {
	return k8upv1a1.Status{
		Conditions: []metav1.Condition{
			{
				Type:   k8upv1a1.ConditionCompleted.String(),
				Status: metav1.ConditionTrue,
				Reason: r.String(),
			},
		},
	}
}
