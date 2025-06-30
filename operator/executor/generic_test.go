package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/job"
)

func TestGeneric_listOldResources_FiltersJobsByOwnership(t *testing.T) {
	ctx := context.Background()
	namespace := "test-namespace"

	// Create test backup lists with proper ownership labels
	backupList1 := createBackupListWithOwnership("backup_backup-scheduler-1", namespace, 3)
	backupList2 := createBackupListWithOwnership("backup_backup-scheduler-2", namespace, 2)

	// Combine all backup objects for the fake client
	allBackups := &k8upv1.BackupList{Items: append(backupList1.Items, backupList2.Items...)}

	// Create fake client with all backups
	scheme := runtime.NewScheme()
	require.NoError(t, k8upv1.AddToScheme(scheme))
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithLists(allBackups).Build()

	// Create test backup for the executor
	testBackup := createTestBackup("backup-scheduler-1", namespace)

	// Create generic executor with fake client
	generic := &Generic{
		Config: job.Config{
			Client: fakeClient,
			Obj:    testBackup,
		},
	}

	// Test filtering for backup-scheduler-1
	resultList1 := &k8upv1.BackupList{}
	err := generic.listOldResources(ctx, namespace, resultList1, "backup_backup-scheduler-1")
	require.NoError(t, err)

	// Should only return backups owned by backup-scheduler-1
	assert.Len(t, resultList1.Items, 3)
	for _, backup := range resultList1.Items {
		assert.Equal(t, "backup_backup-scheduler-1", backup.Labels[k8upv1.LabelK8upOwnedBy])
	}

	// Test filtering for backup-scheduler-2
	resultList2 := &k8upv1.BackupList{}
	err = generic.listOldResources(ctx, namespace, resultList2, "backup_backup-scheduler-2")
	require.NoError(t, err)

	// Should only return backups owned by backup-scheduler-2
	assert.Len(t, resultList2.Items, 2)
	for _, backup := range resultList2.Items {
		assert.Equal(t, "backup_backup-scheduler-2", backup.Labels[k8upv1.LabelK8upOwnedBy])
	}

	// Test filtering for non-existent owner
	resultList3 := &k8upv1.BackupList{}
	err = generic.listOldResources(ctx, namespace, resultList3, "backup_non-existent")
	require.NoError(t, err)
	assert.Len(t, resultList3.Items, 0)
}

func TestGeneric_CleanupOldResources_OnlyDeletesOwnedJobs(t *testing.T) {
	ctx := context.Background()
	namespace := "test-namespace"

	// Create test backups with different owners and completion status
	scheduler1Backups := createBackupListWithMixedStatus("backup_backup-scheduler-1", namespace, 2, 2, 1) // 2 successful, 2 failed, 1 running
	scheduler2Backups := createBackupListWithMixedStatus("backup_backup-scheduler-2", namespace, 3, 1, 0) // 3 successful, 1 failed, 0 running

	// Combine all backup objects for the fake client
	allBackups := &k8upv1.BackupList{Items: append(scheduler1Backups.Items, scheduler2Backups.Items...)}

	// Create fake client with all backups
	scheme := runtime.NewScheme()
	require.NoError(t, k8upv1.AddToScheme(scheme))
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithLists(allBackups).Build()

	// Create test backup object to represent the scheduler
	testBackup := createTestBackup("backup-scheduler-1", namespace)

	// Configure testBackup with limits that will trigger cleanup (keep only 1 successful, 1 failed)
	testBackup.Spec.SuccessfulJobsHistoryLimit = &[]int{1}[0]
	testBackup.Spec.FailedJobsHistoryLimit = &[]int{1}[0]

	// Create generic executor
	generic := &Generic{
		Config: job.Config{
			Client: fakeClient,
			Obj:    testBackup,
		},
	}

	// Run cleanup for backup-scheduler-1
	generic.CleanupOldResources(ctx, &k8upv1.BackupList{}, namespace, testBackup)

	// Verify only scheduler-1's jobs are affected
	resultList := &k8upv1.BackupList{}
	err := fakeClient.List(ctx, resultList, client.InNamespace(namespace))
	require.NoError(t, err)

	// Count remaining jobs by ownership
	scheduler1Jobs := 0
	scheduler2Jobs := 0
	for _, backup := range resultList.Items {
		if backup.DeletionTimestamp == nil { // Only count non-deleted jobs
			if backup.Labels[k8upv1.LabelK8upOwnedBy] == "backup_backup-scheduler-1" {
				scheduler1Jobs++
			} else if backup.Labels[k8upv1.LabelK8upOwnedBy] == "backup_backup-scheduler-2" {
				scheduler2Jobs++
			}
		}
	}

	// Scheduler-1 should have cleaned up old jobs: 1 successful + 1 failed + 1 running = 3 remaining
	assert.Equal(t, 3, scheduler1Jobs, "Scheduler-1 should have 3 jobs remaining after cleanup")

	// Scheduler-2 should be untouched: 3 successful + 1 failed = 4 jobs
	assert.Equal(t, 4, scheduler2Jobs, "Scheduler-2 should still have all 4 jobs")
}

