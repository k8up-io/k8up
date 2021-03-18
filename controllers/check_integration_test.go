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

func (c *CheckTestSuite) BeforeTest(_, _ string) {
	c.CheckBaseName = "check-integration-test"
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

func (c *CheckTestSuite) TestReconciliation() {
	c.givenCheckResources(1)

	result := c.whenReconcile()
	c.Assert().GreaterOrEqual(result.RequeueAfter, 30*time.Second)

	c.expectNumberOfJobsEventually(1)
}

func (c *CheckTestSuite) TestJobCleanup() {
	keepJobs := 1
	c.KeepJobs = &keepJobs

	createJobs := 2
	c.givenCheckResources(createJobs)

	c.whenReconcile()
	c.expectNumberOfJobsEventually(createJobs)

	c.whenJobCallbackIsInvoked(c.CheckNames[0])
	c.expectCheckCleanupEventually(createJobs - keepJobs)
}

func (c *CheckTestSuite) expectCheckCleanupEventually(expectedDeletes int) {
	failureMsg := fmt.Sprintf("Not enough Checks deleted, expected %d.", expectedDeletes)
	c.RepeatedAssert(10*time.Second, time.Second, failureMsg, func(timedCtx context.Context) (done bool, err error) {
		checkResourceList := &k8upv1a1.CheckList{}
		err = c.Client.List(c.Ctx, checkResourceList, &client.ListOptions{
			Namespace: c.NS,
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

		c.T().Logf("%d deleted Checks found", amountOfDeletedItems)
		done = amountOfDeletedItems == expectedDeletes

		return
	})
}

func (c *CheckTestSuite) whenJobCallbackIsInvoked(checkName string) {
	checkNSName := types.NamespacedName{Name: checkName, Namespace: c.NS}

	check := &batchv1.Job{}
	err := c.Client.Get(c.Ctx, checkNSName, check)
	c.Require().NoError(err)

	o := observer.GetObserver()
	observableJob := o.GetJobByName(checkNSName.String())
	observableJob.Event = observer.Suceeded
	observableJob.Job = check

	eventChannel := o.GetUpdateChannel()
	eventChannel <- observableJob
}

func (c *CheckTestSuite) givenCheckResources(amount int) {
	for i := 0; i < amount; i++ {
		checkName := c.CheckBaseName + strconv.Itoa(i)
		check := NewCheckResource(checkName, c.NS, c.KeepJobs)
		c.EnsureResources(check)
		c.GivenChecks = append(c.GivenChecks, check)
		c.CheckNames = append(c.CheckNames, checkName)
	}
}

func (c *CheckTestSuite) whenReconcile() (lastResult controllerruntime.Result) {
	for _, checkName := range c.CheckNames {
		controller := controllers.CheckReconciler{
			Client: c.Client,
			Log:    c.Logger,
			Scheme: c.Scheme,
		}

		key := types.NamespacedName{
			Namespace: c.NS,
			Name:      checkName,
		}
		request := controllerruntime.Request{
			NamespacedName: key,
		}

		result, err := controller.Reconcile(c.Ctx, request)
		c.Require().NoError(err)

		lastResult = result
	}

	return
}

func (c *CheckTestSuite) expectNumberOfJobsEventually(jobAmount int) {
	c.RepeatedAssert(10*time.Second, time.Second, "Jobs not found", func(timedCtx context.Context) (done bool, err error) {
		jobs := new(batchv1.JobList)
		err = c.Client.List(timedCtx, jobs, &client.ListOptions{Namespace: c.NS})
		c.Require().NoError(err)

		jobsLen := len(jobs.Items)
		c.T().Logf("%d Jobs found", jobsLen)

		if jobsLen >= jobAmount {
			return true, err
		}

		return
	})
}
