// +build integration

package controllers_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1a1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/controllers"
)

type RestoreTestSuite struct {
	EnvTestSuite

	GivenRestore *k8upv1a1.Restore
	RestoreName  string
}

func Test_Restore(t *testing.T) {
	suite.Run(t, new(RestoreTestSuite))
}

func (r *RestoreTestSuite) TestReconciliation() {
	r.givenRestoreResource()

	result := r.whenReconcile()

	assert.GreaterOrEqual(r.T(), result.RequeueAfter, 30*time.Second)
	r.expectAJobEventually()
}

func (r *RestoreTestSuite) SetupTest() {
	r.RestoreName = "restore-integration-test"
}

func NewRestoreResource(restoreName, namespace string) *k8upv1a1.Restore {
	return &k8upv1a1.Restore{
		Spec: k8upv1a1.RestoreSpec{
			RestoreMethod: &k8upv1a1.RestoreMethod{
				S3: &k8upv1a1.S3Spec{
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
	err := r.Client.Create(r.Ctx, r.GivenRestore)
	require.NoError(r.T(), err)
}

func (r *RestoreTestSuite) whenReconcile() controllerruntime.Result {
	controller := controllers.RestoreReconciler{
		Client: r.Client,
		Log:    r.Logger,
		Scheme: scheme.Scheme,
	}

	key := types.NamespacedName{
		Namespace: r.NS,
		Name:      r.RestoreName,
	}
	request := controllerruntime.Request{
		NamespacedName: key,
	}

	result, err := controller.Reconcile(r.Ctx, request)
	require.NoError(r.T(), err)

	return result
}

func (r *RestoreTestSuite) expectAJobEventually() {
	r.RepeatedAssert(3*time.Second, time.Second, "Jobs not found", func(timedCtx context.Context) (done bool, err error) {
		jobs := new(batchv1.JobList)
		err = r.Client.List(timedCtx, jobs, &client.ListOptions{Namespace: r.NS})
		require.NoError(r.T(), err)

		jobsLen := len(jobs.Items)
		r.T().Logf("%d Jobs found", jobsLen)

		if jobsLen > 0 {
			assert.Len(r.T(), jobs.Items, 1)
			return true, err
		}

		return
	})
}
