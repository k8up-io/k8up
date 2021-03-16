// +build integration

package controllers_test

import (
	"context"
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
)

type CheckTestSuite struct {
	EnvTestSuite

	GivenCheck *k8upv1a1.Check
	CheckName  string
}

func Test_Check(t *testing.T) {
	suite.Run(t, new(CheckTestSuite))
}

func (c *CheckTestSuite) TestReconciliation() {
	c.givenCheckResource()

	result := c.whenReconcile()

	c.Assert().GreaterOrEqual(result.RequeueAfter, 30*time.Second)
	c.expectAJobEventually()
}

func (c *CheckTestSuite) BeforeTest(suiteName, testName string) {
	c.CheckName = "check-integration-test"
}

func NewCheckResource(restoreName, namespace string) *k8upv1a1.Check {
	keepJobs := 5
	return &k8upv1a1.Check{
		Spec: k8upv1a1.CheckSpec{
			KeepJobs: &keepJobs,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      restoreName,
			Namespace: namespace,
		},
	}
}

func (c *CheckTestSuite) givenCheckResource() {
	c.GivenCheck = NewCheckResource(c.CheckName, c.NS)
	c.EnsureResources(c.GivenCheck)
}

func (c *CheckTestSuite) whenReconcile() controllerruntime.Result {
	controller := controllers.CheckReconciler{
		Client: c.Client,
		Log:    c.Logger,
		Scheme: c.Scheme,
	}

	key := types.NamespacedName{
		Namespace: c.NS,
		Name:      c.CheckName,
	}
	request := controllerruntime.Request{
		NamespacedName: key,
	}

	result, err := controller.Reconcile(c.Ctx, request)
	c.Require().NoError(err)

	return result
}

func (c *CheckTestSuite) expectAJobEventually() {
	c.RepeatedAssert(3*time.Second, time.Second, "Jobs not found", func(timedCtx context.Context) (done bool, err error) {
		jobs := new(batchv1.JobList)
		err = c.Client.List(timedCtx, jobs, &client.ListOptions{Namespace: c.NS})
		c.Require().NoError(err)

		jobsLen := len(jobs.Items)
		c.T().Logf("%d Jobs found", jobsLen)

		if jobsLen > 0 {
			c.Len(jobs.Items, 1)
			return true, err
		}

		return
	})
}
