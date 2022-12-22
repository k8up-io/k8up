//go:build integration

package schedulecontroller

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
)

func (ts *ScheduleControllerTestSuite) givenScheduleResource(schedule k8upv1.ScheduleDefinition) {
	givenSchedule := ts.newScheduleSpec("test", schedule)
	ts.EnsureResources(givenSchedule)
	ts.givenSchedule = givenSchedule
}

func (ts *ScheduleControllerTestSuite) givenEffectiveSchedule() {
	ts.givenSchedule.Status.EffectiveSchedules = []k8upv1.EffectiveSchedule{
		{JobType: k8upv1.BackupType, GeneratedSchedule: "somevaluetobechanged"},
	}
	ts.UpdateStatus(ts.givenSchedule)
}

func (ts *ScheduleControllerTestSuite) newScheduleSpec(name string, schedule k8upv1.ScheduleDefinition) *k8upv1.Schedule {
	return &k8upv1.Schedule{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ts.NS},
		Spec: k8upv1.ScheduleSpec{
			Backup: &k8upv1.BackupSchedule{
				ScheduleCommon: &k8upv1.ScheduleCommon{
					Schedule: schedule,
				},
			},
		},
	}
}

func (ts *ScheduleControllerTestSuite) thenAssertCondition(resultSchedule *k8upv1.Schedule, condition k8upv1.ConditionType, reason k8upv1.ConditionReason, containsMessage string) {
	c := meta.FindStatusCondition(resultSchedule.Status.Conditions, condition.String())
	ts.Assert().NotNil(c)
	ts.Assert().Equal(reason.String(), c.Reason)
	ts.Assert().Contains(c.Message, containsMessage)
}
