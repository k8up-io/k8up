package kubernetes

import (
	"context"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/restic/dto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SyncSnapshotList will take a k8upv1.SnapshotList and apply them to the k8s cluster.
// It will remove any snapshots on the cluster that are not present in the list.
func SyncSnapshotList(ctx context.Context, list []dto.Snapshot, namespace, repository string) error {

	newList := filterAndConvert(list, namespace, repository)
	oldList := &k8upv1.SnapshotList{}

	kube, err := NewTypedClient()
	if err != nil {
		return err
	}

	err = kube.List(ctx, oldList, &client.ListOptions{Namespace: namespace})
	if err != nil {
		return err
	}

	oldList = filterByRepo(oldList, repository)

	err = createNewSnapshots(ctx, newList, oldList, kube)
	if err != nil {
		return err
	}

	return deleteOldSnapshots(ctx, newList, oldList, kube)
}

// createNewSnapshots is a wrapper for the diff function to correctly pass the right order and function
func createNewSnapshots(ctx context.Context, newList, oldList *k8upv1.SnapshotList, kube client.Client) error {
	return diff(newList, oldList, func(snap k8upv1.Snapshot) error {
		return kube.Create(ctx, &snap)
	})
}

// deleteOldSnapshots is a wrapper for the diff function to correctly pass the right order and function
func deleteOldSnapshots(ctx context.Context, newList, oldList *k8upv1.SnapshotList, kube client.Client) error {
	return diff(oldList, newList, func(snap k8upv1.Snapshot) error {
		return kube.Delete(ctx, &snap)
	})
}

// diff will execute a given function for each element that is in a but not b
// it assumes that both lists belong to the same namespace and repository
func diff(a, b *k8upv1.SnapshotList, diffFunc func(snap k8upv1.Snapshot) error) error {

	snapMap := listToMap(b)

	for _, snapshot := range a.Items {
		// Avoid pointer bug
		snapshot := &snapshot
		if _, ok := snapMap[*snapshot.Spec.ID]; !ok {
			err := diffFunc(*snapshot)
			if err != nil {
				return err
			}
		}
	}

	return nil

}

func listToMap(list *k8upv1.SnapshotList) map[string]bool {

	finalMap := map[string]bool{}

	for _, snap := range list.Items {
		finalMap[*snap.Spec.ID] = true
	}

	return finalMap
}

// filterAndConvert removes snapshots that don't belong to the same namespace
// and it converts them to the k8up snapshot CR.
func filterAndConvert(list []dto.Snapshot, namespace, repository string) *k8upv1.SnapshotList {

	finalList := &k8upv1.SnapshotList{Items: []k8upv1.Snapshot{}}

	for _, snapshot := range list {
		// Avoid pointer bug
		snapshot := snapshot

		// we don't want snapshots that belong to another namespace in here.
		if snapshot.Hostname != namespace {
			continue
		}

		finalList.Items = append(finalList.Items, k8upv1.Snapshot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      snapshot.ID[:8],
				Namespace: namespace,
			},
			Spec: k8upv1.SnapshotSpec{
				ID:         &snapshot.ID,
				Date:       &metav1.Time{Time: snapshot.Time},
				Paths:      &snapshot.Paths,
				Repository: &repository,
			},
		})
	}

	return finalList
}

// filterByRepo will filter the list according to the given repository.
func filterByRepo(list *k8upv1.SnapshotList, repo string) *k8upv1.SnapshotList {
	filteredList := &k8upv1.SnapshotList{Items: []k8upv1.Snapshot{}}

	for _, snap := range list.Items {
		snap := snap
		if *snap.Spec.Repository == repo {
			filteredList.Items = append(filteredList.Items, snap)
		}
	}

	return filteredList
}
