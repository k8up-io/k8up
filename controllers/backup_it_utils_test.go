//go:build integration

package controllers_test

import (
	"context"
	"fmt"
	"time"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
)

const (
	backupTag = "integrationTag"
)

func (ts *BackupTestSuite) newPreBackupPod() *k8upv1.PreBackupPod {
	return &k8upv1.PreBackupPod{
		Spec: k8upv1.PreBackupPodSpec{
			BackupCommand: "/bin/true",
			Pod: &k8upv1.Pod{
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

func (ts *BackupTestSuite) newBackup() *k8upv1.Backup {
	return &k8upv1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup",
			Namespace: ts.NS,
		},
		Spec: k8upv1.BackupSpec{
			RunnableSpec: k8upv1.RunnableSpec{},
		},
	}
}

func (ts *BackupTestSuite) newBackupWithSecurityContext() *k8upv1.Backup {
	runAsNonRoot := true
	sc := &corev1.PodSecurityContext{
		RunAsNonRoot: &runAsNonRoot,
	}

	backup := ts.newBackup()
	backup.Spec.PodSecurityContext = sc
	backup.Spec.ActiveDeadlineSeconds = pointer.Int64(500)
	return backup
}

func (ts *BackupTestSuite) whenReconciling(object metav1.Object) controllerruntime.Result {
	req := ts.MapToRequest(object)
	result, err := ts.Controller.Reconcile(ts.Ctx, req)
	ts.Require().NoError(err)

	return result
}

func (ts *BackupTestSuite) expectABackupJobEventually() (foundJob *batchv1.Job) {
	ts.RepeatedAssert(3*time.Second, time.Second, "Jobs not found", func(timedCtx context.Context) (done bool, err error) {
		jobs := new(batchv1.JobList)
		err = ts.Client.List(timedCtx, jobs, client.InNamespace(ts.NS))
		ts.Require().NoError(err)

		jobsLen := len(jobs.Items)
		ts.T().Logf("%d Jobs found", jobsLen)

		if jobsLen > 0 {
			assert.Len(ts.T(), jobs.Items, 1)
			foundJob = &jobs.Items[0]
			return true, err
		}

		return false, err
	})

	ts.Require().NotNil(foundJob)
	return foundJob
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

func (ts *BackupTestSuite) afterPreBackupDeploymentStarted() {
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
		backups := new(k8upv1.BackupList)
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

func (ts *BackupTestSuite) assertConditionWaitingForPreBackup(backup *k8upv1.Backup) {
	ts.RepeatedAssert(5*time.Second, time.Second, "backup does not have correct condition", func(timedCtx context.Context) (done bool, err error) {
		err = ts.Client.Get(timedCtx, k8upv1.MapToNamespacedName(backup), backup)
		ts.Require().NoError(err)
		preBackupCond := meta.FindStatusCondition(backup.Status.Conditions, k8upv1.ConditionPreBackupPodReady.String())
		if preBackupCond != nil {
			ts.Assert().Equal(k8upv1.ReasonWaiting.String(), preBackupCond.Reason)
			ts.Assert().Equal(metav1.ConditionUnknown, preBackupCond.Status)
			return true, nil
		}
		return false, nil
	})
}

func (ts *BackupTestSuite) assertConditionReadyForPreBackup(backup *k8upv1.Backup) {
	ts.RepeatedAssert(5*time.Second, time.Second, "backup does not have expected condition", func(timedCtx context.Context) (done bool, err error) {
		err = ts.Client.Get(timedCtx, k8upv1.MapToNamespacedName(backup), backup)
		ts.Require().NoError(err)
		preBackupCond := meta.FindStatusCondition(backup.Status.Conditions, k8upv1.ConditionPreBackupPodReady.String())
		if preBackupCond != nil && preBackupCond.Reason == k8upv1.ReasonReady.String() {
			ts.Assert().Equal(k8upv1.ReasonReady.String(), preBackupCond.Reason)
			ts.Assert().Equal(metav1.ConditionTrue, preBackupCond.Status)
			return true, nil
		}
		return false, nil
	})
}

func (ts *BackupTestSuite) assertPreBackupPodConditionFailed(backup *k8upv1.Backup) {
	ts.RepeatedAssert(5*time.Second, time.Second, "backup does not have expected condition", func(timedCtx context.Context) (done bool, err error) {
		err = ts.Client.Get(timedCtx, k8upv1.MapToNamespacedName(backup), backup)
		ts.Require().NoError(err)
		preBackupCond := meta.FindStatusCondition(backup.Status.Conditions, k8upv1.ConditionPreBackupPodReady.String())
		if preBackupCond != nil && preBackupCond.Reason == k8upv1.ReasonFailed.String() {
			ts.Assert().Equal(k8upv1.ReasonFailed.String(), preBackupCond.Reason)
			ts.Assert().Equal(metav1.ConditionFalse, preBackupCond.Status)
			return true, nil
		}
		return false, nil
	})
}

func (ts *BackupTestSuite) assertPreBackupPodConditionSucceeded(backup *k8upv1.Backup) {
	ts.RepeatedAssert(5*time.Second, time.Second, "backup does not have expected condition", func(timedCtx context.Context) (done bool, err error) {
		err = ts.Client.Get(timedCtx, k8upv1.MapToNamespacedName(backup), backup)
		ts.Require().NoError(err)
		preBackupCond := meta.FindStatusCondition(backup.Status.Conditions, k8upv1.ConditionPreBackupPodReady.String())
		if preBackupCond != nil && preBackupCond.Reason == k8upv1.ReasonFailed.String() {
			ts.Assert().Equal(k8upv1.ReasonSucceeded.String(), preBackupCond.Reason)
			ts.Assert().Equal(metav1.ConditionFalse, preBackupCond.Status)
			return true, nil
		}
		return false, nil
	})
}

func (ts *BackupTestSuite) assertDeploymentIsDeleted(deployment *appsv1.Deployment) {
	ts.RepeatedAssert(
		5*time.Second, time.Second,
		fmt.Sprintf("deployment '%s/%s' still exists, but it shouldn't", deployment.Namespace, deployment.Name),
		func(timedCtx context.Context) (done bool, err error) {
			isDeleted := !ts.IsResourceExisting(timedCtx, deployment)
			return isDeleted, nil
		},
	)
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

func (ts *BackupTestSuite) markDeploymentAsFinished(deployment *appsv1.Deployment) {
	deployment.Status = appsv1.DeploymentStatus{
		Conditions: []appsv1.DeploymentCondition{
			{Type: "Progressing", Status: corev1.ConditionTrue, LastUpdateTime: metav1.Now(), Reason: "NewReplicaSetAvailable", Message: "deployment successful"},
			{Type: "Available", Status: corev1.ConditionTrue, LastUpdateTime: metav1.Now(), Reason: "MinimumReplicasAvailable", Message: "deployment successful"},
		},
	}
	ts.UpdateStatus(deployment)
}

func (ts *BackupTestSuite) markBackupAsFinished(backup *k8upv1.Backup) {
	backup.Status = k8upv1.Status{
		Started:  true,
		Finished: true,
		Conditions: []metav1.Condition{
			{Type: "PreBackupPodReady", Status: metav1.ConditionTrue, LastTransitionTime: metav1.Now(), Reason: "Ready", Message: "backup successful"},
			{Type: "Ready", Status: metav1.ConditionTrue, LastTransitionTime: metav1.Now(), Reason: "Ready", Message: "backup successful"},
			{Type: "Progressing", Status: metav1.ConditionFalse, LastTransitionTime: metav1.Now(), Reason: "Finished", Message: "backup successful"},
			{Type: "Completed", Status: metav1.ConditionTrue, LastTransitionTime: metav1.Now(), Reason: "Succeeded", Message: "backup successful"},
		},
	}
}

func (ts *BackupTestSuite) newBackupWithTags() *k8upv1.Backup {
	backupWithTags := ts.newBackup()
	backupWithTags.Spec.Tags = []string{backupTag}
	return backupWithTags
}

func (ts *BackupTestSuite) assertJobHasTagArguments(job *batchv1.Job) {
	jobArguments := job.Spec.Template.Spec.Containers[0].Args
	ts.Assert().Contains(jobArguments, backupTag, "backup tag in job args")
}
