//go:build integration

package restorecontroller

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
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

func (ts *RestoreTestSuite) TestReconciliation() {
	ts.givenRestoreResource()

	result := ts.whenReconcile()

	ts.Assert().GreaterOrEqual(result.RequeueAfter, 30*time.Second)
	ts.expectAJobEventually()
}

func (ts *RestoreTestSuite) BeforeTest(suiteName, testName string) {
	ts.RestoreName = "restore-integration-test"
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

func (ts *RestoreTestSuite) givenRestoreResource() {
	ts.GivenRestore = NewRestoreResource(ts.RestoreName, ts.NS)
	ts.EnsureResources(ts.GivenRestore)
}

func (ts *RestoreTestSuite) whenReconcile() controllerruntime.Result {
	controller := RestoreReconciler{
		Kube: ts.Client,
	}

	result, err := controller.Provision(ts.Ctx, ts.GivenRestore)
	ts.Require().NoError(err)

	return result
}

func (ts *RestoreTestSuite) expectAJobEventually() {
	jobs := new(batchv1.JobList)
	err := ts.Client.List(ts.Ctx, jobs, &client.ListOptions{Namespace: ts.NS})
	ts.Require().NoError(err)

	jobsLen := len(jobs.Items)
	ts.T().Logf("%d Jobs found", jobsLen)

	ts.Assert().Len(jobs.Items, 1)
}