func TestGeneric_listOldResources_HandlesEmptyNamespace(t *testing.T) {
	ctx := context.Background()
	namespace := "empty-namespace"

	scheme := runtime.NewScheme()
	require.NoError(t, k8upv1.AddToScheme(scheme))
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	testBackup := createTestBackup("test-backup", namespace)
	generic := &Generic{
		Config: job.Config{
			Client: fakeClient,
			Obj:    testBackup,
		},
	}

	resultList := &k8upv1.BackupList{}
	err := generic.listOldResources(ctx, namespace, resultList, "backup_test-backup")
	require.NoError(t, err)
	assert.Len(t, resultList.Items, 0)
}

func TestGeneric_listOldResources_HandlesWrongNamespace(t *testing.T) {
	ctx := context.Background()
	targetNamespace := "target-namespace"
	wrongNamespace := "wrong-namespace"

	// Create backups in wrong namespace
	backupList := createBackupListWithOwnership("backup_test-backup", wrongNamespace, 2)

	scheme := runtime.NewScheme()
	require.NoError(t, k8upv1.AddToScheme(scheme))
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithLists(backupList).Build()

	testBackup := createTestBackup("test-backup", targetNamespace)
	generic := &Generic{
		Config: job.Config{
			Client: fakeClient,
			Obj:    testBackup,
		},
	}

	// Search in target namespace (should find nothing)
	resultList := &k8upv1.BackupList{}
	err := generic.listOldResources(ctx, targetNamespace, resultList, "backup_test-backup")
	require.NoError(t, err)
	assert.Len(t, resultList.Items, 0)
}

// Helper functions

func createTestBackup(name, namespace string) *k8upv1.Backup {
	return &k8upv1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(uuid.NewUUID()),
		},
		Spec: k8upv1.BackupSpec{},
	}
}

func createBackupListWithOwnership(ownedBy, namespace string, count int) *k8upv1.BackupList {
	backups := make([]k8upv1.Backup, count)
	for i := range backups {
		backups[i] = k8upv1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "backup-" + string(uuid.NewUUID()),
				Namespace: namespace,
				Labels: map[string]string{
					k8upv1.LabelK8upOwnedBy: ownedBy,
					k8upv1.LabelK8upType:    "backup",
				},
			},
			Spec: k8upv1.BackupSpec{},
		}
	}
	return &k8upv1.BackupList{Items: backups}
}

func createBackupListWithMixedStatus(ownedBy, namespace string, successful, failed, running int) *k8upv1.BackupList {
	var backups []k8upv1.Backup

	// Create successful backups
	for i := 0; i < successful; i++ {
		backup := k8upv1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "backup-successful-" + string(uuid.NewUUID()),
				Namespace: namespace,
				Labels: map[string]string{
					k8upv1.LabelK8upOwnedBy: ownedBy,
					k8upv1.LabelK8upType:    "backup",
				},
			},
			Spec: k8upv1.BackupSpec{},
			Status: k8upv1.Status{
				Conditions: []metav1.Condition{
					{
						Type:               k8upv1.ConditionCompleted.String(),
						Status:             metav1.ConditionTrue,
						Reason:             k8upv1.ReasonSucceeded.String(),
						LastTransitionTime: metav1.Now(),
					},
				},
			},
		}
		backups = append(backups, backup)
	}

	// Create failed backups
	for i := 0; i < failed; i++ {
		backup := k8upv1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "backup-failed-" + string(uuid.NewUUID()),
				Namespace: namespace,
				Labels: map[string]string{
					k8upv1.LabelK8upOwnedBy: ownedBy,
					k8upv1.LabelK8upType:    "backup",
				},
			},
			Spec: k8upv1.BackupSpec{},
			Status: k8upv1.Status{
				Conditions: []metav1.Condition{
					{
						Type:               k8upv1.ConditionCompleted.String(),
						Status:             metav1.ConditionTrue,
						Reason:             k8upv1.ReasonFailed.String(),
						LastTransitionTime: metav1.Now(),
					},
				},
			},
		}
		backups = append(backups, backup)
	}

	// Create running backups (no completion condition)
	for i := 0; i < running; i++ {
		backup := k8upv1.Backup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "backup-running-" + string(uuid.NewUUID()),
				Namespace: namespace,
				Labels: map[string]string{
					k8upv1.LabelK8upOwnedBy: ownedBy,
					k8upv1.LabelK8upType:    "backup",
				},
			},
			Spec:   k8upv1.BackupSpec{},
			Status: k8upv1.Status{}, // No conditions = running
		}
		backups = append(backups, backup)
	}

	return &k8upv1.BackupList{Items: backups}
}

type testLimiter struct {
	successfulLimit int
	failedLimit     int
}

func (t *testLimiter) GetSuccessfulJobsHistoryLimit() *int {
	return &t.successfulLimit
}

func (t *testLimiter) GetFailedJobsHistoryLimit() *int {
	return &t.failedLimit
}