package kubernetes

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
)

// PodLister holds the state for listing the pods.
type PodLister struct {
	backupCommandAnnotation   string
	backupContainerAnnotation string
	fileExtensionAnnotation   string
	fileNameAnnotation        string
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
	// FileName replaces the default <namespace>-<container> name of the
	// backed up file when set.
	FileName string
}

// StdinFilename returns the file name the output of the backup command is
// stored under in the snapshot, without the file extension. It defaults to
// /<namespace>-<container>, which is the same for every pod of a namespace
// that runs its command in a container of the same name, so the file name
// annotation lets those snapshots be told apart.
func (b BackupPod) StdinFilename(namespace string) (string, error) {
	name := strings.TrimSpace(b.FileName)
	if name == "" {
		return fmt.Sprintf("/%s-%s", namespace, b.ContainerName), nil
	}
	if strings.Contains(name, "/") || name == "." || name == ".." {
		return "", fmt.Errorf("invalid file name %q for pod %s: it can't contain \"/\" or be \".\" or \"..\"", b.FileName, b.PodName)
	}
	return "/" + name, nil
}

// NewPodLister returns a PodLister configured to find the defined annotations.
func NewPodLister(ctx context.Context, k8cli client.Client, backupCommandAnnotation, fileExtensionAnnotation, fileNameAnnotation, backupContainerAnnotation, namespace string, targetPods []string, skipPreBackup bool, log logr.Logger) *PodLister {
	tp := make(map[string]struct{})
	for _, name := range targetPods {
		tp[name] = struct{}{}
	}

	return &PodLister{
		backupCommandAnnotation:   backupCommandAnnotation,
		backupContainerAnnotation: backupContainerAnnotation,
		fileExtensionAnnotation:   fileExtensionAnnotation,
		fileNameAnnotation:        fileNameAnnotation,
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
			fileName := annotations[p.fileNameAnnotation]

			owner := pod.OwnerReferences
			var firstOwnerID string
			if len(owner) > 0 {
				firstOwnerID = string(owner[0].UID)
			} else {
				firstOwnerID = string(pod.UID)
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
					FileName:      fileName,
				})
			}
		}
	}

	return foundPods, nil
}
