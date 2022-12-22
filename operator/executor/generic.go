// executor contains the logic that is needed to apply the actual k8s job objects to a cluster.
// each job type should implement its own executor that handles its own job creation.
// There are various methods that provide default env vars and batch.job scaffolding.

package executor

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/executor/cleaner"
	"github.com/k8up-io/k8up/v2/operator/job"
)

type Generic struct {
	job.Config
}

func (g *Generic) Logger() logr.Logger {
	return g.Log
}

func (*Generic) Exclusive() bool {
	return false
}

func (g *Generic) GetRepository() string {
	return g.Repository
}

func (g *Generic) GetJobNamespace() string {
	return g.Obj.GetNamespace()
}

func (g *Generic) GetJobNamespacedName() types.NamespacedName {
	return types.NamespacedName{Namespace: g.Obj.GetNamespace(), Name: g.Obj.GetJobName()}
}

func (g *Generic) GetJobType() k8upv1.JobType {
	return g.Obj.GetType()
}

// listOldResources retrieves a list of the given resource type in the given namespace and fills the Item property
// of objList. On errors, the error is being logged and the Scrubbed condition set to False with reason RetrievalFailed.
func (g *Generic) listOldResources(namespace string, objList client.ObjectList) error {
	err := g.Client.List(g.CTX, objList, &client.ListOptions{
		Namespace: namespace,
	})
	if err != nil {
		g.Log.Error(err, "could not list objects to cleanup old resources", "Namespace", namespace, "kind", objList.GetObjectKind().GroupVersionKind().Kind)
		g.SetConditionFalseWithMessage(k8upv1.ConditionScrubbed, k8upv1.ReasonRetrievalFailed, "could not list objects to cleanup old resources: %v", err)
		return err
	}
	return nil
}

type jobObjectList interface {
	client.ObjectList

	GetJobObjects() k8upv1.JobObjectList
}

func (g *Generic) CleanupOldResources(typ jobObjectList, name types.NamespacedName, limits cleaner.GetJobsHistoryLimiter) {
	err := g.listOldResources(name.Namespace, typ)
	if err != nil {
		return
	}

	cl := cleaner.ObjectCleaner{Client: g.Client, Limits: limits, Log: g.Logger()}
	deleted, err := cl.CleanOldObjects(g.CTX, typ.GetJobObjects())
	if err != nil {
		g.SetConditionFalseWithMessage(k8upv1.ConditionScrubbed, k8upv1.ReasonDeletionFailed, "could not cleanup old resources: %v", err)
		return
	}
	g.SetConditionTrueWithMessage(k8upv1.ConditionScrubbed, k8upv1.ReasonSucceeded, "Deleted %v resources", deleted)

}
