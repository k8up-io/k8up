package handler

import (
	"github.com/vshn/k8up/operator/executor"
	"github.com/vshn/k8up/operator/job"
	"github.com/vshn/k8up/operator/queue"
)

// Handler is the generic job handler for most of the k8up jobs.
type Handler struct {
	job.Config
}

// NewHandler will return a new generic handler.
func NewHandler(config job.Config) *Handler {
	return &Handler{
		Config: config,
	}
}

// Handle will add it to the queue.
func (h *Handler) Handle() error {
	return h.queueJob()
}

func (h *Handler) queueJob() error {
	h.Log.V(1).Info("adding job to the queue")

	e := executor.NewExecutor(h.Config)

	queue.GetExecQueue().Add(e)

	return nil
}
