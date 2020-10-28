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
	object job.Object
}

func NewHandler(config job.Config, obj job.Object) *Handler {
	return &Handler{
		Config: config,
		object: obj,
	}
}

func (h *Handler) Handle() error {
	jobObj := &batchv1.Job{}
	err := h.Client.Get(h.CTX, types.NamespacedName{
		Name:      h.object.GetMetaObject().GetName(),
		Namespace: h.object.GetMetaObject().GetNamespace()}, jobObj)
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

	queue.GetExecQueue().Add(executor.NewExecutor(h.object, h.Config))

	return nil
}
