//go:build integration

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

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/controllers"
	"github.com/k8up-io/k8up/v2/envtest"
)

type RestoreTestSuite struct {
	envtest.Suite

	GivenRestore *k8upv1.Restore
	RestoreName  string
}

func Test_Restore(t *testing.T) {
	suite.Run(t, new(RestoreTestSuite))
}

func (r *RestoreTestSuite) TestReconciliation() {
	r.givenRestoreResource()

	result := r.whenReconcile()

	r.Assert().GreaterOrEqual(result.RequeueAfter, 30*time.Second)
	r.expectAJobEventually()
}

func (r *RestoreTestSuite) BeforeTest(suiteName, testName string) {
	r.RestoreName = "restore-integration-test"
}

func NewRestoreResource(restoreName, namespace string) *k8upv1.Restore {
	return &k8upv1.Restore{
		Spec: k8upv1.RestoreSpec{
			RestoreMethod: &k8upv1.RestoreMethod{
				S3: &k8upv1.S3Spec{
					Bucket:   "backups",
					Endpoint: "https://s3-endpoint.local",
				},
			},
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      restoreName,
			Namespace: namespace,
		},
	}
}

func (r *RestoreTestSuite) givenRestoreResource() {
	r.GivenRestore = NewRestoreResource(r.RestoreName, r.NS)
	r.EnsureResources(r.GivenRestore)
}

func (r *RestoreTestSuite) whenReconcile() controllerruntime.Result {
	controller := controllers.RestoreReconciler{
		Client: r.Client,
		Log:    r.Logger,
		Scheme: r.Scheme,
	}

	key := types.NamespacedName{
		Namespace: r.NS,
		Name:      r.RestoreName,
	}
	request := controllerruntime.Request{
		NamespacedName: key,
	}

	result, err := controller.Reconcile(r.Ctx, request)
	r.Require().NoError(err)

	return result
}

func (r *RestoreTestSuite) expectAJobEventually() {
	r.RepeatedAssert(3*time.Second, time.Second, "Jobs not found", func(timedCtx context.Context) (done bool, err error) {
		jobs := new(batchv1.JobList)
		err = r.Client.List(timedCtx, jobs, &client.ListOptions{Namespace: r.NS})
		r.Require().NoError(err)

		jobsLen := len(jobs.Items)
		r.T().Logf("%d Jobs found", jobsLen)

		if jobsLen > 0 {
			r.Len(jobs.Items, 1)
			return true, err
		}

		return
	})
}
