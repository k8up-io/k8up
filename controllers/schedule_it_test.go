// +build integration

package controllers_test

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/cfg"
	"github.com/vshn/k8up/controllers"
	"github.com/vshn/k8up/handler"
	"github.com/vshn/k8up/scheduler"
)

type (
	ScheduleControllerTestSuite struct {
		EnvTestSuite
		reconciler              *controllers.ScheduleReconciler
		givenEffectiveSchedules []k8upv1alpha1.EffectiveSchedule
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
	schedule := ts.givenScheduleResource("test", handler.ScheduleDailyRandom)

	ts.whenReconciling(schedule)

	ts.thenAssertEffectiveScheduleExists(0, schedule.Name, handler.ScheduleDailyRandom)

	actualSchedule := &k8upv1alpha1.Schedule{}
	ts.FetchResource(k8upv1alpha1.GetNamespacedName(schedule), actualSchedule)
	ts.thenAssertCondition(actualSchedule, k8upv1alpha1.ConditionReady, k8upv1alpha1.ReasonReady, "resource is ready")

	actualESList := ts.whenListEffectiveSchedules()
	ts.Assert().Len(actualESList, 1)
}

func (ts *ScheduleControllerTestSuite) Test_GivenEffectiveScheduleWithRandomSchedules_WhenChangingToStandardSchedule_ThenCleanupEffectiveSchedule() {
	schedule := ts.givenScheduleResource("test", handler.ScheduleDailyRandom)
	ts.givenEffectiveScheduleResource(*schedule)

	schedule.Spec.Check.Schedule = "* * * * *"
	ts.UpdateResources(schedule)

	ts.whenReconciling(schedule)

	actualSchedule := &k8upv1alpha1.Schedule{}
	ts.FetchResource(k8upv1alpha1.GetNamespacedName(schedule), actualSchedule)
	ts.thenAssertCondition(actualSchedule, k8upv1alpha1.ConditionReady, k8upv1alpha1.ReasonReady, "resource is ready")

	actualESList := ts.whenListEffectiveSchedules()
	ts.Assert().Len(actualESList, 0)
}

func (ts *ScheduleControllerTestSuite) Test_GivenEffectiveScheduleWithRandomSchedules_WhenReconcile_ThenUsePreGeneratedSchedule() {
	schedule := ts.givenScheduleResource("test", handler.ScheduleHourlyRandom)
	ts.givenEffectiveScheduleResource(*schedule)

	ts.whenReconciling(schedule)

	actualSchedule := &k8upv1alpha1.Schedule{}
	name := k8upv1alpha1.GetNamespacedName(schedule)
	ts.FetchResource(name, actualSchedule)
	ts.thenAssertCondition(actualSchedule, k8upv1alpha1.ConditionReady, k8upv1alpha1.ReasonReady, "resource is ready")
	scheduler.GetScheduler().HasSchedule(name, "1 * * * *", k8upv1alpha1.BackupType)

	actualESList := ts.whenListEffectiveSchedules()
	ts.Assert().Len(actualESList, 1)
}

func (ts *ScheduleControllerTestSuite) Test_GivenEffectiveScheduleWithRandomSchedules_WhenDeletingSchedule_ThenCleanupEffectiveSchedule() {
	schedule := ts.givenScheduleResource("test", handler.ScheduleDailyRandom)
	ts.givenEffectiveScheduleResource(*schedule)

	controllerutil.AddFinalizer(schedule, k8upv1alpha1.ScheduleFinalizerName)
	ts.UpdateResources(schedule)
	ts.DeleteResources(schedule)

	ts.whenReconciling(schedule)

	actualScheduleList := ts.whenListSchedules()
	ts.Assert().Len(actualScheduleList, 0)

	actualESList := ts.whenListEffectiveSchedules()
	ts.Assert().Len(actualESList, 0)
}

func (ts *ScheduleControllerTestSuite) Test_GivenEffectiveScheduleWithRandomSchedules_WhenChangingSchedule_ThenMakeNewEffectiveSchedule() {
	schedule := ts.givenScheduleResourceWithBackend("test", "bucket", handler.ScheduleDailyRandom)
	ts.givenEffectiveScheduleResource(*schedule)

	schedule.Spec.Check.Schedule = handler.ScheduleHourlyRandom
	ts.UpdateResources(schedule)

	ts.whenReconciling(schedule)

	actualESList := ts.whenListEffectiveSchedules()
	ts.Require().Len(actualESList, 1)
	ts.Assert().NotEqual("test-randomstring", actualESList[0].Name)
	ts.thenAssertEffectiveScheduleExists(0, schedule.Name, handler.ScheduleDailyRandom)
}

func (ts *ScheduleControllerTestSuite) Test_GivenEffectiveScheduleWithRandomSchedules_WhenChangingBackend_ThenMakeNewEffectiveSchedule() {
	schedule1 := ts.givenScheduleResourceWithBackend("first", "bucket", handler.ScheduleDailyRandom)
	schedule2 := ts.givenScheduleResourceWithBackend("second", "another", handler.ScheduleDailyRandom)
	ts.givenEffectiveScheduleResource(*schedule1, schedule2.Name)

	ts.whenReconciling(schedule2)

	actualESList := ts.whenListEffectiveSchedules()
	ts.Require().Len(actualESList, 2)
	ts.Assert().NotEqual("test-randomstring", actualESList[0].Name)
	ts.thenAssertEffectiveScheduleExists(0, schedule1.Name, handler.ScheduleDailyRandom)
	ts.thenAssertEffectiveScheduleExists(1, schedule2.Name, handler.ScheduleDailyRandom)
}

func (ts *ScheduleControllerTestSuite) Test_GivenJobsWithSameScheduleAndBackend_WhenReconcileSecondSchedule_ThenDeduplicateFromEffectiveSchedule() {
	firstSchedule := ts.givenScheduleResourceWithBackend("first", "bucket", handler.ScheduleDailyRandom)
	secondSchedule := ts.givenScheduleResourceWithBackend("second", "bucket", handler.ScheduleDailyRandom)

	ts.whenReconciling(firstSchedule)
	ts.whenReconciling(secondSchedule)
	ts.whenReconciling(firstSchedule) // reconcile again to make sure it doesn't deduplicate itself.

	actualESList := ts.whenListEffectiveSchedules()
	ts.Assert().Len(actualESList, 1)
	ts.Assert().Len(actualESList[0].Spec.ScheduleRefs, 2)
	actualESSpec := actualESList[0].Spec
	ts.Assert().Contains(actualESSpec.ScheduleRefs, k8upv1alpha1.ScheduleRef{
		Name:      "first",
		Namespace: ts.NS,
	})
	ts.Assert().Contains(actualESSpec.ScheduleRefs, k8upv1alpha1.ScheduleRef{
		Name:      "second",
		Namespace: ts.NS,
	})
	ts.Assert().True(scheduler.GetScheduler().HasSchedule(k8upv1alpha1.GetNamespacedName(firstSchedule), actualESSpec.GeneratedSchedule, k8upv1alpha1.CheckType))
	ts.Assert().False(scheduler.GetScheduler().HasSchedule(k8upv1alpha1.GetNamespacedName(secondSchedule), actualESSpec.GeneratedSchedule, k8upv1alpha1.CheckType))
}

func (ts *ScheduleControllerTestSuite) Test_GivenJobsWithSameScheduleAndBackend_WhenRemovingDeduplicatedSchedule_ThenRemoveFromEffectiveSchedule() {
	firstSchedule := ts.givenScheduleResource("first", handler.ScheduleDailyRandom)
	_ = ts.givenScheduleResourceWithBackend("second", "bucket", handler.ScheduleDailyRandom)
	ts.givenEffectiveScheduleResource(*firstSchedule, "second")

	firstSchedule.Spec.Check.Schedule = "* 2 * * *"
	ts.UpdateResources(firstSchedule)

	ts.whenReconciling(firstSchedule)

	actualESList := ts.whenListEffectiveSchedules()
	ts.Assert().Len(actualESList, 1)
	ts.Assert().Len(actualESList[0].Spec.ScheduleRefs, 1)
	ts.Assert().Contains(actualESList[0].Spec.ScheduleRefs, k8upv1alpha1.ScheduleRef{
		Name:      "second",
		Namespace: ts.NS,
	})
}

func (ts *ScheduleControllerTestSuite) whenReconciling(givenSchedule *k8upv1alpha1.Schedule) {
	newResult, err := ts.reconciler.Reconcile(ts.Ctx, ts.MapToRequest(givenSchedule))
	ts.Assert().NoError(err)
	ts.Assert().False(newResult.Requeue)
}

func (ts *ScheduleControllerTestSuite) whenListEffectiveSchedules() []k8upv1alpha1.EffectiveSchedule {
	effectiveSchedules := &k8upv1alpha1.EffectiveScheduleList{}
	ts.FetchResources(effectiveSchedules, client.InNamespace(ts.NS))
	return effectiveSchedules.Items
}

func (ts *ScheduleControllerTestSuite) whenListSchedules() []k8upv1alpha1.Schedule {
	schedules := &k8upv1alpha1.ScheduleList{}
	ts.FetchResources(schedules, client.InNamespace(ts.NS))
	return schedules.Items
}
