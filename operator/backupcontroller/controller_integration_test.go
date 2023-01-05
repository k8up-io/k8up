//go:build integration

package backupcontroller

import (
	"context"
	"testing"
	"time"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/envtest"
	"github.com/k8up-io/k8up/v2/operator/locker"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type BackupTestSuite struct {
	envtest.Suite

	PreBackupPodName string
	CancelCtx        context.CancelFunc
	BackupResource   *k8upv1.Backup
	Controller       BackupReconciler
}

func Test_Backup(t *testing.T) {
	suite.Run(t, new(BackupTestSuite))
}

func (ts *BackupTestSuite) BeforeTest(_, _ string) {
	ts.Controller = BackupReconciler{
		Kube:   ts.Client,
		Locker: &locker.Locker{Kube: ts.Client},
	}
	ts.PreBackupPodName = "pre-backup-pod"
	ts.Ctx, ts.CancelCtx = context.WithCancel(context.Background())
	ts.BackupResource = ts.newBackup()
}

func (ts *BackupTestSuite) Test_GivenBackup_ExpectBackupJob() {
	ts.EnsureResources(ts.BackupResource)
	result := ts.whenReconciling(ts.BackupResource)
	ts.Assert().GreaterOrEqual(result.RequeueAfter, 30*time.Second)

	ts.expectABackupJob()
}

func (ts *BackupTestSuite) Test_GivenBackupWithSecurityContext_ExpectBackupJobWithSecurityContext() {
	ts.BackupResource = ts.newBackupWithSecurityContext()
	ts.EnsureResources(ts.BackupResource)
	result := ts.whenReconciling(ts.BackupResource)
	ts.Require().GreaterOrEqual(result.RequeueAfter, 30*time.Second)

	backupJob := ts.expectABackupJob()
	ts.Assert().NotNil(backupJob.Spec.Template.Spec.SecurityContext)
	ts.Assert().Equal(*ts.BackupResource.Spec.PodSecurityContext, *backupJob.Spec.Template.Spec.SecurityContext)
	ts.Assert().Equal(int64(500), *backupJob.Spec.ActiveDeadlineSeconds)
}

func (ts *BackupTestSuite) Test_GivenPreBackupPods_ExpectPreBackupDeployment() {
	ts.EnsureResources(ts.BackupResource, ts.newPreBackupPod())

	result := ts.whenReconciling(ts.BackupResource)
	ts.Assert().GreaterOrEqual(result.RequeueAfter, 30*time.Second)
	ts.assertPrebackupDeploymentExists()
	ts.assertConditionWaitingForPreBackup(ts.BackupResource)

	ts.afterPreBackupDeploymentStarted()
	_ = ts.whenReconciling(ts.BackupResource)
	ts.assertConditionReadyForPreBackup(ts.BackupResource)
	ts.assertBackupExists()
}

func (ts *BackupTestSuite) Test_GivenPreBackupDeployment_WhenDeploymentStartsUp_ThenExpectBackupToBeWaiting() {
	deployment := ts.newPreBackupDeployment()
	ts.EnsureResources(ts.BackupResource, ts.newPreBackupPod(), deployment)

	result := ts.whenReconciling(ts.BackupResource)
	ts.Assert().GreaterOrEqual(result.RequeueAfter, 30*time.Second)
	ts.assertConditionWaitingForPreBackup(ts.BackupResource)
}

func (ts *BackupTestSuite) Test_GivenFailedPreBackupDeployment_WhenCreatingNewBackup_ThenExpectPreBackupToBeRemoved() {
	failedDeployment := ts.newPreBackupDeployment()
	ts.EnsureResources(failedDeployment, ts.newPreBackupPod(), ts.BackupResource)
	ts.markDeploymentAsFailed(failedDeployment)

	result := ts.whenReconciling(ts.BackupResource)
	ts.Assert().GreaterOrEqual(result.RequeueAfter, 30*time.Second)
	ts.Assert().False(ts.IsResourceExisting(ts.Ctx, failedDeployment))
	ts.assertPreBackupPodConditionFailed(ts.BackupResource)
}

func (ts *BackupTestSuite) Test_GivenFinishedPreBackupDeployment_WhenReconciling_ThenExpectPreBackupToBeRemoved() {
	// The backup reconciliation loop must run in order to start a backup,
	// so that the callback is registered which eventually cleans up the PreBackupPods.
	ts.EnsureResources(ts.newPreBackupPod(), ts.BackupResource)
	_ = ts.whenReconciling(ts.BackupResource)
	ts.assertPrebackupDeploymentExists()

	ts.afterPreBackupDeploymentStarted()
	_ = ts.whenReconciling(ts.BackupResource)
	ts.expectABackupJob()

	result := &k8upv1.Backup{}
	ts.FetchResource(types.NamespacedName{Name: ts.BackupResource.Name, Namespace: ts.BackupResource.Namespace}, result)
	ts.markBackupAsFinished(result)
	ts.UpdateStatus(result)

	_ = ts.whenReconciling(result)

	ts.Assert().False(ts.IsResourceExisting(ts.Ctx, ts.newPreBackupDeployment()))
}

func (ts *BackupTestSuite) Test_GivenPreBackupPods_WhenRestartingK8up_ThenExpectToContinueWhereItLeftOff() {
	ts.EnsureResources(ts.BackupResource, ts.newPreBackupPod())

	_ = ts.whenReconciling(ts.BackupResource)
	ts.assertPrebackupDeploymentExists()

	ts.whenCancellingTheContext()
	ts.afterPreBackupDeploymentStarted()
	_ = ts.whenReconciling(ts.BackupResource)
	ts.assertBackupExists()
}

func (ts *BackupTestSuite) Test_GivenFinishedBackup_WhenReconciling_ThenIgnore() {
	ts.EnsureResources(ts.BackupResource)
	ts.SetCondition(ts.BackupResource, &ts.BackupResource.Status.Conditions,
		k8upv1.ConditionCompleted, metav1.ConditionTrue, k8upv1.ReasonSucceeded)
	ts.SetCondition(ts.BackupResource, &ts.BackupResource.Status.Conditions,
		k8upv1.ConditionPreBackupPodReady, metav1.ConditionFalse, k8upv1.ReasonFinished)

	result := ts.whenReconciling(ts.BackupResource)
	ts.Assert().Equal(float64(0), result.RequeueAfter.Seconds())
}

func (ts *BackupTestSuite) Test_GivenFailedBackup_WhenReconciling_ThenIgnore() {
	ts.EnsureResources(ts.BackupResource)
	ts.SetCondition(ts.BackupResource, &ts.BackupResource.Status.Conditions,
		k8upv1.ConditionPreBackupPodReady, metav1.ConditionFalse, k8upv1.ReasonFailed)
	ts.SetCondition(ts.BackupResource, &ts.BackupResource.Status.Conditions,
		k8upv1.ConditionCompleted, metav1.ConditionTrue, k8upv1.ReasonFailed)

	result := ts.whenReconciling(ts.BackupResource)
	ts.Assert().Equal(float64(0), result.RequeueAfter.Seconds())
}

func (ts *BackupTestSuite) Test_GivenBackupWithTags_WhenCreatingBackupjob_ThenHaveTagArguments() {
	ts.BackupResource = ts.newBackupWithTags()
	ts.EnsureResources(ts.BackupResource)
	ts.whenReconciling(ts.BackupResource)
	backupJob := ts.expectABackupJob()
	ts.assertJobHasTagArguments(backupJob)
}
