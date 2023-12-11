package kubernetes

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
)

// PodLister holds the state for listing the pods.
type PodLister struct {
	backupCommandAnnotation   string
	backupContainerAnnotation string
	fileExtensionAnnotation   string
	k8scli                    client.Client
	err                       error
	namespace                 string
	targetPods                map[string]struct{}
	skipPreBackup             bool
	log                       logr.Logger
	ctx                       context.Context
}

// BackupPod contains all information nessecary to execute the backupcommands.
type BackupPod struct {
	Command       string
	PodName       string
	ContainerName string
	Namespace     string
	FileExtension string
}

// NewPodLister returns a PodLister configured to find the defined annotations.
func NewPodLister(ctx context.Context, k8cli client.Client, backupCommandAnnotation, fileExtensionAnnotation, backupContainerAnnotation, namespace string, targetPods []string, skipPreBackup bool, log logr.Logger) *PodLister {
	tp := make(map[string]struct{})
	for _, name := range targetPods {
		tp[name] = struct{}{}
	}

	return &PodLister{
		backupCommandAnnotation:   backupCommandAnnotation,
		backupContainerAnnotation: backupContainerAnnotation,
		fileExtensionAnnotation:   fileExtensionAnnotation,
		k8scli:                    k8cli,
		namespace:                 namespace,
		targetPods:                tp,
		skipPreBackup:             skipPreBackup,
		log:                       log.WithName("k8sClient"),
		ctx:                       ctx,
	}
}

// ListPods finds a list of pods which have backup commands in their annotations.
func (p *PodLister) ListPods() ([]BackupPod, error) {
	p.log.Info("listing all pods", "annotation", p.backupCommandAnnotation, "namespace", p.namespace)

	if p.err != nil {
		return nil, p.err
	}

	if p.skipPreBackup {
		return nil, nil
	}

	pods := &corev1.PodList{}
	err := p.k8scli.List(p.ctx, pods, &client.ListOptions{Namespace: p.namespace})
	if err != nil {
		return nil, fmt.Errorf("can't list pods: %v", err)
	}

	foundPods := make([]BackupPod, 0)
	sameOwner := make(map[string]bool)
	for _, pod := range pods.Items {
		var execInContainer = pod.Spec.Containers[0].Name
		annotations := pod.GetAnnotations()

		if execInContainerName, ok := annotations[p.backupContainerAnnotation]; ok {
			execInContainer = execInContainerName
		}

		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		// if TARGET_PODS is set, skip pods which should not be targetd.
		_, ok := p.targetPods[pod.GetName()]
		if len(p.targetPods) > 0 && !ok {
			p.log.V(1).Info("pod not in target pod list, skipping", "pod", pod.GetName())
			continue
		}

		if command, ok := annotations[p.backupCommandAnnotation]; ok {

			fileExtension := annotations[p.fileExtensionAnnotation]

			owner := pod.OwnerReferences
			var firstOwnerID string
			if len(owner) > 0 {
				firstOwnerID = string(owner[0].UID)
			} else {
				firstOwnerID = string(pod.ObjectMeta.UID)
			}

			if _, ok := sameOwner[firstOwnerID]; !ok {
				sameOwner[firstOwnerID] = true
				p.log.Info("adding to backup list", "namespace", p.namespace, "pod", pod.Name)
				foundPods = append(foundPods, BackupPod{
					Command:       command,
					PodName:       pod.Name,
					ContainerName: execInContainer,
					Namespace:     p.namespace,
					FileExtension: fileExtension,
				})
			}
		}
	}

	return foundPods, nil
}
