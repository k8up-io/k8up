package backupcontroller

import (
	"fmt"

	"golang.org/x/net/context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
)

// fetchPreBackupPodTemplates fetches all PreBackupPods from the same namespace as the originating backup.
func (b *BackupExecutor) fetchPreBackupPodTemplates(ctx context.Context) (*k8upv1.PreBackupPodList, error) {
	podList := &k8upv1.PreBackupPodList{}

	err := b.Client.List(ctx, podList, client.InNamespace(b.Obj.GetNamespace()))
	if err != nil {
		return nil, fmt.Errorf("could not list pod templates: %w", err)
	}

	return podList, nil
}

// generateDeployments creates a new PreBackupDeployment for each given PreBackupPod template.
func (b *BackupExecutor) generateDeployments(ctx context.Context, templates []k8upv1.PreBackupPod) []*appsv1.Deployment {
	log := controllerruntime.LoggerFrom(ctx)
	deployments := make([]*appsv1.Deployment, 0)

	for _, template := range templates {

		template.Spec.Pod.PodTemplateSpec.ObjectMeta.Annotations = map[string]string{
			cfg.Config.BackupCommandAnnotation: template.Spec.BackupCommand,
			cfg.Config.FileExtensionAnnotation: template.Spec.FileExtension,
		}

		podLabels := map[string]string{
			"k8up.io/backupCommandPod": "true",
			"k8up.io/preBackupPod":     template.Name,
		}

		template.Spec.Pod.PodTemplateSpec.ObjectMeta.Labels = podLabels

		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      template.GetName(),
				Namespace: b.Obj.GetNamespace(),
				Labels: labels.Set{
					"k8up.io/preBackupPod": template.Name,
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(1)),
				Template: template.Spec.Pod.PodTemplateSpec,
				Selector: &metav1.LabelSelector{
					MatchLabels: podLabels,
				},
			},
		}

		err := controllerutil.SetOwnerReference(b.Config.Obj, deployment, b.Client.Scheme())
		if err != nil {
			log.Error(err, "cannot set the owner reference", "name", b.Config.Obj.GetName(), "namespace", b.Config.Obj.GetNamespace())
		}

		deployments = append(deployments, deployment)
	}

	return deployments
}

// fetchOrCreatePreBackupDeployment fetches a deployment with the given name or creates it if not existing.
// On errors, the ConditionPreBackupPodsReady will be set to false and the error is returned.
func (b *BackupExecutor) fetchOrCreatePreBackupDeployment(ctx context.Context, deployment *appsv1.Deployment) error {
	name := k8upv1.MapToNamespacedName(deployment)
	log := controllerruntime.LoggerFrom(ctx)
	fetchErr := b.Generic.Client.Get(ctx, name, deployment)
	if fetchErr != nil {
		if !errors.IsNotFound(fetchErr) {
			err := fmt.Errorf("error getting pre backup pod '%v': %w", name.String(), fetchErr)
			b.SetConditionFalseWithMessage(ctx, k8upv1.ConditionPreBackupPodReady, k8upv1.ReasonRetrievalFailed, err.Error())
			return err
		}

		createErr := b.Client.Create(ctx, deployment)
		if createErr != nil {
			err := fmt.Errorf("error creating pre backup pod '%v': %w", name.String(), createErr)
			b.SetConditionFalseWithMessage(ctx, k8upv1.ConditionPreBackupPodReady, k8upv1.ReasonCreationFailed, err.Error())
			return err
		}
		log.Info("started pre backup pod", "preBackup", name.String())
	}
	return nil
}

// isPreBackupDeploymentReady returns true if the given deployment is ready, false if it is still progressing.
// In the case of a failed deployment, it returns false with an error.
func isPreBackupDeploymentReady(deployment *appsv1.Deployment) (bool, error) {
	progressingCondition := getProgressingCondition(deployment)
	if progressingCondition != nil && isDeadlineExceeded(progressingCondition) {
		return false, fmt.Errorf("error starting pre backup pod %v: %v", deployment.GetName(), progressingCondition.Message)
	}

	if hasAvailableReplica(deployment) {
		return true, nil
	}

	return false, nil
}

func isDeadlineExceeded(condition *appsv1.DeploymentCondition) bool {
	// if the deadline can't be respected https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#progress-deadline-seconds
	return condition.Status == corev1.ConditionFalse && condition.Reason == "ProgressDeadlineExceeded"
}

func hasAvailableReplica(deployment *appsv1.Deployment) bool {
	return deployment.Status.AvailableReplicas > 0
}

func getProgressingCondition(deployment *appsv1.Deployment) *appsv1.DeploymentCondition {
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentProgressing {
			return &condition
		}
	}
	return nil
}

func isPrebackupFailed(backup *k8upv1.Backup) bool {
	prebackupCond := meta.FindStatusCondition(backup.Status.Conditions, k8upv1.ConditionPreBackupPodReady.String())
	if prebackupCond == nil {
		return false
	}
	return prebackupCond.Reason == k8upv1.ReasonFailed.String() || prebackupCond.Reason == k8upv1.ReasonRetrievalFailed.String()
}
