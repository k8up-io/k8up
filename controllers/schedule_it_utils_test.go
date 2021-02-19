// +build integration

package controllers_test

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
)

func (ts *ScheduleControllerTestSuite) givenScheduleResource(name string, schedule k8upv1alpha1.ScheduleDefinition) *k8upv1alpha1.Schedule {
	givenSchedule := ts.newScheduleSpec(name, schedule)
	ts.EnsureResources(givenSchedule)
	return givenSchedule
}

func (ts *ScheduleControllerTestSuite) givenScheduleResourceWithBackend(name, bucket string, schedule k8upv1alpha1.ScheduleDefinition) *k8upv1alpha1.Schedule {
	givenSchedule := ts.newScheduleSpec(name, schedule)
	givenSchedule.Spec.Backend = &k8upv1alpha1.Backend{
		S3: &k8upv1alpha1.S3Spec{
			Endpoint: "https://endpoint",
			Bucket:   bucket,
		},
	}
	ts.EnsureResources(givenSchedule)
	return givenSchedule
}

func (ts *ScheduleControllerTestSuite) givenEffectiveScheduleResource(schedule k8upv1alpha1.Schedule, additionalRefs ...string) {
	givenSchedule := k8upv1alpha1.EffectiveSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: schedule.Name + "-randomstring", Namespace: ts.NS},
		Spec: k8upv1alpha1.EffectiveScheduleSpec{
			GeneratedSchedule: "1 * * * *",
			JobType:           k8upv1alpha1.CheckType,
			OriginalSchedule:  schedule.Spec.Check.Schedule,
			ScheduleRefs: []k8upv1alpha1.ScheduleRef{
				{Name: schedule.Name, Namespace: ts.NS},
			},
		},
	}
	for _, ref := range additionalRefs {
		givenSchedule.Spec.ScheduleRefs = append(givenSchedule.Spec.ScheduleRefs, k8upv1alpha1.ScheduleRef{Name: ref, Namespace: ts.NS})
	}
	if schedule.Spec.Backend != nil {
		givenSchedule.Spec.BackendString = schedule.Spec.Backend.String()
	}
	ts.EnsureResources(&givenSchedule)
	ts.givenEffectiveSchedules = append(ts.givenEffectiveSchedules, givenSchedule)
}

func (ts *ScheduleControllerTestSuite) newScheduleSpec(name string, schedule k8upv1alpha1.ScheduleDefinition) *k8upv1alpha1.Schedule {
	return &k8upv1alpha1.Schedule{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ts.NS},
		Spec: k8upv1alpha1.ScheduleSpec{
			Check: &k8upv1alpha1.CheckSchedule{
				ScheduleCommon: &k8upv1alpha1.ScheduleCommon{
					Schedule: schedule,
				},
			},
		},
	}
}

func (ts *ScheduleControllerTestSuite) thenAssertEffectiveScheduleExists(index int, expectedScheduleName string, originalSchedule k8upv1alpha1.ScheduleDefinition) {
	list := ts.whenListEffectiveSchedules()
	ts.Require().NotEmpty(list)
	spec := list[index].Spec
	ts.Require().Len(spec.ScheduleRefs, 1)
	ref := spec.ScheduleRefs[0]
	ts.Assert().Equal(expectedScheduleName, ref.Name)
	ts.Assert().Equal(ts.NS, ref.Namespace)
	ts.Assert().False(spec.GeneratedSchedule.IsRandom())
	ts.Assert().Equal(originalSchedule, spec.OriginalSchedule)
	ts.Assert().Equal(k8upv1alpha1.CheckType, spec.JobType)
}

func (ts *ScheduleControllerTestSuite) thenAssertCondition(resultSchedule *k8upv1alpha1.Schedule, condition k8upv1alpha1.ConditionType, reason k8upv1alpha1.ConditionReason, containsMessage string) {
	c := meta.FindStatusCondition(resultSchedule.Status.Conditions, condition.String())
	ts.Assert().NotNil(c)
	ts.Assert().Equal(reason.String(), c.Reason)
	ts.Assert().Contains(c.Message, containsMessage)
}
