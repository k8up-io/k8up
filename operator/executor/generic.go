// executor contains the logic that is needed to apply the actual k8s job objects to a cluster.
// each job type should implement its own executor that handles its own job creation.
// There are various methods that provide default env vars and batch.job scaffolding.

package executor

import (
	"context"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/executor/cleaner"
	"github.com/k8up-io/k8up/v2/operator/job"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Generic struct {
	job.Config
}

// listOldResources retrieves a list of the given resource type in the given namespace and fills the Item property
// of objList. On errors, the error is being logged and the Scrubbed condition set to False with reason RetrievalFailed.
func (g *Generic) listOldResources(ctx context.Context, namespace string, objList client.ObjectList) error {
	log := controllerruntime.LoggerFrom(ctx)
	err := g.Client.List(ctx, objList, &client.ListOptions{
		Namespace: namespace,
	})
	if err != nil {
		log.Error(err, "could not list objects to cleanup old resources")
		g.SetConditionFalseWithMessage(ctx, k8upv1.ConditionScrubbed, k8upv1.ReasonRetrievalFailed, "could not list objects to cleanup old resources: %v", err)
		return err
	}
	return nil
}

type jobObjectList interface {
	client.ObjectList

	GetJobObjects() k8upv1.JobObjectList
}

func (g *Generic) CleanupOldResources(ctx context.Context, typ jobObjectList, namespace string, limits cleaner.GetJobsHistoryLimiter) {
	err := g.listOldResources(ctx, namespace, typ)
	if err != nil {
		return
	}

	cl := cleaner.NewObjectCleaner(g.Client, limits)
	deleted, err := cl.CleanOldObjects(ctx, typ.GetJobObjects())
	if err != nil {
		g.SetConditionFalseWithMessage(ctx, k8upv1.ConditionScrubbed, k8upv1.ReasonDeletionFailed, "could not cleanup old resources: %s", err.Error())
		return
	}
	g.SetConditionTrueWithMessage(ctx, k8upv1.ConditionScrubbed, k8upv1.ReasonSucceeded, "Deleted %d resources", deleted)

}
