//go:build integration

package backupcontroller

import (
	"context"
	"strings"
	"testing"
	"time"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/envtest"
	"github.com/stretchr/testify/suite"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	pvc := ts.newPvc("test-pvc", corev1.ReadWriteMany)
	ts.EnsureResources(ts.BackupResource, pvc)

	pvc.Status.Phase = corev1.ClaimBound
	ts.UpdateStatus(pvc)

	result := ts.whenReconciling(ts.BackupResource)
	ts.Assert().GreaterOrEqual(result.RequeueAfter, 30*time.Second)

	ts.expectABackupJob()
}

func (ts *BackupTestSuite) Test_GivenBackup_AndJob_KeepBackupProgressing() {
	backupJob := ts.newJob(ts.BackupResource)
	pvc := ts.newPvc("test-pvc", corev1.ReadWriteMany)
	ts.EnsureResources(ts.BackupResource, backupJob, pvc)

	pvc.Status.Phase = corev1.ClaimBound
	ts.UpdateStatus(pvc)

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
	pvc := ts.newPvc("test-pvc", corev1.ReadWriteMany)
	ts.EnsureResources(ts.BackupResource, backupJob, pvc)

	pvc.Status.Phase = corev1.ClaimBound
	ts.UpdateStatus(pvc)

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

	pvc := ts.newPvc("test-pvc", corev1.ReadWriteMany)
	ts.EnsureResources(ts.BackupResource, backupJob, pvc)

	pvc.Status.Phase = corev1.ClaimBound
	ts.UpdateStatus(pvc)

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
	pvc := ts.newPvc("test-pvc", corev1.ReadWriteMany)
	ts.EnsureResources(ts.BackupResource, pvc)

	pvc.Status.Phase = corev1.ClaimBound
	ts.UpdateStatus(pvc)

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
	pvc := ts.newPvc("test-pvc", corev1.ReadWriteMany)
	ts.EnsureResources(ts.BackupResource, pvc)

	pvc.Status.Phase = corev1.ClaimBound
	ts.UpdateStatus(pvc)

	ts.whenReconciling(ts.BackupResource)
	backupJob := ts.expectABackupJob()
	ts.assertJobHasTagArguments(backupJob)
}

func (ts *BackupTestSuite) Test_GivenBackupAndMountedRWOPVCOnOneNode_ExpectBackupOnOneNode() {
	pvc1 := ts.newPvc("test-pvc1", corev1.ReadWriteOnce)
	pvc2 := ts.newPvc("test-pvc2", corev1.ReadWriteOnce)
	nodeName := "worker"
	tolerations := make([]corev1.Toleration, 0)
	volumePvc1 := corev1.Volume{
		Name: "test-pvc1",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvc1.Name,
			},
		},
	}
	pod1 := ts.newPod("test-pod1", nodeName, tolerations, []corev1.Volume{volumePvc1})
	volumePvc2 := corev1.Volume{
		Name: "test-pvc2",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvc2.Name,
			},
		},
	}
	pod2 := ts.newPod("test-pod2", nodeName, tolerations, []corev1.Volume{volumePvc2})
	ts.EnsureResources(ts.BackupResource, pvc1, pvc2, pod1, pod2)

	pvc1.Status.Phase = corev1.ClaimBound
	pvc2.Status.Phase = corev1.ClaimBound
	ts.UpdateStatus(pvc1, pvc2)

	result := ts.whenReconciling(ts.BackupResource)
	ts.Assert().GreaterOrEqual(result.RequeueAfter, 30*time.Second)

	job := ts.expectABackupJob()
	ts.assertJobSpecs(job, nodeName, []corev1.Volume{volumePvc1, volumePvc2}, tolerations, []string{pod1.Name, pod2.Name})
}

func (ts *BackupTestSuite) Test_GivenBackupAndMountedRWOPVCOnTwoNodes_ExpectBackupOnTwoNodes() {
	pvc1 := ts.newPvc("test-pvc1", corev1.ReadWriteOnce)
	pvc2 := ts.newPvc("test-pvc2", corev1.ReadWriteOnce)
	nodeNamePod1 := "worker"
	nodeNamePod2 := "control-plane"
	tolerationsPod1 := make([]corev1.Toleration, 0)
	tolerationsPod2 := []corev1.Toleration{
		{
			Key:    "node-role.kubernetes.io/control-plane",
			Effect: corev1.TaintEffectNoSchedule,
		},
		{
			Key:    "node-role.kubernetes.io/master",
			Effect: corev1.TaintEffectNoSchedule,
		},
	}
	volumePvc1 := corev1.Volume{
		Name: "test-pvc1",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvc1.Name,
			},
		},
	}
	pod1 := ts.newPod("test-pod1", nodeNamePod1, tolerationsPod1, []corev1.Volume{volumePvc1})
	volumePvc2 := corev1.Volume{
		Name: "test-pvc2",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvc2.Name,
			},
		},
	}
	pod2 := ts.newPod("test-pod2", nodeNamePod2, tolerationsPod2, []corev1.Volume{volumePvc2})
	ts.EnsureResources(ts.BackupResource, pvc1, pvc2, pod1, pod2)

	pvc1.Status.Phase = corev1.ClaimBound
	pvc2.Status.Phase = corev1.ClaimBound
	ts.UpdateStatus(pvc1, pvc2)

	result := ts.whenReconciling(ts.BackupResource)
	ts.Assert().GreaterOrEqual(result.RequeueAfter, 30*time.Second)

	jobs := new(batchv1.JobList)
	err := ts.Client.List(ts.Ctx, jobs, client.InNamespace(ts.NS))
	ts.Require().NoError(err)
	ts.Assert().Len(jobs.Items, 2)

	job1 := jobs.Items[0]
	job2 := jobs.Items[1]
	ts.assertJobSpecs(&job1, nodeNamePod1, []corev1.Volume{volumePvc1}, tolerationsPod1, []string{pod1.Name})
	ts.assertJobSpecs(&job2, nodeNamePod2, []corev1.Volume{volumePvc2}, tolerationsPod2, []string{pod2.Name})
}

