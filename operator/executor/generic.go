// executor contains the logic that is needed to apply the actual k8s job objects to a cluster.
// each job type should implement its own executor that handles its own job creation.
// There are various methods that provide default env vars and batch.job scaffolding.

package executor

import (
	"context"
	"strings"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/executor/cleaner"
	"github.com/k8up-io/k8up/v2/operator/job"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

// CleanupJobObject is satisfied by the running job. It is used to scope
// history-limit cleanup to siblings of that job — jobs that share the same
// owner, e.g. the same Schedule.
type CleanupJobObject interface {
	client.Object
	cleaner.GetJobsHistoryLimiter
}

func (g *Generic) CleanupOldResources(ctx context.Context, typ jobObjectList, runningJob CleanupJobObject) {
	err := g.listOldResources(ctx, runningJob.GetNamespace(), typ)
	if err != nil {
		return
	}

	// Multiple Schedules creating jobs of the same kind in the same namespace
	// are otherwise indistinguishable at cleanup time, so a busy Schedule could
	// evict every recent job from a less busy one — making backups look like
	// they never ran. Restrict cleanup to siblings of the running job.
	siblings := filterByController(typ.GetJobObjects(), runningJob)

	cl := cleaner.NewObjectCleaner(g.Client, runningJob)
	deleted, err := cl.CleanOldObjects(ctx, siblings)
	if err != nil {
		g.SetConditionFalseWithMessage(ctx, k8upv1.ConditionScrubbed, k8upv1.ReasonDeletionFailed, "could not cleanup old resources: %s", err.Error())
		return
	}
	g.SetConditionTrueWithMessage(ctx, k8upv1.ConditionScrubbed, k8upv1.ReasonSucceeded, "Deleted %d resources", deleted)

}

// filterByController returns only those jobs that share the same owner UID as
// runningJob, as resolved by controllerUID. If runningJob has no owner (e.g. a
// one-off Backup created directly by the user), the result contains only jobs
// that also have none.
func filterByController(jobs k8upv1.JobObjectList, runningJob client.Object) k8upv1.JobObjectList {
	ownerUID := controllerUID(runningJob)
	filtered := make(k8upv1.JobObjectList, 0, len(jobs))
	for _, j := range jobs {
		if controllerUID(j) == ownerUID {
			filtered = append(filtered, j)
		}
	}
	return filtered
}

func controllerUID(obj metav1.Object) types.UID {
	if ref := metav1.GetControllerOf(obj); ref != nil {
		return ref.UID
	}
	// Jobs created before k8up set Controller=true carry the Schedule owner
	// reference without the flag. Fall back to it so pre-upgrade jobs group
	// with their Schedule instead of with standalone jobs.
	for _, ref := range obj.GetOwnerReferences() {
		if ref.Kind == "Schedule" && strings.HasPrefix(ref.APIVersion, k8upv1.GroupVersion.Group+"/") {
			return ref.UID
		}
	}
	return ""
}
