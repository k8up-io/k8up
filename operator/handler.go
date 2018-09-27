package operator

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"

	baas8scli "git.vshn.net/vshn/baas/client/k8s/clientset/versioned"
	"git.vshn.net/vshn/baas/log"
	"git.vshn.net/vshn/baas/service"
)

// Handler  is the pod terminator handler that will handle the
// events received from kubernetes.
type handler struct {
	baasService service.CRDEnsurer
	logger      log.Logger
}

// newHandler returns a new handler.
func newHandler(k8sCli kubernetes.Interface, baasCLI baas8scli.Interface, logger log.Logger, obj service.CRDEnsurer) *handler {
	return &handler{
		baasService: obj,
		logger:      logger,
	}
}

// Add will ensure that the required pod terminator is running.
func (h *handler) Add(ctx context.Context, obj runtime.Object) error {
	return h.baasService.Ensure(obj)
}

// Delete will ensure the reuited pod terminator is not running.
func (h *handler) Delete(ctx context.Context, name string) error {
	return h.baasService.Delete(name)
}
