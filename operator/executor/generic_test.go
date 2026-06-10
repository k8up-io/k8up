package executor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/job"
)

func TestFilterByController_GroupsBySchedule(t *testing.T) {
	scheduleA := backupOwnedBy("a", "schedule-a")
	scheduleB1 := backupOwnedBy("b1", "schedule-b")
	scheduleB2 := backupOwnedBy("b2", "schedule-b")
	standalone := backupOwnedBy("standalone", "")

	jobs := k8upv1.JobObjectList{&scheduleA, &scheduleB1, &scheduleB2, &standalone}

	filtered := filterByController(jobs, &scheduleB1)

	assert.Len(t, filtered, 2)
	assert.Contains(t, filtered, k8upv1.JobObject(&scheduleB1))
	assert.Contains(t, filtered, k8upv1.JobObject(&scheduleB2))
}

func TestFilterByController_LegacyOwnerRefsGroupWithTheirSchedule(t *testing.T) {
	current := backupOwnedBy("a-new", "schedule-a")
	legacyA := legacyBackupOwnedBy("a-legacy", "schedule-a")
	legacyB := legacyBackupOwnedBy("b-legacy", "schedule-b")
	standalone := backupOwnedBy("standalone", "")

	jobs := k8upv1.JobObjectList{&current, &legacyA, &legacyB, &standalone}

	filtered := filterByController(jobs, &current)

	assert.Len(t, filtered, 2)
	assert.Contains(t, filtered, k8upv1.JobObject(&current))
	assert.Contains(t, filtered, k8upv1.JobObject(&legacyA))
}

func TestFilterByController_StandalonesGroupTogether(t *testing.T) {
	scheduleA := backupOwnedBy("a", "schedule-a")
	standalone1 := backupOwnedBy("s1", "")
	standalone2 := backupOwnedBy("s2", "")

	jobs := k8upv1.JobObjectList{&scheduleA, &standalone1, &standalone2}

	filtered := filterByController(jobs, &standalone1)

	assert.Len(t, filtered, 2)
	assert.Contains(t, filtered, k8upv1.JobObject(&standalone1))
	assert.Contains(t, filtered, k8upv1.JobObject(&standalone2))
}

// TestCleanupOldResources_DoesNotEvictOtherSchedulesBackups is a regression
// test for issue #1212. With successfulJobsHistoryLimit=1 set on Schedule A's
// running Backup, Schedule B's backups must remain untouched — before the
// fix, cleanup operated on all Backups in the namespace and a busy Schedule
// could evict siblings owned by a different Schedule.
func TestCleanupOldResources_DoesNotEvictOtherSchedulesBackups(t *testing.T) {
	const ns = "myns"

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, k8upv1.AddToScheme(scheme))

	now := time.Now()
	runningA := newSuccessfulBackup("a-new", ns, "schedule-a", now)
	runningA.Spec.SuccessfulJobsHistoryLimit = ptr.To(1)
	oldA := newSuccessfulBackup("a-old", ns, "schedule-a", now.Add(-2*time.Hour))
	oldB := newSuccessfulBackup("b-old", ns, "schedule-b", now.Add(-3*time.Hour))
	olderB := newSuccessfulBackup("b-older", ns, "schedule-b", now.Add(-4*time.Hour))
	legacyA := newSuccessfulBackup("a-legacy", ns, "schedule-a", now.Add(-5*time.Hour))
	legacyA.OwnerReferences[0].Controller = nil

	fclient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&k8upv1.Backup{}).
		WithObjects(runningA, oldA, oldB, olderB, legacyA).
		Build()

	g := &Generic{Config: job.Config{Client: fclient, Obj: runningA}}
	g.CleanupOldResources(context.Background(), &k8upv1.BackupList{}, runningA)

	after := &k8upv1.BackupList{}
	require.NoError(t, fclient.List(context.Background(), after))

	remaining := make(map[string]bool, len(after.Items))
	for _, b := range after.Items {
		if b.DeletionTimestamp == nil {
			remaining[b.Name] = true
		}
	}

	assert.True(t, remaining["a-new"], "running backup must survive")
	assert.False(t, remaining["a-old"], "schedule-a's history limit should have evicted a-old")
	assert.False(t, remaining["a-legacy"], "backup created before the Controller flag was set must still be evicted by its schedule's history limit")
	assert.True(t, remaining["b-old"], "schedule-b's backups must not be touched by schedule-a's cleanup")
	assert.True(t, remaining["b-older"], "schedule-b's backups must not be touched by schedule-a's cleanup")
}

// backupOwnedBy returns a Backup whose controlling OwnerReference UID is set
// to ownerUID. An empty ownerUID produces a Backup with no controller.
func backupOwnedBy(name, ownerUID string) k8upv1.Backup {
	b := k8upv1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
	if ownerUID != "" {
		b.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: k8upv1.GroupVersion.String(),
			Kind:       "Schedule",
			Name:       ownerUID,
			UID:        types.UID(ownerUID),
			Controller: ptr.To(true),
		}}
	}
	return b
}

// legacyBackupOwnedBy returns a Backup as created by k8up before it set the
// Controller flag: a Schedule owner reference without Controller=true.
func legacyBackupOwnedBy(name, ownerUID string) k8upv1.Backup {
	b := backupOwnedBy(name, ownerUID)
	b.OwnerReferences[0].Controller = nil
	return b
}

func newSuccessfulBackup(name, ns, scheduleUID string, createdAt time.Time) *k8upv1.Backup {
	return &k8upv1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         ns,
			CreationTimestamp: metav1.NewTime(createdAt),
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: k8upv1.GroupVersion.String(),
				Kind:       "Schedule",
				Name:       scheduleUID,
				UID:        types.UID(scheduleUID),
				Controller: ptr.To(true),
			}},
		},
		Status: k8upv1.Status{
			Conditions: []metav1.Condition{{
				Type:               k8upv1.ConditionCompleted.String(),
				Status:             metav1.ConditionTrue,
				Reason:             k8upv1.ReasonSucceeded.String(),
				LastTransitionTime: metav1.Now(),
			}},
		},
	}
}
