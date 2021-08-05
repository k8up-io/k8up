package executor

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/operator/cfg"
)

// fetchPreBackupPodTemplates fetches all PreBackupPods from the same namespace as the originating backup.
func (b *BackupExecutor) fetchPreBackupPodTemplates() (*k8upv1alpha1.PreBackupPodList, error) {
	podList := &k8upv1alpha1.PreBackupPodList{}

	err := b.Client.List(b.CTX, podList, client.InNamespace(b.Obj.GetMetaObject().GetNamespace()))
	if err != nil {
		return nil, fmt.Errorf("could not list pod templates: %w", err)
	}

	return podList, nil
}

// generateDeployments creates a new PreBackupDeployment for each given PreBackupPod template.
func (b *BackupExecutor) generateDeployments(templates []k8upv1alpha1.PreBackupPod) []*appsv1.Deployment {
	deployments := make([]*appsv1.Deployment, 0)

	for _, template := range templates {

		template.Spec.Pod.PodTemplateSpec.ObjectMeta.Annotations = map[string]string{
			cfg.Config.BackupCommandAnnotation: template.Spec.BackupCommand,
			cfg.Config.FileExtensionAnnotation: template.Spec.FileExtension,
		}

		podLabels := map[string]string{
			"k8up.syn.tools/backupCommandPod": "true",
			"k8up.syn.tools/preBackupPod":     template.Name,
		}

		template.Spec.Pod.PodTemplateSpec.ObjectMeta.Labels = podLabels

		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      template.GetName(),
				Namespace: b.Obj.GetMetaObject().GetNamespace(),
				Labels: labels.Set{
					"k8up.syn.tools/preBackupPod": template.Name,
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: pointer.Int32Ptr(1),
				Template: template.Spec.Pod.PodTemplateSpec,
				Selector: &metav1.LabelSelector{
					MatchLabels: podLabels,
				},
			},
		}

		err := controllerutil.SetOwnerReference(b.Config.Obj.GetMetaObject(), deployment, b.Scheme)
		if err != nil {
			b.Config.Log.Error(err, "cannot set the owner reference", "name", b.Config.Obj.GetMetaObject().GetName(), "namespace", b.Config.Obj.GetMetaObject().GetNamespace())
		}

		deployments = append(deployments, deployment)
	}

	return deployments
}

// fetchOrCreatePreBackupDeployment fetches a deployment with the given name or creates it if not existing.
// On errors, the ConditionPreBackupPodsReady will be set to false and the error is returned.
func (b *BackupExecutor) fetchOrCreatePreBackupDeployment(deployment *appsv1.Deployment) error {
	name := k8upv1alpha1.MapToNamespacedName(deployment)
	fetchErr := b.Client.Get(b.CTX, name, deployment)
	if fetchErr != nil {
		if !errors.IsNotFound(fetchErr) {
			err := fmt.Errorf("error getting pre backup pod '%v': %w", name.String(), fetchErr)
			b.SetConditionFalseWithMessage(k8upv1alpha1.ConditionPreBackupPodReady, k8upv1alpha1.ReasonRetrievalFailed, err.Error())
			return err
		}

		createErr := b.Client.Create(b.CTX, deployment)
		if createErr != nil {
			err := fmt.Errorf("error creating pre backup pod '%v': %w", name.String(), createErr)
			b.SetConditionFalseWithMessage(k8upv1alpha1.ConditionPreBackupPodReady, k8upv1alpha1.ReasonCreationFailed, err.Error())
			return err
		}
		b.Log.Info("started pre backup pod", "preBackup", name.String())
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
