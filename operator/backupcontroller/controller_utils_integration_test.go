//go:build integration

package backupcontroller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/pointer"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
)

const (
	backupTag = "integrationTag"
)

func (ts *BackupTestSuite) newPvc(name string, accessMode corev1.PersistentVolumeAccessMode) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ts.NS,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{accessMode},
			Resources: corev1.VolumeResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: resource.MustParse("1Mi"),
				},
			},
			VolumeName: name,
		},
	}
}

func (ts *BackupTestSuite) newPv(name string, nodeName string, accessMode corev1.PersistentVolumeAccessMode) *corev1.PersistentVolume {
	return &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PersistentVolumeSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{accessMode},
			Capacity: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceStorage: resource.MustParse("1Mi"),
			},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/tmp/integration-tests"},
			},
			NodeAffinity: &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      corev1.LabelHostname,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{nodeName},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (ts *BackupTestSuite) newPod(name, nodeName string, tolerations []corev1.Toleration, volumes []corev1.Volume) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ts.NS,
		},
		Spec: corev1.PodSpec{
			NodeName:    nodeName,
			Tolerations: tolerations,
			Volumes:     volumes,
			Containers: []corev1.Container{
				{
					Name:    "main",
					Command: []string{"/bin/sh"},
					Image:   "dummy",
				},
			},
		},
	}
}

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
			UID:       uuid.NewUUID(),
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

func (ts *BackupTestSuite) whenReconciling(object *k8upv1.Backup) controllerruntime.Result {
	result, err := ts.Controller.Provision(ts.Ctx, object)
	ts.Require().NoError(err)

	return result
}

func (ts *BackupTestSuite) expectABackupJob() (foundJob *batchv1.Job) {
	jobs := new(batchv1.JobList)
	err := ts.Client.List(ts.Ctx, jobs, client.InNamespace(ts.NS))
	ts.Require().NoError(err)

	jobsLen := len(jobs.Items)
	ts.T().Logf("%d Jobs found", jobsLen)
	ts.Require().Len(jobs.Items, 1, "job exists")
	return &jobs.Items[0]
}

func (ts *BackupTestSuite) assertPrebackupDeploymentExists() {
	pod := ts.newPreBackupDeployment()
	ts.Assert().True(ts.IsResourceExisting(ts.Ctx, pod), "expected pre backup deployment to be existing")
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

func (ts *BackupTestSuite) assertBackupExists() {
	backups := new(k8upv1.BackupList)
	err := ts.Client.List(ts.Ctx, backups, client.InNamespace(ts.NS))
	ts.Require().NoError(err)

	backupsLen := len(backups.Items)
	ts.T().Logf("%d Backups found", backupsLen)
	ts.Assert().Len(backups.Items, 1)
}

func (ts *BackupTestSuite) assertConditionWaitingForPreBackup(backup *k8upv1.Backup) {
	err := ts.Client.Get(ts.Ctx, k8upv1.MapToNamespacedName(backup), backup)
	ts.Require().NoError(err)
	preBackupCond := meta.FindStatusCondition(backup.Status.Conditions, k8upv1.ConditionPreBackupPodReady.String())
	ts.Require().NotNil(preBackupCond)
	ts.Assert().Equal(k8upv1.ReasonWaiting.String(), preBackupCond.Reason)
	ts.Assert().Equal(metav1.ConditionUnknown, preBackupCond.Status)
}

func (ts *BackupTestSuite) assertConditionReadyForPreBackup(backup *k8upv1.Backup) {
	err := ts.Client.Get(ts.Ctx, k8upv1.MapToNamespacedName(backup), backup)
	ts.Require().NoError(err)
	preBackupCond := meta.FindStatusCondition(backup.Status.Conditions, k8upv1.ConditionPreBackupPodReady.String())
	ts.Require().NotNil(preBackupCond)
	ts.Assert().Equal(k8upv1.ReasonReady.String(), preBackupCond.Reason)
	ts.Assert().Equal(metav1.ConditionTrue, preBackupCond.Status)
}

func (ts *BackupTestSuite) assertPreBackupPodConditionFailed(backup *k8upv1.Backup) {
	err := ts.Client.Get(ts.Ctx, k8upv1.MapToNamespacedName(backup), backup)
	ts.Require().NoError(err)
	preBackupCond := meta.FindStatusCondition(backup.Status.Conditions, k8upv1.ConditionPreBackupPodReady.String())
	ts.Require().NotNil(preBackupCond)
	ts.Assert().Equal(k8upv1.ReasonFailed.String(), preBackupCond.Reason)
	ts.Assert().Equal(metav1.ConditionFalse, preBackupCond.Status)
}

func (ts *BackupTestSuite) assertPreBackupPodConditionSucceeded(backup *k8upv1.Backup) {
	err := ts.Client.Get(ts.Ctx, k8upv1.MapToNamespacedName(backup), backup)
	ts.Require().NoError(err)
	preBackupCond := meta.FindStatusCondition(backup.Status.Conditions, k8upv1.ConditionPreBackupPodReady.String())
	ts.Require().NotNil(preBackupCond)
	ts.Assert().Equal(k8upv1.ReasonSucceeded.String(), preBackupCond.Reason)
	ts.Assert().Equal(metav1.ConditionFalse, preBackupCond.Status)
}

func (ts *BackupTestSuite) assertPreBackupPodConditionReady(backup *k8upv1.Backup) {
	err := ts.Client.Get(ts.Ctx, k8upv1.MapToNamespacedName(backup), backup)
	ts.Require().NoError(err)
	preBackupCond := meta.FindStatusCondition(backup.Status.Conditions, k8upv1.ConditionPreBackupPodReady.String())
	ts.Require().NotNil(preBackupCond)
	ts.Assert().Equal(k8upv1.ReasonReady.String(), preBackupCond.Reason)
	ts.Assert().Equal(metav1.ConditionTrue, preBackupCond.Status)
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

func (ts *BackupTestSuite) newJob(owner client.Object) *batchv1.Job {
	jb := &batchv1.Job{}
	jb.Name = k8upv1.BackupType.String() + "-" + ts.BackupResource.Name
	jb.Namespace = ts.NS
	jb.Labels = labels.Set{
		k8upv1.LabelK8upType:    k8upv1.BackupType.String(),
		k8upv1.LabelK8upOwnedBy: k8upv1.BackupType.String() + "_" + ts.BackupResource.Name,
	}
	jb.Spec.Template.Spec.Containers = []corev1.Container{{Name: "container", Image: "image"}}
	jb.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyOnFailure
	ts.Assert().NoError(controllerruntime.SetControllerReference(owner, jb, ts.Scheme), "set controller ref")
	return jb
}
