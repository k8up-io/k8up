//go:build integration

package checkcontroller

import (
	"strconv"
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

type CheckTestSuite struct {
	envtest.Suite

	CheckBaseName string

	CheckNames     []string
	GivenChecks    []*k8upv1.Check
	KeepSuccessful int
	KeepFailed     int

	BackupResource *k8upv1.Backup
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
	ts.T().Skipf("this doesn't currently work, no idea why")
	ts.givenCheckResources(1)

	result := ts.whenReconcile()
	ts.Assert().GreaterOrEqual(result.RequeueAfter, 30*time.Second)

	ts.expectNumberOfJobs(1)
}

func (ts *CheckTestSuite) TestJobCleanup() {
	ts.KeepSuccessful = 1
	ts.KeepFailed = 2

	createJobs := 6
	ts.givenCheckResourcesWithOwnership(createJobs)

	successfulJobs := 2
	failedJobs := 3
	for i := 0; i < successfulJobs; i++ {
		ts.GivenChecks[i].Status.SetSucceeded("finished")
		ts.UpdateStatus(ts.GivenChecks[i])
	}
	for i := successfulJobs; i < successfulJobs+failedJobs; i++ {
		ts.GivenChecks[i].Status.SetFailed("finished")
		ts.UpdateStatus(ts.GivenChecks[i])
	}

	// Reconcile all checks - cleanup will trigger for finished ones
	ts.whenReconcile()
	ts.expectCheckCleanup((successfulJobs - ts.KeepSuccessful) + (failedJobs - ts.KeepFailed))
}

func (ts *CheckTestSuite) expectCheckCleanup(expectedDeletes int) {
	checkResourceList := &k8upv1.CheckList{}
	err := ts.Client.List(ts.Ctx, checkResourceList, &client.ListOptions{
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
	ts.Assert().Equal(expectedDeletes, amountOfDeletedItems)
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

// givenCheckResourcesWithOwnership creates check resources with ownership labels for cleanup testing
func (ts *CheckTestSuite) givenCheckResourcesWithOwnership(amount int) {
	// Use the first check as the owner for all checks - this simulates all checks being created by the same schedule
	var ownerCheckName string

	for i := 0; i < amount; i++ {
		checkName := ts.CheckBaseName + strconv.Itoa(i)
		check := NewCheckResource(checkName, ts.NS, ts.KeepFailed, ts.KeepSuccessful)

		// Use the first check as the owner for all checks
		if i == 0 {
			ownerCheckName = checkName
		}

		// Add ownership label - all checks are owned by the first check
		if check.Labels == nil {
			check.Labels = make(map[string]string)
		}
		check.Labels[k8upv1.LabelK8upOwnedBy] = k8upv1.CheckType.String() + "_" + ownerCheckName

		ts.EnsureResources(check)
		ts.GivenChecks = append(ts.GivenChecks, check)
		ts.CheckNames = append(ts.CheckNames, checkName)
	}
}

func (ts *CheckTestSuite) whenReconcile() (lastResult controllerruntime.Result) {
	for _, check := range ts.GivenChecks {
		controller := CheckReconciler{
			Kube: ts.Client,
		}

		result, err := controller.Provision(ts.Ctx, check)
		ts.Require().NoError(err)

		lastResult = result
	}

	return
}

func (ts *CheckTestSuite) expectNumberOfJobs(jobAmount int) {
	jobs := &batchv1.JobList{}
	err := ts.Client.List(ts.Ctx, jobs, &client.ListOptions{Namespace: ts.NS})
	ts.Require().NoError(err)

	jobsLen := len(jobs.Items)
	ts.T().Logf("%d Jobs found", jobsLen)

	ts.Assert().GreaterOrEqual(jobsLen, jobAmount)
}

func (ts *CheckTestSuite) Test_GivenCheckWithTlsOptions_ExpectCheckJobWithTlsOptions() {
	checkResource := ts.newCheckTls()
	ts.EnsureResources(checkResource)

	result := ts.whenReconciling(checkResource)
	ts.Require().GreaterOrEqual(result.RequeueAfter, 30*time.Second)

	checkJob := ts.expectACheckJob()
	ts.assertCheckTlsVolumeAndTlsOptions(checkJob)
}

func (ts *CheckTestSuite) Test_GivenCheckWithMutualTlsOptions_ExpectCheckJobWithMutualTlsOptions() {
	checkResource := ts.newCheckMutualTls()
	ts.EnsureResources(checkResource)

	result := ts.whenReconciling(checkResource)
	ts.Require().GreaterOrEqual(result.RequeueAfter, 30*time.Second)

	checkJob := ts.expectACheckJob()
	ts.assertCheckMutualTlsVolumeAndMutualTlsOptions(checkJob)
}
