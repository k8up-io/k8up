//go:build integration

package schedulecontroller

import (
	"testing"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/envtest"
	"github.com/stretchr/testify/suite"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type (
	ScheduleControllerTestSuite struct {
		envtest.Suite
		reconciler              *ScheduleReconciler
		givenSchedule           *k8upv1.Schedule
		givenEffectiveSchedules []k8upv1.EffectiveSchedule
	}
)

func Test_Schedule(t *testing.T) {
	suite.Run(t, new(ScheduleControllerTestSuite))
}

func (ts *ScheduleControllerTestSuite) BeforeTest(suiteName, testName string) {
	ts.reconciler = &ScheduleReconciler{
		Kube: ts.Client,
	}
}

func (ts *ScheduleControllerTestSuite) Test_GivenScheduleWithRandomSchedules_WhenReconcile_ThenUpdateEffectiveScheduleInStatus() {
	ts.givenScheduleResource(ScheduleDailyRandom)

	ts.whenReconciling(ts.givenSchedule)

	actualSchedule := &k8upv1.Schedule{}
	ts.FetchResource(k8upv1.MapToNamespacedName(ts.givenSchedule), actualSchedule)
	ts.thenAssertCondition(actualSchedule, k8upv1.ConditionReady, k8upv1.ReasonReady, "resource is ready")

	effectiveSchedule := actualSchedule.Status.EffectiveSchedules[0]
	ts.Assert().Equal(k8upv1.BackupType, effectiveSchedule.JobType, "job type")
	ts.Assert().NotEmpty(effectiveSchedule.GeneratedSchedule, "generated schedule")
}

func (ts *ScheduleControllerTestSuite) Test_GivenEffectiveScheduleWithRandomSchedules_WhenChangingToStandardSchedule_ThenCleanupEffectiveScheduleInStatus() {
	ts.givenScheduleResource("* * * * *")
	ts.givenEffectiveSchedule()

	ts.whenReconciling(ts.givenSchedule)

	actualSchedule := &k8upv1.Schedule{}
	ts.FetchResource(k8upv1.MapToNamespacedName(ts.givenSchedule), actualSchedule)
	ts.thenAssertCondition(actualSchedule, k8upv1.ConditionReady, k8upv1.ReasonReady, "resource is ready")

	ts.Assert().Len(actualSchedule.Status.EffectiveSchedules, 0, "slice of effective schedules")
}

func (ts *ScheduleControllerTestSuite) Test_GivenEffectiveScheduleWithRandomSchedules_WhenReconcile_ThenUpdateScheduleInStatus() {
	ts.givenScheduleResource(ScheduleHourlyRandom)
	ts.givenEffectiveSchedule()

	ts.whenReconciling(ts.givenSchedule)

	actualSchedule := &k8upv1.Schedule{}
	name := k8upv1.MapToNamespacedName(ts.givenSchedule)
	ts.FetchResource(name, actualSchedule)
	ts.thenAssertCondition(actualSchedule, k8upv1.ConditionReady, k8upv1.ReasonReady, "resource is ready")
	ts.Assert().Len(actualSchedule.Status.EffectiveSchedules, 1, "slice of effective schedules")
	ts.Assert().NotEqual("somevaluetobechanged", actualSchedule.Status.EffectiveSchedules[0].GeneratedSchedule)
}

func (ts *ScheduleControllerTestSuite) whenReconciling(givenSchedule *k8upv1.Schedule) {
	newResult, err := ts.reconciler.Provision(ts.Ctx, givenSchedule)
	ts.Assert().NoError(err)
	ts.Assert().False(newResult.Requeue)
}

func (ts *ScheduleControllerTestSuite) whenListSchedules() []k8upv1.Schedule {
	schedules := &k8upv1.ScheduleList{}
	ts.FetchResources(schedules, client.InNamespace(ts.NS))
	return schedules.Items
}
