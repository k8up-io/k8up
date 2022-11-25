package executor

import (
	"context"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CITANode struct {
	namespace string
	name      string
	Client    client.Client
	CTX       context.Context
}

func NewCITANode(ctx context.Context, client client.Client, namespace, name string) *CITANode {
	return &CITANode{
		CTX:       ctx,
		Client:    client,
		name:      name,
		namespace: namespace,
	}
}

func (c *CITANode) Stop() (bool, error) {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		sts := &appsv1.StatefulSet{}
		err := c.Client.Get(c.CTX, types.NamespacedName{Name: c.name, Namespace: c.namespace}, sts)
		if err != nil {
			return err
		}
		sts.Spec.Replicas = pointer.Int32(0)
		err = c.Client.Update(c.CTX, sts)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return false, err
	}

	found := &appsv1.StatefulSet{}
	err = c.Client.Get(c.CTX, types.NamespacedName{Name: c.name, Namespace: c.namespace}, found)
	if err != nil {
		return false, err
	}
	if found.Status.ReadyReplicas == 0 {
		return true, nil
	}
	return false, nil
}

func (c *CITANode) Start() {
	_ = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		sts := &appsv1.StatefulSet{}
		err := c.Client.Get(c.CTX, types.NamespacedName{Name: c.name, Namespace: c.namespace}, sts)
		if err != nil {
			return err
		}
		sts.Spec.Replicas = pointer.Int32(1)
		err = c.Client.Update(c.CTX, sts)
		if err != nil {
			return err
		}
		return nil
	})
	return
}
