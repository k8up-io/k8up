package handler

import (
	"github.com/vshn/k8up/executor"
	"github.com/vshn/k8up/job"
	"github.com/vshn/k8up/queue"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

type Handler struct {
	job.Config
}

func NewHandler(config job.Config) *Handler {
	return &Handler{
		Config: config,
	}
}

func (h *Handler) Handle() error {
	jobObj := &batchv1.Job{}
	err := h.Client.Get(h.CTX, types.NamespacedName{
		Name:      h.Obj.GetMetaObject().GetName(),
		Namespace: h.Obj.GetMetaObject().GetNamespace()}, jobObj)
	if err != nil && errors.IsNotFound(err) {
		return h.queueJob(jobObj)
	} else if err != nil {
		h.Log.Error(err, "Failed to get Job")
		return err
	}

	return nil
}

func (h *Handler) queueJob(job *batchv1.Job) error {
	h.Log.Info("adding job to the queue")

	queue.GetExecQueue().Add(executor.NewExecutor(h.Config))

	return nil
}