func (ts *BackupTestSuite) Test_GivenBackupAndMountedRWOPVCOnOneNodeWithFinishedBackupPod_ExpectTargetNodeExcludesBackup() {
	pvc1 := ts.newPvc("test-pvc1", corev1.ReadWriteOnce)
	nodeName := "worker"
	tolerations := make([]corev1.Toleration, 0)
	volumePvc1 := corev1.Volume{
		Name: "test-pvc1",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvc1.Name,
			},
		},
	}
	pod1 := ts.newPod("test-pod1", nodeName, tolerations, []corev1.Volume{volumePvc1})
	pod2 := ts.newPod("test-pod2", nodeName, tolerations, []corev1.Volume{volumePvc1})
	pod2.Labels = labels.Set{
		"k8upjob": "true",
	}
	ts.EnsureResources(ts.BackupResource, pvc1, pod1, pod2)

	pvc1.Status.Phase = corev1.ClaimBound
	ts.UpdateStatus(pvc1)

	result := ts.whenReconciling(ts.BackupResource)
	ts.Assert().GreaterOrEqual(result.RequeueAfter, 30*time.Second)

	job := ts.expectABackupJob()
	ts.assertJobSpecs(job, nodeName, []corev1.Volume{volumePvc1}, tolerations, []string{pod1.Name})
}

func (ts *BackupTestSuite) Test_GivenBackupAndUnmountedRWOPVCOnTwoNodes_ExpectBackupOnTwoNodes() {
	pvc1 := ts.newPvc("test-pvc1", corev1.ReadWriteOnce)
	pvc2 := ts.newPvc("test-pvc2", corev1.ReadWriteOnce)
	nodeNamePv1 := "worker"
	nodeNamePv2 := "control-plane"
	pv1 := ts.newPv(pvc1.Spec.VolumeName, nodeNamePv1, corev1.ReadWriteOnce)
	pv2 := ts.newPv(pvc2.Spec.VolumeName, nodeNamePv2, corev1.ReadWriteOnce)

	ts.EnsureResources(ts.BackupResource, pv1, pv2, pvc1, pvc2)

	pv1.Status.Phase = corev1.VolumeBound
	pv2.Status.Phase = corev1.VolumeBound
	pvc1.Status.Phase = corev1.ClaimBound
	pvc2.Status.Phase = corev1.ClaimBound
	ts.UpdateStatus(pv1, pv2, pvc1, pvc2)

	volumePvc1 := corev1.Volume{
		Name: "test-pvc1",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvc1.Name,
			},
		},
	}
	volumePvc2 := corev1.Volume{
		Name: "test-pvc2",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvc2.Name,
			},
		},
	}

	result := ts.whenReconciling(ts.BackupResource)
	ts.Assert().GreaterOrEqual(result.RequeueAfter, 30*time.Second)

	jobs := new(batchv1.JobList)
	err := ts.Client.List(ts.Ctx, jobs, client.InNamespace(ts.NS))
	ts.Require().NoError(err)
	ts.Assert().Len(jobs.Items, 2)

	job1 := jobs.Items[0]
	job2 := jobs.Items[1]
	ts.assertJobSpecs(&job1, nodeNamePv1, []corev1.Volume{volumePvc1}, nil, []string{})
	ts.assertJobSpecs(&job2, nodeNamePv2, []corev1.Volume{volumePvc2}, nil, []string{})
}

func (ts *BackupTestSuite) assertCondition(conditions []metav1.Condition, condType k8upv1.ConditionType, reason k8upv1.ConditionReason, status metav1.ConditionStatus) {
	cond := meta.FindStatusCondition(conditions, condType.String())
	ts.Require().NotNil(cond, "condition of type %s missing", condType)
	ts.Assert().Equal(reason.String(), cond.Reason, "condition %s doesn't contain reason %s", condType, reason)
	ts.Assert().Equal(status, cond.Status, "condition %s isn't %s", condType, status)
}

func (ts *BackupTestSuite) assertJobSpecs(job *batchv1.Job, nodeName string, volumes []corev1.Volume, tolerations []corev1.Toleration, targetPods []string) {
	ts.Assert().Equal(nodeName, job.Spec.Template.Spec.NodeSelector[corev1.LabelHostname])
	for i, volume := range volumes {
		ts.Assert().Equal(volume.Name, job.Spec.Template.Spec.Volumes[i].Name)
		ts.Assert().Equal(volume.VolumeSource.PersistentVolumeClaim.ClaimName, job.Spec.Template.Spec.Volumes[i].VolumeSource.PersistentVolumeClaim.ClaimName)
	}

	if len(targetPods) > 0 {
		ts.Assert().Contains(job.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "TARGET_PODS", Value: strings.Join(targetPods, ",")})
	}

	for _, toleration := range tolerations {
		ts.Assert().Contains(job.Spec.Template.Spec.Tolerations, toleration)
	}
}
