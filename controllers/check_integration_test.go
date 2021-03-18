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

	k8upv1a1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/controllers"
	"github.com/vshn/k8up/observer"
)

type CheckTestSuite struct {
	EnvTestSuite

	CheckBaseName string

	CheckNames  []string
	GivenChecks []*k8upv1a1.Check
	KeepJobs    *int
}

func Test_Check(t *testing.T) {
	suite.Run(t, new(CheckTestSuite))
}

func (ts *CheckTestSuite) BeforeTest(_, _ string) {
	ts.CheckBaseName = "check-integration-test"
}

func NewCheckResource(restoreName, namespace string, keepJobs *int) *k8upv1a1.Check {
	return &k8upv1a1.Check{
		Spec: k8upv1a1.CheckSpec{
			KeepJobs: keepJobs,
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
	keepJobs := 1
	ts.KeepJobs = &keepJobs

	createJobs := 2
	ts.givenCheckResources(createJobs)

	ts.whenReconcile()
	ts.expectNumberOfJobsEventually(createJobs)

	ts.whenJobCallbackIsInvoked(ts.CheckNames[0])
	ts.expectCheckCleanupEventually(createJobs - keepJobs)
}

func (ts *CheckTestSuite) expectCheckCleanupEventually(expectedDeletes int) {
	failureMsg := fmt.Sprintf("Not enough Checks deleted, expected %d.", expectedDeletes)
	ts.RepeatedAssert(10*time.Second, time.Second, failureMsg, func(timedCtx context.Context) (done bool, err error) {
		checkResourceList := &k8upv1a1.CheckList{}
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

func (ts *CheckTestSuite) whenJobCallbackIsInvoked(checkName string) {
	checkNSName := types.NamespacedName{Name: checkName, Namespace: ts.NS}

	check := &batchv1.Job{}
	ts.FetchResource(checkNSName, check)

	o := observer.GetObserver()
	observableJob := o.GetJobByName(checkNSName.String())
	observableJob.Event = observer.Suceeded
	observableJob.Job = check

	eventChannel := o.GetUpdateChannel()
	eventChannel <- observableJob
}

func (ts *CheckTestSuite) givenCheckResources(amount int) {
	for i := 0; i < amount; i++ {
		checkName := ts.CheckBaseName + strconv.Itoa(i)
		check := NewCheckResource(checkName, ts.NS, ts.KeepJobs)
		ts.EnsureResources(check)
		ts.GivenChecks = append(ts.GivenChecks, check)
		ts.CheckNames = append(ts.CheckNames, checkName)
	}
}

func (ts *CheckTestSuite) whenReconcile() (lastResult controllerruntime.Result) {
	for _, checkName := range ts.CheckNames {
		controller := controllers.CheckReconciler{
			Client: ts.Client,
			Log:    ts.Logger,
			Scheme: ts.Scheme,
		}

		key := types.NamespacedName{
			Namespace: ts.NS,
			Name:      checkName,
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
