package handler

import (
	"github.com/vshn/k8up/executor"
	"github.com/vshn/k8up/job"
	"github.com/vshn/k8up/queue"
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

// Handle checks if that job is started and will add it to the queue, if not.
func (h *Handler) Handle() error {
	if !h.Obj.GetK8upStatus().Started {
		return h.queueJob()
	}

	return nil
}

func (h *Handler) queueJob() error {
	h.Log.Info("adding job to the queue")

	e := executor.NewExecutor(h.Config)

	queue.GetExecQueue().Add(e)

	return nil
}
