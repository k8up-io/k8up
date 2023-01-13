package cleaner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
)

func TestGroupByStatus(t *testing.T) {
	successJob := createJob(completedStatusWithReason(k8upv1.ReasonSucceeded))
	failedJob := createJob(completedStatusWithReason(k8upv1.ReasonFailed))
	runningJob := createJob(k8upv1.Status{})

	runningJobs, failedJobs, successfulJobs := groupByStatus([]k8upv1.JobObject{&successJob, &failedJob, &runningJob})
	assert.Len(t, runningJobs, 1)
	assert.True(t, runningJobs[0] == &runningJob)
	assert.Len(t, failedJobs, 1)
	assert.True(t, failedJobs[0] == &failedJob)
	assert.Len(t, successfulJobs, 1)
	assert.True(t, successfulJobs[0] == &successJob)

}

func createJob(status k8upv1.Status) k8upv1.Restore {
	return k8upv1.Restore{
		ObjectMeta: metav1.ObjectMeta{Name: "job-" + string(uuid.NewUUID())},
		Spec:       k8upv1.RestoreSpec{},
		Status:     status,
	}
}

func completedStatusWithReason(r k8upv1.ConditionReason) k8upv1.Status {
	return k8upv1.Status{
		Conditions: []metav1.Condition{
			{
				Type:   k8upv1.ConditionCompleted.String(),
				Status: metav1.ConditionTrue,
				Reason: r.String(),
			},
		},
	}
}
