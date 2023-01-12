//go:build integration

package jobcontroller

import (
	"context"
	"testing"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/envtest"
	"github.com/stretchr/testify/suite"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type JobTestSuite struct {
	envtest.Suite

	CancelCtx  context.CancelFunc
	Controller JobReconciler
}

func Test_Backup(t *testing.T) {
	suite.Run(t, new(JobTestSuite))
}

func (ts *JobTestSuite) BeforeTest(_, _ string) {
	ts.Controller = JobReconciler{
		Kube: ts.Client,
	}
	ts.Ctx, ts.CancelCtx = context.WithCancel(context.Background())
}

func (ts *JobTestSuite) Test_GivenRunningJob_ThenKeepBackupProgressing() {
	// Arrange
	backup := ts.newBackup()
	backupJob := ts.newJob(backup)
	ts.EnsureResources(backup, backupJob)
	backup.Status.Started = true
	backupJob.Status.Active = 1
	ts.UpdateStatus(backup, backupJob)

	// Act
	ts.whenReconciling(backupJob)

	// Assert
	result := &k8upv1.Backup{}
	ts.FetchResource(types.NamespacedName{Namespace: backup.Namespace, Name: backup.Name}, result)

	ts.assertCondition(result.Status.Conditions, k8upv1.ConditionProgressing, k8upv1.ReasonStarted, metav1.ConditionTrue)
	ts.assertCondition(result.Status.Conditions, k8upv1.ConditionReady, k8upv1.ReasonReady, metav1.ConditionTrue)
	ts.Assert().Len(result.Status.Conditions, 2, "amount of conditions")
}

func (ts *JobTestSuite) Test_GivenCompletedJob_ThenCompleteBackup() {
	// Arrange
	backup := ts.newBackup()
	backupJob := ts.newJob(backup)
	ts.EnsureResources(backup, backupJob)
	backup.Status.Started = true
	backupJob.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
	ts.UpdateStatus(backup, backupJob)

	// Act
	ts.whenReconciling(backupJob)

	// Assert
	result := &k8upv1.Backup{}
	ts.FetchResource(types.NamespacedName{Namespace: backup.Namespace, Name: backup.Name}, result)

	ts.assertCondition(result.Status.Conditions, k8upv1.ConditionCompleted, k8upv1.ReasonSucceeded, metav1.ConditionTrue)
	ts.assertCondition(result.Status.Conditions, k8upv1.ConditionProgressing, k8upv1.ReasonFinished, metav1.ConditionFalse)
	ts.Assert().Len(result.Status.Conditions, 2, "amount of conditions")
}

func (ts *JobTestSuite) Test_GivenFailedJob_ThenCompleteBackup() {
	// Arrange
	backup := ts.newBackup()
	backupJob := ts.newJob(backup)
	ts.EnsureResources(backup, backupJob)
	backup.Status.Started = true
	backupJob.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobFailed, Status: corev1.ConditionTrue}}
	ts.UpdateStatus(backup, backupJob)

	// Act
	ts.whenReconciling(backupJob)

	// Assert
	result := &k8upv1.Backup{}
	ts.FetchResource(types.NamespacedName{Namespace: backup.Namespace, Name: backup.Name}, result)
	ts.assertCondition(result.Status.Conditions, k8upv1.ConditionCompleted, k8upv1.ReasonFailed, metav1.ConditionTrue)
	ts.assertCondition(result.Status.Conditions, k8upv1.ConditionProgressing, k8upv1.ReasonFinished, metav1.ConditionFalse)
	ts.Assert().Len(result.Status.Conditions, 2, "amount of conditions")
}

func (ts *JobTestSuite) assertCondition(conditions []metav1.Condition, condType k8upv1.ConditionType, reason k8upv1.ConditionReason, status metav1.ConditionStatus) {
	cond := meta.FindStatusCondition(conditions, condType.String())
	ts.Require().NotNil(cond, "condition of type %s missing", condType)
	ts.Assert().Equal(reason.String(), cond.Reason, "condition %s doesn't contain reason %s", condType, reason)
	ts.Assert().Equal(status, cond.Status, "condition %s isn't %s", condType, status)
}

func (ts *JobTestSuite) whenReconciling(object *batchv1.Job) controllerruntime.Result {
	result, err := ts.Controller.Provision(ts.Ctx, object)
	ts.Require().NoError(err)

	return result
}

func (ts *JobTestSuite) newBackup() *k8upv1.Backup {
	obj := &k8upv1.Backup{}
	obj.Name = "backup"
	obj.Namespace = ts.NS
	obj.UID = uuid.NewUUID()
	return obj
}

func (ts *JobTestSuite) newJob(owner client.Object) *batchv1.Job {
	jb := &batchv1.Job{}
	jb.Name = "backup-job"
	jb.Namespace = ts.NS
	jb.Labels = labels.Set{k8upv1.LabelK8upType: k8upv1.BackupType.String()}
	jb.Spec.Template.Spec.Containers = []corev1.Container{{Name: "container", Image: "image"}}
	jb.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyOnFailure
	ts.Assert().NoError(controllerruntime.SetControllerReference(owner, jb, ts.Scheme), "set controller ref")
	return jb
}
