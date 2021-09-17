//go:build integration
// +build integration

package controllers_test

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	k8upv1 "github.com/k8up-io/k8up/api/v1"
	"github.com/k8up-io/k8up/controllers"
	"github.com/k8up-io/k8up/envtest"
	"github.com/k8up-io/k8up/operator/cfg"
	"github.com/k8up-io/k8up/operator/handler"
	"github.com/k8up-io/k8up/operator/scheduler"
)

type (
	ScheduleControllerTestSuite struct {
		envtest.Suite
		reconciler              *controllers.ScheduleReconciler
		givenSchedule           *k8upv1.Schedule
		givenEffectiveSchedules []k8upv1.EffectiveSchedule
	}
)

func Test_Schedule(t *testing.T) {
	suite.Run(t, new(ScheduleControllerTestSuite))
}

func (ts *ScheduleControllerTestSuite) BeforeTest(suiteName, testName string) {
	cfg.Config.OperatorNamespace = ts.NS
	ts.reconciler = &controllers.ScheduleReconciler{
		Client: ts.Client,
		Log:    ts.Logger.WithName(suiteName + "_" + testName),
		Scheme: ts.Scheme,
	}
}

func (ts *ScheduleControllerTestSuite) Test_GivenScheduleWithRandomSchedules_WhenReconcile_ThenCreateEffectiveSchedule() {
	ts.givenScheduleResource(handler.ScheduleDailyRandom)

	ts.whenReconciling(ts.givenSchedule)

	ts.thenAssertEffectiveScheduleExists(ts.givenSchedule.Name, handler.ScheduleDailyRandom)

	actualSchedule := &k8upv1.Schedule{}
	ts.FetchResource(k8upv1.MapToNamespacedName(ts.givenSchedule), actualSchedule)
	ts.thenAssertCondition(actualSchedule, k8upv1.ConditionReady, k8upv1.ReasonReady, "resource is ready")

	actualESList := ts.whenListEffectiveSchedules()
	ts.Assert().Len(actualESList, 1)
}

func (ts *ScheduleControllerTestSuite) Test_GivenEffectiveScheduleWithRandomSchedules_WhenChangingToStandardSchedule_ThenCleanupEffectiveSchedule() {
	ts.givenScheduleResource("* * * * *")
	ts.givenEffectiveScheduleResource(ts.givenSchedule.Name)

	ts.whenReconciling(ts.givenSchedule)

	actualSchedule := &k8upv1.Schedule{}
	ts.FetchResource(k8upv1.MapToNamespacedName(ts.givenSchedule), actualSchedule)
	ts.thenAssertCondition(actualSchedule, k8upv1.ConditionReady, k8upv1.ReasonReady, "resource is ready")

	actualESList := ts.whenListEffectiveSchedules()
	ts.Assert().Len(actualESList, 0)
}

func (ts *ScheduleControllerTestSuite) Test_GivenEffectiveScheduleWithRandomSchedules_WhenReconcile_ThenUsePreGeneratedSchedule() {
	ts.givenScheduleResource(handler.ScheduleHourlyRandom)
	ts.givenEffectiveScheduleResource(ts.givenSchedule.Name)

	ts.whenReconciling(ts.givenSchedule)

	actualSchedule := &k8upv1.Schedule{}
	name := k8upv1.MapToNamespacedName(ts.givenSchedule)
	ts.FetchResource(name, actualSchedule)
	ts.thenAssertCondition(actualSchedule, k8upv1.ConditionReady, k8upv1.ReasonReady, "resource is ready")
	scheduler.GetScheduler().HasSchedule(name, "1 * * * *", k8upv1.BackupType)

	actualESList := ts.whenListEffectiveSchedules()
	ts.Assert().Len(actualESList, 1)
}

func (ts *ScheduleControllerTestSuite) Test_GivenEffectiveScheduleWithRandomSchedules_WhenDeletingSchedule_ThenCleanupEffectiveSchedule() {
	ts.givenScheduleResource("* * * * *")
	ts.givenEffectiveScheduleResource(ts.givenSchedule.Name)

	controllerutil.AddFinalizer(ts.givenSchedule, k8upv1.ScheduleFinalizerName)
	ts.UpdateResources(ts.givenSchedule)
	ts.DeleteResources(ts.givenSchedule)

	ts.whenReconciling(ts.givenSchedule)

	actualScheduleList := ts.whenListSchedules()
	ts.Assert().Len(actualScheduleList, 0)

	actualESList := ts.whenListEffectiveSchedules()
	ts.Assert().Len(actualESList, 0)
}

func (ts *ScheduleControllerTestSuite) Test_GivenEffectiveScheduleWithRandomSchedules_WhenChangingSchedule_ThenMakeNewEffectiveSchedule() {
	ts.givenScheduleResource(handler.ScheduleDailyRandom)
	ts.givenEffectiveScheduleResource(ts.givenSchedule.Name)

	ts.whenReconciling(ts.givenSchedule)

	actualESList := ts.whenListEffectiveSchedules()
	ts.Assert().Len(actualESList, 1)
	ts.thenAssertEffectiveScheduleExists(ts.givenSchedule.Name, handler.ScheduleDailyRandom)
}

func (ts *ScheduleControllerTestSuite) whenReconciling(givenSchedule *k8upv1.Schedule) {
	newResult, err := ts.reconciler.Reconcile(ts.Ctx, ts.MapToRequest(givenSchedule))
	ts.Assert().NoError(err)
	ts.Assert().False(newResult.Requeue)
}

func (ts *ScheduleControllerTestSuite) whenListEffectiveSchedules() []k8upv1.EffectiveSchedule {
	effectiveSchedules := &k8upv1.EffectiveScheduleList{}
	ts.FetchResources(effectiveSchedules, client.InNamespace(ts.NS))
	return effectiveSchedules.Items
}

func (ts *ScheduleControllerTestSuite) whenListSchedules() []k8upv1.Schedule {
	schedules := &k8upv1.ScheduleList{}
	ts.FetchResources(schedules, client.InNamespace(ts.NS))
	return schedules.Items
}
