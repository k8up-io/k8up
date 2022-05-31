//go:build integration
// +build integration

package controllers_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/controllers"
	"github.com/k8up-io/k8up/v2/envtest"
	"github.com/k8up-io/k8up/v2/operator/observer"
)

type CheckTestSuite struct {
	envtest.Suite

	CheckBaseName string

	CheckNames     []string
	GivenChecks    []*k8upv1.Check
	KeepSuccessful int
	KeepFailed     int
}

func Test_Check(t *testing.T) {
	suite.Run(t, new(CheckTestSuite))
}

func (ts *CheckTestSuite) BeforeTest(_, _ string) {
	ts.CheckBaseName = "check-integration-test"
}

func NewCheckResource(restoreName, namespace string, keepFailed, keepSuccessful int) *k8upv1.Check {
	return &k8upv1.Check{
		Spec: k8upv1.CheckSpec{
			SuccessfulJobsHistoryLimit: &keepSuccessful,
			FailedJobsHistoryLimit:     &keepFailed,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      restoreName,
			Namespace: namespace,
		},
	}
}

func (ts *CheckTestSuite) TestReconciliation() {
	ts.givenCheckResources(1)

	result := ts.whenReconcile()
	ts.Assert().GreaterOrEqual(result.RequeueAfter, 30*time.Second)

	ts.expectNumberOfJobsEventually(1)
}

func (ts *CheckTestSuite) TestJobCleanup() {
	ts.KeepSuccessful = 1
	ts.KeepFailed = 2

	createJobs := 6
	ts.givenCheckResources(createJobs)

	ts.whenReconcile()
	ts.expectNumberOfJobsEventually(createJobs)

	successfulJobs := 2
	failedJobs := 3
	for i := 0; i < successfulJobs; i++ {
		ts.whenJobCallbackIsInvoked(ts.GivenChecks[i], observer.Succeeded)
	}
	for i := successfulJobs; i < successfulJobs+failedJobs; i++ {
		ts.whenJobCallbackIsInvoked(ts.GivenChecks[i], observer.Failed)
	}

	ts.expectCheckCleanupEventually((successfulJobs - ts.KeepSuccessful) + (failedJobs - ts.KeepFailed))
}

func (ts *CheckTestSuite) expectCheckCleanupEventually(expectedDeletes int) {
	failureMsg := fmt.Sprintf("Not enough Checks deleted, expected %d.", expectedDeletes)
	ts.RepeatedAssert(10*time.Second, time.Second, failureMsg, func(timedCtx context.Context) (done bool, err error) {
		checkResourceList := &k8upv1.CheckList{}
		err = ts.Client.List(ts.Ctx, checkResourceList, &client.ListOptions{
			Namespace: ts.NS,
		})
		if err != nil {
			return
		}

		amountOfDeletedItems := 0
		for _, item := range checkResourceList.Items {
			if item.DeletionTimestamp != nil {
				amountOfDeletedItems++
			}
		}

		ts.T().Logf("%d deleted Checks found", amountOfDeletedItems)
		done = amountOfDeletedItems == expectedDeletes

		return
	})
}

func (ts *CheckTestSuite) whenJobCallbackIsInvoked(check k8upv1.JobObject, evtType observer.EventType) {
	checkNSName := types.NamespacedName{Name: check.GetJobName(), Namespace: ts.NS}

	childJob := &batchv1.Job{}
	ts.FetchResource(checkNSName, childJob)

	o := observer.GetObserver()
	observableJob := o.GetJobByName(checkNSName.String())
	observableJob.Event = evtType
	observableJob.Job = childJob

	eventChannel := o.GetUpdateChannel()
	eventChannel <- observableJob
}

func (ts *CheckTestSuite) givenCheckResources(amount int) {
	for i := 0; i < amount; i++ {
		checkName := ts.CheckBaseName + strconv.Itoa(i)
		check := NewCheckResource(checkName, ts.NS, ts.KeepFailed, ts.KeepSuccessful)
		ts.EnsureResources(check)
		ts.GivenChecks = append(ts.GivenChecks, check)
		ts.CheckNames = append(ts.CheckNames, checkName)
	}
}

func (ts *CheckTestSuite) whenReconcile() (lastResult controllerruntime.Result) {
	for _, check := range ts.GivenChecks {
		controller := controllers.CheckReconciler{
			Client: ts.Client,
			Log:    ts.Logger,
			Scheme: ts.Scheme,
		}

		key := types.NamespacedName{
			Namespace: ts.NS,
			Name:      check.GetMetaObject().GetName(),
		}
		request := controllerruntime.Request{
			NamespacedName: key,
		}

		result, err := controller.Reconcile(ts.Ctx, request)
		ts.Require().NoError(err)

		lastResult = result
	}

	return
}

func (ts *CheckTestSuite) expectNumberOfJobsEventually(jobAmount int) {
	ts.RepeatedAssert(10*time.Second, time.Second, "Jobs not found", func(timedCtx context.Context) (done bool, err error) {
		jobs := new(batchv1.JobList)
		err = ts.Client.List(timedCtx, jobs, &client.ListOptions{Namespace: ts.NS})
		ts.Require().NoError(err)

		jobsLen := len(jobs.Items)
		ts.T().Logf("%d Jobs found", jobsLen)

		if jobsLen >= jobAmount {
			return true, err
		}

		return
	})
}
