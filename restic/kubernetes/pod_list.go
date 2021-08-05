package kubernetes

import (
	"fmt"

	"github.com/go-logr/logr"
	kube "k8s.io/client-go/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodLister holds the state for listing the pods.
type PodLister struct {
	backupCommandAnnotation string
	fileExtensionAnnotation string
	k8scli                  *kube.Clientset
	err                     error
	namespace               string
	log                     logr.Logger
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
func NewPodLister(backupCommandAnnotation, fileExtensionAnnotation, namespace string, log logr.Logger) *PodLister {
	k8cli, err := newk8sClient()
	if err != nil {
		err = fmt.Errorf("can't create podLister: %v", err)
	}
	return &PodLister{
		backupCommandAnnotation: backupCommandAnnotation,
		fileExtensionAnnotation: fileExtensionAnnotation,
		k8scli:                  k8cli,
		err:                     err,
		namespace:               namespace,
		log:                     log.WithName("k8sClient"),
	}
}

// ListPods resturns a list of pods which have backup commands in their annotations.
func (p *PodLister) ListPods() ([]BackupPod, error) {
	p.log.Info("listing all pods", "annotation", p.backupCommandAnnotation, "namespace", p.namespace)

	if p.err != nil {
		return nil, p.err
	}

	pods, err := p.k8scli.CoreV1().Pods(p.namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("can't list pods: %v", err)
	}

	foundPods := []BackupPod{}

	sameOwner := make(map[string]bool)

	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		annotations := pod.GetAnnotations()

		if command, ok := annotations[p.backupCommandAnnotation]; ok {

			fileExtension := annotations[p.fileExtensionAnnotation]

			owner := pod.OwnerReferences
			firstOwnerID := string(owner[0].UID)

			if _, ok := sameOwner[firstOwnerID]; !ok {
				sameOwner[firstOwnerID] = true
				p.log.Info("adding to backup list", "namespace", p.namespace, "pod", pod.Name)
				foundPods = append(foundPods, BackupPod{
					Command:       command,
					PodName:       pod.Name,
					ContainerName: pod.Spec.Containers[0].Name,
					Namespace:     p.namespace,
					FileExtension: fileExtension,
				})
			}

		}
	}

	return foundPods, nil
}
