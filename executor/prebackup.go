package executor

import (
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
)

// StartPreBackup will start the defined pods as deployments.
// It returns true and no error if there are no PreBackups to run after setting the ConditionPreBackupPodsReady accordingly.
// Returns false and error if the retrieval of PreBackupPod templates failed (with status update).
func (b *BackupExecutor) StartPreBackup() (bool, error) {

	templates, err := b.fetchPreBackupPodTemplates()
	if err != nil {
		b.SetConditionFalseWithMessage(k8upv1alpha1.ConditionPreBackupPodReady, k8upv1alpha1.ReasonRetrievalFailed, "error while retrieving container definitions: %v", err.Error())
		return false, err
	}

	if len(templates.Items) == 0 {
		b.SetConditionTrueWithMessage(k8upv1alpha1.ConditionPreBackupPodReady, k8upv1alpha1.ReasonNoPreBackupPodsFound, "no container definitions found")
		return true, nil
	}

	deployments := b.generateDeployments(templates.Items)

	return b.startAllPreBackupDeployments(deployments)
}

// startAllPreBackupDeployments attempts to start all given deployment templates.
// If some of them are already existing, their status will determine whether the backup is considered failed.
// It returns true if all deployments are ready, also the ConditionPreBackupPodsReady will be set to ReasonReady.
// Otherwise it returns false, indicating that not all PreBackup deployment(s) are ready.
func (b *BackupExecutor) startAllPreBackupDeployments(deployments []*appsv1.Deployment) (bool, error) {
	for _, template := range deployments {
		err := b.fetchOrCreatePreBackupDeployment(template)
		if err != nil {
			return false, err
		}
	}
	ready, err := b.allDeploymentsAreReady(deployments)
	if err != nil {
		return false, nil
	}
	if ready {
		b.Log.Info("pre backup pod(s) now ready")
		b.SetConditionTrue(k8upv1alpha1.ConditionPreBackupPodReady, k8upv1alpha1.ReasonReady)
	} else {
		b.SetConditionUnknownWithMessage(k8upv1alpha1.ConditionPreBackupPodReady, k8upv1alpha1.ReasonWaiting, "waiting for %d PreBackupPods to become ready", len(deployments))
	}
	return ready, nil
}

// allDeploymentsAreReady returns true if all given pre backup deployments have replicas in ready state.
// If one of the deployment has failed (determined via "Progressing" condition), then it returns false with an error.
// In that case, a backup is considered failed completely and status conditions will be set accordingly.
// If one deployment is neither failed nor ready, then it returns false without error, indicating that it's still waiting.
func (b *BackupExecutor) allDeploymentsAreReady(deployments []*appsv1.Deployment) (bool, error) {
	for _, deployment := range deployments {
		log := b.Log.WithValues("preBackup", k8upv1alpha1.MapToNamespacedName(deployment).String())
		ready, err := isPreBackupDeploymentReady(deployment)
		if err != nil {
			log.Info("backup failed: deadline exceeded on pre backup deployment")
			b.SetConditionFalseWithMessage(k8upv1alpha1.ConditionPreBackupPodReady, k8upv1alpha1.ReasonFailed, err.Error())
			b.SetConditionTrueWithMessage(k8upv1alpha1.ConditionReady, k8upv1alpha1.ReasonFailed, err.Error())
			b.deletePreBackupDeployment(deployment)
			return false, err
		}
		if !ready {
			log.Info("waiting on pre backup pod...")
			return false, nil
		}
		log.V(1).Info("pre backup pod is in ready state")
	}
	return true, nil
}

// StopPreBackupDeployments will remove the deployments.
func (b *BackupExecutor) StopPreBackupDeployments() {
	templates, err := b.fetchPreBackupPodTemplates()
	if err != nil {
		b.Log.Error(err, "could not fetch pod templates", "name", b.Obj.GetMetaObject().GetName(), "namespace", b.Obj.GetMetaObject().GetNamespace())
		b.SetConditionFalseWithMessage(k8upv1alpha1.ConditionPreBackupPodReady, k8upv1alpha1.ReasonRetrievalFailed, "could not fetch pod templates: %v", err)
		return
	}

	if len(templates.Items) == 0 {
		b.SetConditionTrue(k8upv1alpha1.ConditionPreBackupPodReady, k8upv1alpha1.ReasonNoPreBackupPodsFound)
		return
	}

	deployments := b.generateDeployments(templates.Items)
	for _, deployment := range deployments {
		// Avoid exportloopref
		deployment := deployment
		b.deletePreBackupDeployment(deployment)
	}

	b.SetConditionTrue(k8upv1alpha1.ConditionPreBackupPodReady, k8upv1alpha1.ReasonReady)
}

// deletePreBackupDeployment deletes the given deployment, if existing.
// On errors, the ConditionPreBackupPodReady will be set to false with the error message.
func (b *BackupExecutor) deletePreBackupDeployment(deployment *appsv1.Deployment) {
	b.Log.Info("removing PreBackupPod deployment", "name", deployment.Name, "namespace", deployment.Namespace)
	option := metav1.DeletePropagationForeground
	err := b.Client.Delete(b.CTX, deployment, &client.DeleteOptions{
		PropagationPolicy: &option,
	})
	if err != nil && !errors.IsNotFound(err) {
		b.Log.Error(err, "could not delete deployment", "name", b.Obj.GetMetaObject().GetName(), "namespace", b.Obj.GetMetaObject().GetNamespace())
		b.SetConditionFalseWithMessage(k8upv1alpha1.ConditionPreBackupPodReady, k8upv1alpha1.ReasonDeletionFailed, "could not delete deployment: %v", err.Error())
	}
}
