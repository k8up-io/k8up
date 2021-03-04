// +build integration

package controllers_test

import (
	"context"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	k8upv1a1 "github.com/vshn/k8up/api/v1alpha1"
)

func (ts *BackupTestSuite) newPreBackupPod() *k8upv1a1.PreBackupPod {
	return &k8upv1a1.PreBackupPod{
		Spec: k8upv1a1.PreBackupPodSpec{
			BackupCommand: "/bin/true",
			Pod: &k8upv1a1.Pod{
				PodTemplateSpec: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:    "alpine",
								Image:   "alpine",
								Command: []string{"/bin/true"},
							},
						},
					},
				},
			},
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ts.PreBackupPodName,
			Namespace: ts.NS,
		},
	}
}

func (ts *BackupTestSuite) newBackup() *k8upv1a1.Backup {
	return &k8upv1a1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup",
			Namespace: ts.NS,
		},
		Spec: k8upv1a1.BackupSpec{
			RunnableSpec: k8upv1a1.RunnableSpec{},
		},
	}
}

func (ts *BackupTestSuite) whenReconciling(object metav1.Object) controllerruntime.Result {
	req := ts.MapToRequest(object)
	result, err := ts.Controller.Reconcile(ts.Ctx, req)
	ts.Require().NoError(err)

	return result
}

func (ts *BackupTestSuite) expectABackupJobEventually() {
	ts.RepeatedAssert(3*time.Second, time.Second, "Jobs not found", func(timedCtx context.Context) (done bool, err error) {
		jobs := new(batchv1.JobList)
		err = ts.Client.List(timedCtx, jobs, client.InNamespace(ts.NS))
		ts.Require().NoError(err)

		jobsLen := len(jobs.Items)
		ts.T().Logf("%d Jobs found", jobsLen)

		if jobsLen > 0 {
			assert.Len(ts.T(), jobs.Items, 1)
			return true, err
		}

		return
	})
}

func (ts *BackupTestSuite) expectAPreBackupDeploymentEventually() {
	ts.RepeatedAssert(5*time.Second, time.Second, "Deployments not found", func(timedCtx context.Context) (done bool, err error) {
		pod := ts.newPreBackupDeployment()
		if ts.IsResourceExisting(timedCtx, pod) {
			return true, nil
		}
		return false, nil
	})
}

func (ts *BackupTestSuite) whenCancellingTheContext() {
	ts.CancelCtx()
	ts.Ctx, ts.CancelCtx = context.WithCancel(context.Background())
}

func (ts *BackupTestSuite) afterDeploymentStarted() {
	deployment := &appsv1.Deployment{}
	deploymentIdentifier := types.NamespacedName{Namespace: ts.NS, Name: ts.PreBackupPodName}
	ts.FetchResource(deploymentIdentifier, deployment)

	deployment.Status.AvailableReplicas = 1
	deployment.Status.ReadyReplicas = 1
	deployment.Status.Replicas = 1

	ts.UpdateStatus(deployment)
}

func (ts *BackupTestSuite) expectABackupContainer() {
	ts.RepeatedAssert(5*time.Second, time.Second, "Backup not found", func(timedCtx context.Context) (done bool, err error) {
		backups := new(k8upv1a1.BackupList)
		err = ts.Client.List(timedCtx, backups, client.InNamespace(ts.NS))
		ts.Require().NoError(err)

		backupsLen := len(backups.Items)
		ts.T().Logf("%d Backups found", backupsLen)

		if backupsLen > 0 {
			ts.Assert().Len(backups.Items, 1)
			return true, err
		}

		return
	})
}

func (ts *BackupTestSuite) assertConditionWaitingForPreBackup(backup *k8upv1a1.Backup) {
	ts.RepeatedAssert(5*time.Second, time.Second, "backup does not have correct condition", func(timedCtx context.Context) (done bool, err error) {
		err = ts.Client.Get(timedCtx, k8upv1a1.MapToNamespacedName(backup), backup)
		ts.Require().NoError(err)
		preBackupCond := meta.FindStatusCondition(backup.Status.Conditions, k8upv1a1.ConditionPreBackupPodReady.String())
		if preBackupCond != nil {
			ts.Assert().Equal(k8upv1a1.ReasonWaiting.String(), preBackupCond.Reason)
			ts.Assert().Equal(metav1.ConditionUnknown, preBackupCond.Status)
			return true, nil
		}
		return false, nil
	})
}

func (ts *BackupTestSuite) assertConditionReadyForPreBackup(backup *k8upv1a1.Backup) {
	ts.RepeatedAssert(5*time.Second, time.Second, "backup does not have expected condition", func(timedCtx context.Context) (done bool, err error) {
		err = ts.Client.Get(timedCtx, k8upv1a1.MapToNamespacedName(backup), backup)
		ts.Require().NoError(err)
		preBackupCond := meta.FindStatusCondition(backup.Status.Conditions, k8upv1a1.ConditionPreBackupPodReady.String())
		if preBackupCond != nil && preBackupCond.Reason == k8upv1a1.ReasonReady.String() {
			ts.Assert().Equal(k8upv1a1.ReasonReady.String(), preBackupCond.Reason)
			ts.Assert().Equal(metav1.ConditionTrue, preBackupCond.Status)
			return true, nil
		}
		return false, nil
	})
}

func (ts *BackupTestSuite) assertConditionFailedBackup(backup *k8upv1a1.Backup) {
	ts.RepeatedAssert(5*time.Second, time.Second, "backup does not have expected condition", func(timedCtx context.Context) (done bool, err error) {
		err = ts.Client.Get(timedCtx, k8upv1a1.MapToNamespacedName(backup), backup)
		ts.Require().NoError(err)
		preBackupCond := meta.FindStatusCondition(backup.Status.Conditions, k8upv1a1.ConditionPreBackupPodReady.String())
		if preBackupCond != nil && preBackupCond.Reason == k8upv1a1.ReasonFailed.String() {
			ts.Assert().Equal(k8upv1a1.ReasonFailed.String(), preBackupCond.Reason)
			ts.Assert().Equal(metav1.ConditionFalse, preBackupCond.Status)
			return true, nil
		}
		return false, nil
	})
}

func (ts *BackupTestSuite) assertDeploymentIsDeleted(failedDeployment *appsv1.Deployment) {
	ts.RepeatedAssert(5*time.Second, time.Second, "deployment still exists, but it shouldn't", func(timedCtx context.Context) (done bool, err error) {
		if !ts.IsResourceExisting(timedCtx, failedDeployment) {
			return true, nil
		}
		return false, nil
	})
}

func (ts *BackupTestSuite) newPreBackupDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ts.NS,
			Name:      ts.PreBackupPodName,
		},
		Spec: appsv1.DeploymentSpec{
			ProgressDeadlineSeconds: pointer.Int32Ptr(30),
			Selector: metav1.SetAsLabelSelector(labels.Set{
				"key": "value",
			}),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels.Set{
						"key": "value",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "container", Image: "busybox"},
					},
				},
			},
		},
	}
}

func (ts *BackupTestSuite) markDeploymentAsFailed(deployment *appsv1.Deployment) {
	deployment.Status = appsv1.DeploymentStatus{
		Conditions: []appsv1.DeploymentCondition{
			{Type: "Progressing", Status: corev1.ConditionFalse, LastUpdateTime: metav1.Now(), Reason: "ProgressDeadlineExceeded", Message: "deployment failed"},
		},
	}
	ts.UpdateStatus(deployment)
}
