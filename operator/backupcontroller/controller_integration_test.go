//go:build integration

package backupcontroller

import (
	"context"
	"testing"
	"time"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/envtest"
	"github.com/stretchr/testify/suite"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
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
		Kube: ts.Client,
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

func (ts *BackupTestSuite) Test_GivenBackup_AndJob_KeepBackupProgressing() {
	backupJob := ts.newJob(ts.BackupResource)
	ts.EnsureResources(ts.BackupResource, backupJob)
	ts.BackupResource.Status.Started = true
	backupJob.Status.Active = 1
	ts.UpdateStatus(ts.BackupResource, backupJob)

	ts.whenReconciling(ts.BackupResource)

	result := &k8upv1.Backup{}
	err := ts.Client.Get(ts.Ctx, k8upv1.MapToNamespacedName(ts.BackupResource), result)
	ts.Require().NoError(err)
	ts.assertCondition(result.Status.Conditions, k8upv1.ConditionProgressing, k8upv1.ReasonStarted, metav1.ConditionTrue)
	ts.assertCondition(result.Status.Conditions, k8upv1.ConditionReady, k8upv1.ReasonReady, metav1.ConditionTrue)
	ts.Assert().Len(result.Status.Conditions, 2, "amount of conditions")
}

func (ts *BackupTestSuite) Test_GivenBackup_AndCompletedJob_ThenCompleteBackup() {
	backupJob := ts.newJob(ts.BackupResource)
	ts.EnsureResources(ts.BackupResource, backupJob)
	ts.BackupResource.Status.Started = true
	backupJob.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
	ts.UpdateStatus(ts.BackupResource, backupJob)

	ts.whenReconciling(ts.BackupResource)

	result := &k8upv1.Backup{}
	ts.FetchResource(types.NamespacedName{Namespace: ts.BackupResource.Namespace, Name: ts.BackupResource.Name}, result)

	ts.assertCondition(result.Status.Conditions, k8upv1.ConditionCompleted, k8upv1.ReasonSucceeded, metav1.ConditionTrue)
	ts.assertCondition(result.Status.Conditions, k8upv1.ConditionProgressing, k8upv1.ReasonFinished, metav1.ConditionFalse)
	ts.Assert().Len(result.Status.Conditions, 4, "amount of conditions")
}

func (ts *BackupTestSuite) Test_GivenBackup_AndFailedJob_ThenCompleteBackup() {
	backupJob := ts.newJob(ts.BackupResource)
	ts.EnsureResources(ts.BackupResource, backupJob)
	ts.BackupResource.Status.Started = true
	backupJob.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue}}
	ts.UpdateStatus(ts.BackupResource, backupJob)

	ts.whenReconciling(ts.BackupResource)

	result := &k8upv1.Backup{}
	ts.FetchResource(types.NamespacedName{Namespace: ts.BackupResource.Namespace, Name: ts.BackupResource.Name}, result)
	ts.assertCondition(result.Status.Conditions, k8upv1.ConditionCompleted, k8upv1.ReasonFailed, metav1.ConditionTrue)
	ts.assertCondition(result.Status.Conditions, k8upv1.ConditionProgressing, k8upv1.ReasonFinished, metav1.ConditionFalse)
	ts.Assert().Len(result.Status.Conditions, 4, "amount of conditions")
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

func (ts *BackupTestSuite) Test_GivenPreBackupDeployment_WhenDeploymentIsReady_ThenExpectPreBackupConditionReady() {
	deployment := ts.newPreBackupDeployment()
	ts.EnsureResources(ts.BackupResource, ts.newPreBackupPod(), deployment)
	ts.afterPreBackupDeploymentStarted()

	result := ts.whenReconciling(ts.BackupResource)
	ts.Assert().GreaterOrEqual(result.RequeueAfter, 30*time.Second)
	ts.assertPreBackupPodConditionReady(ts.BackupResource)
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
	preBackupDeployment := ts.newPreBackupDeployment()
	ts.EnsureResources(ts.newPreBackupPod(), ts.BackupResource, preBackupDeployment)
	ts.markBackupAsFinished(ts.BackupResource)
	ts.UpdateStatus(ts.BackupResource)
	_ = ts.whenReconciling(ts.BackupResource)

	result := &k8upv1.Backup{}
	ts.FetchResource(types.NamespacedName{Name: ts.BackupResource.Name, Namespace: ts.BackupResource.Namespace}, result)

	_ = ts.whenReconciling(result)

	ts.Assert().False(ts.IsResourceExisting(ts.Ctx, preBackupDeployment))
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

func (ts *BackupTestSuite) Test_GivenFailedPreBackup_WhenReconciling_ThenIgnore() {
	ts.EnsureResources(ts.BackupResource, ts.newPreBackupPod())
	ts.SetCondition(ts.BackupResource, &ts.BackupResource.Status.Conditions,
		k8upv1.ConditionPreBackupPodReady, metav1.ConditionFalse, k8upv1.ReasonFailed)

	result := ts.whenReconciling(ts.BackupResource)
	ts.Assert().Equal(0.0, result.RequeueAfter.Seconds())
	ts.Assert().False(result.Requeue)
	ts.assertPreBackupPodConditionFailed(ts.BackupResource) // should stay failed
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

func (ts *BackupTestSuite) assertCondition(conditions []metav1.Condition, condType k8upv1.ConditionType, reason k8upv1.ConditionReason, status metav1.ConditionStatus) {
	cond := meta.FindStatusCondition(conditions, condType.String())
	ts.Require().NotNil(cond, "condition of type %s missing", condType)
	ts.Assert().Equal(reason.String(), cond.Reason, "condition %s doesn't contain reason %s", condType, reason)
	ts.Assert().Equal(status, cond.Status, "condition %s isn't %s", condType, status)
}
