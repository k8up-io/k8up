package operator

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"

	backupv1alpha1 "git.vshn.net/vshn/baas/apis/backup/v1alpha1"
	baas8scli "git.vshn.net/vshn/baas/client/k8s/clientset/versioned"
	"git.vshn.net/vshn/baas/log"
	"git.vshn.net/vshn/baas/service/baas"
)

// Handler  is the pod terminator handler that will handle the
// events received from kubernetes.
type handler struct {
	baasService baas.Syncer
	logger      log.Logger
}

// newHandler returns a new handler.
func newHandler(k8sCli kubernetes.Interface, baasCLI baas8scli.Interface, logger log.Logger) *handler {
	return &handler{
		baasService: baas.NewBaas(k8sCli, baasCLI, logger),
		logger:      logger,
	}
}

// Add will ensure that the required pod terminator is running.
func (h *handler) Add(obj runtime.Object) error {
	bw, ok := obj.(*backupv1alpha1.Backup)
	if !ok {
		return fmt.Errorf("%v is not a pod terminator object", obj.GetObjectKind())
	}

	return h.baasService.EnsureBackup(bw)
}

// Delete will ensure the reuited pod terminator is not running.
func (h *handler) Delete(name string) error {
	return h.baasService.DeleteBackup(name)
}
