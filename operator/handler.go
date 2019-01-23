package operator

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/vshn/k8up/log"
	"github.com/vshn/k8up/service"
)

// Handler  is the pod terminator handler that will handle the
// events received from kubernetes.
type handler struct {
	baasService service.Handler
	logger      log.Logger
}

// newHandler returns a new handler.
func newHandler(logger log.Logger, obj service.Handler) *handler {
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
