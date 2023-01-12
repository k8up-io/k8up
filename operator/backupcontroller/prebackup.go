package backupcontroller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
)

// StartPreBackup will start the defined pods as deployments.
// It returns true and no error if there are no PreBackups to run after setting the ConditionPreBackupPodsReady accordingly.
// Returns false and error if the retrieval of PreBackupPod templates failed (with status update).
func (b *BackupExecutor) StartPreBackup(ctx context.Context) (bool, error) {

	templates, err := b.fetchPreBackupPodTemplates(ctx)
	if err != nil {
		b.SetConditionFalseWithMessage(ctx, k8upv1.ConditionPreBackupPodReady, k8upv1.ReasonRetrievalFailed, "error while retrieving container definitions: %v", err.Error())
		return false, err
	}

	if len(templates.Items) == 0 {
		b.SetConditionTrueWithMessage(ctx, k8upv1.ConditionPreBackupPodReady, k8upv1.ReasonNoPreBackupPodsFound, "no container definitions found")
		return true, nil
	}

	deployments := b.generateDeployments(ctx, templates.Items)

	return b.startAllPreBackupDeployments(ctx, deployments)
}

// startAllPreBackupDeployments attempts to start all given deployment templates.
// If some of them are already existing, their status will determine whether the backup is considered failed.
// It returns true if all deployments are ready, also the ConditionPreBackupPodsReady will be set to ReasonReady.
// Otherwise it returns false, indicating that not all PreBackup deployment(s) are ready.
func (b *BackupExecutor) startAllPreBackupDeployments(ctx context.Context, deployments []*appsv1.Deployment) (bool, error) {
	log := controllerruntime.LoggerFrom(ctx)

	for _, template := range deployments {
		err := b.fetchOrCreatePreBackupDeployment(ctx, template)
		if err != nil {
			return false, err
		}
	}
	ready, err := b.allDeploymentsAreReady(ctx, deployments)
	if err != nil {
		return false, nil
	}
	if ready {
		log.Info("pre backup pod(s) now ready")
		b.SetConditionTrue(ctx, k8upv1.ConditionPreBackupPodReady, k8upv1.ReasonReady)
	} else {
		b.SetConditionUnknownWithMessage(ctx, k8upv1.ConditionPreBackupPodReady, k8upv1.ReasonWaiting, "waiting for %d PreBackupPods to become ready", len(deployments))
	}
	return ready, nil
}

// allDeploymentsAreReady returns true if all given pre backup deployments have replicas in ready state.
// If one of the deployment has failed (determined via "Progressing" condition), then it returns false with an error.
// In that case, a backup is considered failed completely and status conditions will be set accordingly.
// If one deployment is neither failed nor ready, then it returns false without error, indicating that it's still waiting.
func (b *BackupExecutor) allDeploymentsAreReady(ctx context.Context, deployments []*appsv1.Deployment) (bool, error) {
	log := controllerruntime.LoggerFrom(ctx)

	for _, deployment := range deployments {
		log := log.WithValues("preBackup", k8upv1.MapToNamespacedName(deployment).String())
		ready, err := isPreBackupDeploymentReady(deployment)
		if err != nil {
			log.Info("backup failed: deadline exceeded on pre backup deployment")
			b.SetConditionFalseWithMessage(ctx, k8upv1.ConditionPreBackupPodReady, k8upv1.ReasonFailed, err.Error())
			b.SetConditionTrueWithMessage(ctx, k8upv1.ConditionReady, k8upv1.ReasonFailed, err.Error())
			b.deletePreBackupDeployment(ctx, deployment)
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
func (b *BackupExecutor) StopPreBackupDeployments(ctx context.Context) {
	log := controllerruntime.LoggerFrom(ctx)
	templates, err := b.fetchPreBackupPodTemplates(ctx)
	if err != nil {
		log.Error(err, "could not fetch pod templates", "name", b.Obj.GetName(), "namespace", b.Obj.GetNamespace())
		b.SetConditionFalseWithMessage(ctx, k8upv1.ConditionPreBackupPodReady, k8upv1.ReasonRetrievalFailed, "could not fetch pod templates: %v", err)
		return
	}

	if len(templates.Items) == 0 {
		b.SetConditionTrue(ctx, k8upv1.ConditionPreBackupPodReady, k8upv1.ReasonNoPreBackupPodsFound)
		return
	}

	deployments := b.generateDeployments(ctx, templates.Items)
	for _, deployment := range deployments {
		// Avoid exportloopref
		deployment := deployment
		b.deletePreBackupDeployment(ctx, deployment)
	}

	b.SetConditionTrue(ctx, k8upv1.ConditionPreBackupPodReady, k8upv1.ReasonFinished)
}

// deletePreBackupDeployment deletes the given deployment, if existing.
// On errors, the ConditionPreBackupPodReady will be set to false with the error message.
func (b *BackupExecutor) deletePreBackupDeployment(ctx context.Context, deployment *appsv1.Deployment) {
	log := controllerruntime.LoggerFrom(ctx)
	log.Info("removing PreBackupPod deployment", "name", deployment.Name, "namespace", deployment.Namespace)
	option := metav1.DeletePropagationForeground
	err := b.Client.Delete(ctx, deployment, &client.DeleteOptions{
		PropagationPolicy: &option,
	})
	if err != nil && !errors.IsNotFound(err) {
		log.Error(err, "could not delete deployment", "name", b.Obj.GetName(), "namespace", b.Obj.GetNamespace())
		b.SetConditionFalseWithMessage(ctx, k8upv1.ConditionPreBackupPodReady, k8upv1.ReasonDeletionFailed, "could not delete deployment: %v", err.Error())
	}
}
