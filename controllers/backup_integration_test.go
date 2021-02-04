// +build integration

package controllers_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1a1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/controllers"
)

type BackupTestSuite struct {
	EnvTestSuite

	BackupName              string
	DeploymentPodName       string
	DeploymentContainerName string
	PreBackupPodName        string
	CancelCtx               context.CancelFunc
	BackupResource          *k8upv1a1.Backup
	Controller              controllers.BackupReconciler
}

func Test_Backup(t *testing.T) {
	suite.Run(t, new(BackupTestSuite))
}

func (r *BackupTestSuite) BeforeTest(_, _ string) {
	r.Controller = controllers.BackupReconciler{
		Client: r.Client,
		Log:    r.Logger,
		Scheme: r.Scheme,
	}
	r.BackupName = "backup"
	r.DeploymentPodName = "pre-backup-deployment-pod"
	r.DeploymentContainerName = "pre-backup-deployment-pod-container"
	r.PreBackupPodName = "pre-backup-pod"
	r.Ctx, r.CancelCtx = context.WithCancel(context.Background())
	r.BackupResource = r.newBackup()
}

func (r *BackupTestSuite) Test_GivenBackup_ExpectDeployment() {
	r.EnsureResources(r.BackupResource)
	result := r.whenReconciling(r.BackupResource)
	assert.GreaterOrEqual(r.T(), result.RequeueAfter, 30*time.Second)

	r.expectABackupJobEventually()
}

func (r *BackupTestSuite) Test_GivenPreBackupPods_ExpectDeployment() {
	r.EnsureResources(r.BackupResource, r.newPreBackupPod())

	result := r.whenReconciling(r.BackupResource)
	assert.GreaterOrEqual(r.T(), result.RequeueAfter, 30*time.Second)
	r.expectADeploymentEventually()

	r.afterDeploymentStarted()
	_ = r.whenReconciling(r.BackupResource)
	r.expectABackupContainer()
}

func (r *BackupTestSuite) Test_GivenPreBackupPods_WhenRestarting() {
	r.EnsureResources(r.BackupResource, r.newPreBackupPod())

	_ = r.whenReconciling(r.BackupResource)
	r.expectADeploymentEventually()

	r.whenCancellingTheContext()
	r.afterDeploymentStarted()
	_ = r.whenReconciling(r.BackupResource)
	r.expectABackupContainer()
}

func (r *BackupTestSuite) newPreBackupPod() *k8upv1a1.PreBackupPod {
	return &k8upv1a1.PreBackupPod{
		Spec: k8upv1a1.PreBackupPodSpec{
			BackupCommand: "/bin/true",
			Pod: &k8upv1a1.Pod{
				PodTemplateSpec: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name:      r.DeploymentPodName,
						Namespace: r.NS,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:    r.DeploymentContainerName,
								Image:   "alpine",
								Command: []string{"/bin/true"},
							},
						},
					},
				},
			},
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.PreBackupPodName,
			Namespace: r.NS,
		},
	}
}

func (r *BackupTestSuite) newBackup() *k8upv1a1.Backup {
	return &k8upv1a1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.BackupName,
			Namespace: r.NS,
		},
		Spec: k8upv1a1.BackupSpec{
			RunnableSpec: k8upv1a1.RunnableSpec{},
		},
	}
}

func (r *BackupTestSuite) whenReconciling(object metav1.Object) controllerruntime.Result {
	result, err := r.Controller.Reconcile(r.Ctx, r.MapToRequest(object))
	require.NoError(r.T(), err)

	return result
}

func (r *BackupTestSuite) expectABackupJobEventually() {
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

func (r *BackupTestSuite) expectADeploymentEventually() {
	r.RepeatedAssert(5*time.Second, time.Second, "Deployments not found", func(timedCtx context.Context) (done bool, err error) {
		deployments := new(appsv1.DeploymentList)
		err = r.Client.List(timedCtx, deployments, &client.ListOptions{Namespace: r.NS})
		require.NoError(r.T(), err)

		jobsLen := len(deployments.Items)
		r.T().Logf("%d Deployments found", jobsLen)

		if jobsLen > 0 {
			assert.Equal(r.T(), jobsLen, 1)
			return true, err
		}

		return
	})
}

func (r *BackupTestSuite) whenCancellingTheContext() {
	r.CancelCtx()
	r.Ctx, r.CancelCtx = context.WithCancel(context.Background())
}

func (r *BackupTestSuite) afterDeploymentStarted() {
	deployment := new(appsv1.Deployment)
	deploymentIdentifier := types.NamespacedName{Namespace: r.NS, Name: r.PreBackupPodName}
	r.FetchResource(deploymentIdentifier, deployment)

	deployment.Status.AvailableReplicas = 1

	r.UpdateResources(deployment)
}

func (r *BackupTestSuite) expectABackupContainer() {
	r.RepeatedAssert(5*time.Second, time.Second, "Backup not found", func(timedCtx context.Context) (done bool, err error) {
		backups := new(k8upv1a1.BackupList)
		err = r.Client.List(timedCtx, backups, &client.ListOptions{Namespace: r.NS})
		require.NoError(r.T(), err)

		backupsLen := len(backups.Items)
		r.T().Logf("%d Backups found", backupsLen)

		if backupsLen > 0 {
			assert.Equal(r.T(), backupsLen, 1)
			return true, err
		}

		return
	})
}
