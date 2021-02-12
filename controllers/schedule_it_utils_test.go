// +build integration

package controllers_test

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/handler"
)

func (ts *ScheduleControllerTestSuite) givenScheduleResource(schedule k8upv1alpha1.ScheduleDefinition) {
	givenSchedule := ts.newScheduleSpec("test", schedule)
	ts.EnsureResources(givenSchedule)
	ts.givenSchedule = givenSchedule
}

func (ts *ScheduleControllerTestSuite) givenEffectiveScheduleResource(scheduleName string) {
	givenSchedule := k8upv1alpha1.EffectiveSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: scheduleName + "-randomstring", Namespace: ts.NS},
		Spec: k8upv1alpha1.EffectiveScheduleSpec{
			GeneratedSchedule: "1 * * * *",
			JobType:           k8upv1alpha1.BackupType,
			ScheduleRefs: []k8upv1alpha1.ScheduleRef{
				{Name: scheduleName, Namespace: ts.NS},
			},
			OriginalSchedule: handler.ScheduleHourlyRandom,
		},
	}
	ts.EnsureResources(&givenSchedule)
	ts.givenEffectiveSchedules = append(ts.givenEffectiveSchedules, givenSchedule)
}

func (ts *ScheduleControllerTestSuite) newScheduleSpec(name string, schedule k8upv1alpha1.ScheduleDefinition) *k8upv1alpha1.Schedule {
	return &k8upv1alpha1.Schedule{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ts.NS},
		Spec: k8upv1alpha1.ScheduleSpec{
			Backup: &k8upv1alpha1.BackupSchedule{
				ScheduleCommon: &k8upv1alpha1.ScheduleCommon{
					Schedule: schedule,
				},
			},
		},
	}
}

func (ts *ScheduleControllerTestSuite) thenAssertEffectiveScheduleExists(expectedScheduleName string, originalSchedule k8upv1alpha1.ScheduleDefinition) {
	list := ts.whenListEffectiveSchedules()
	ts.Require().NotEmpty(list)
	spec := list[0].Spec
	ts.Require().Len(spec.ScheduleRefs, 1)
	ref := spec.ScheduleRefs[0]
	ts.Assert().Equal(expectedScheduleName, ref.Name)
	ts.Assert().Equal(spec.OriginalSchedule, originalSchedule)
	ts.Assert().Equal(ts.NS, ref.Namespace)
	ts.Assert().False(spec.GeneratedSchedule.IsRandom())
}

func (ts *ScheduleControllerTestSuite) thenAssertCondition(resultSchedule *k8upv1alpha1.Schedule, condition k8upv1alpha1.ConditionType, reason k8upv1alpha1.ConditionReason, containsMessage string) {
	c := meta.FindStatusCondition(resultSchedule.Status.Conditions, condition.String())
	ts.Assert().NotNil(c)
	ts.Assert().Equal(reason.String(), c.Reason)
	ts.Assert().Contains(c.Message, containsMessage)
}
