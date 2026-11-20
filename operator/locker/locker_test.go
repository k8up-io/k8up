package locker

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/job"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestLocker_IsConcurrentJobsLimitReached(t *testing.T) {
	tests := map[string]struct {
		givenLimit     int
		givenJobs      []batchv1.Job
		expectedResult bool
	}{
		"Unlimited_ExpectFalse": {
			givenLimit: 0,
			givenJobs: []batchv1.Job{
				{Status: batchv1.JobStatus{Active: 1}},
			},
			expectedResult: false,
		},
		"ActiveLowerThanLimit_ExpectFalse": {
			givenLimit: 4,
			givenJobs: []batchv1.Job{
				{Status: batchv1.JobStatus{Active: 1}},
				{Status: batchv1.JobStatus{Active: 2}},
			},
			expectedResult: false,
		},
		"ActiveEqualsLimit_ExpectTrue": {
			givenLimit: 1,
			givenJobs: []batchv1.Job{
				{Status: batchv1.JobStatus{Active: 1}},
			},
			expectedResult: true,
		},
		"ActiveGreaterThanLimit_ExpectTrue": {
			givenLimit: 2,
			givenJobs: []batchv1.Job{
				{Status: batchv1.JobStatus{Active: 1}},
				{Status: batchv1.JobStatus{Active: 0}},
				{Status: batchv1.JobStatus{Active: 2}},
			},
			expectedResult: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			for i, batchJob := range tc.givenJobs {
				metav1.SetMetaDataLabel(&batchJob.ObjectMeta, k8upv1.LabelK8upType, k8upv1.BackupType.String())
				tc.givenJobs[i] = batchJob
			}
			result := isConcurrencyLimitReached(tc.givenJobs, k8upv1.BackupType, tc.givenLimit)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func TestLockerImpl_fetchRunningJobs(t *testing.T) {
	tests := map[string]struct {
		givenJobs    []batchv1.Job
		expectedJobs []batchv1.Job
	}{
		"NoneRunning": {
			givenJobs: []batchv1.Job{
				{Status: batchv1.JobStatus{Active: 0}},
				{Status: batchv1.JobStatus{Active: 0}},
			},
			expectedJobs: []batchv1.Job{},
		},
		"OneRunning": {
			givenJobs: []batchv1.Job{
				{Status: batchv1.JobStatus{Active: 0}},
				{Status: batchv1.JobStatus{Active: 1}},
			},
			expectedJobs: []batchv1.Job{
				{Status: batchv1.JobStatus{Active: 1}},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			oldFn := jobListFn
			defer func() {
				// reset function, just to be safe
				jobListFn = oldFn
			}()
			jobListFn = func(*lockerImpl, context.Context, ...client.ListOption) (batchv1.JobList, error) {
				// fake response
				return batchv1.JobList{Items: tc.givenJobs}, nil
			}
			l := &lockerImpl{repository: "repository"}
			result, err := l.fetchRunningJobs(context.TODO())
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedJobs, result)
		})
	}
}

var testNow = time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)

// newTestJob returns a batch Job with the given number of active and ready pods that was started the given duration before testNow.
// A nil ready means the cluster doesn't report ready pods.
func newTestJob(name string, jobType k8upv1.JobType, exclusive bool, active int32, ready *int32, startedAgo time.Duration) batchv1.Job {
	batchJob := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels:    map[string]string{k8upv1.LabelK8upType: jobType.String()},
		},
		Status: batchv1.JobStatus{
			Active:    active,
			Ready:     ready,
			StartTime: ptr.To(metav1.NewTime(testNow.Add(-startedAgo))),
		},
	}
	if exclusive {
		batchJob.Labels[job.K8upExclusive] = "true"
	}
	return batchJob
}

func TestIsUsingRepository(t *testing.T) {
	tests := map[string]struct {
		givenJob       batchv1.Job
		expectedResult bool
	}{
		"NoActivePods_ExpectFalse": {
			givenJob:       newTestJob("job", k8upv1.BackupType, false, 0, ptr.To(int32(0)), time.Minute),
			expectedResult: false,
		},
		"ReadyPod_ExpectTrue": {
			givenJob:       newTestJob("job", k8upv1.BackupType, false, 1, ptr.To(int32(1)), time.Minute),
			expectedResult: true,
		},
		"ReadyPodPastGracePeriod_ExpectTrue": {
			givenJob:       newTestJob("job", k8upv1.BackupType, false, 1, ptr.To(int32(1)), 5*time.Hour),
			expectedResult: true,
		},
		"PendingPodWithinGracePeriod_ExpectTrue": {
			givenJob:       newTestJob("job", k8upv1.BackupType, false, 1, ptr.To(int32(0)), time.Minute),
			expectedResult: true,
		},
		"PendingPodPastGracePeriod_ExpectFalse": {
			givenJob:       newTestJob("job", k8upv1.BackupType, false, 1, ptr.To(int32(0)), startupGracePeriod+time.Second),
			expectedResult: false,
		},
		"ReadyNotReported_ExpectTrue": {
			givenJob:       newTestJob("job", k8upv1.BackupType, false, 1, nil, 5*time.Hour),
			expectedResult: true,
		},
		"NoStartTime_ExpectTrue": {
			givenJob: func() batchv1.Job {
				batchJob := newTestJob("job", k8upv1.BackupType, false, 1, ptr.To(int32(0)), 0)
				batchJob.Status.StartTime = nil
				return batchJob
			}(),
			expectedResult: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expectedResult, isUsingRepository(tc.givenJob, testNow))
		})
	}
}

// quietCtx returns a context whose logger discards everything, so the locker doesn't log during tests.
func quietCtx() context.Context {
	return logr.NewContext(context.Background(), logr.Discard())
}

// fakeJobs makes the locker see the given jobs at testNow and restores the defaults when the test ends.
func fakeJobs(t *testing.T, jobs ...batchv1.Job) {
	oldListFn, oldNowFn := jobListFn, nowFn
	t.Cleanup(func() {
		jobListFn, nowFn = oldListFn, oldNowFn
	})
	jobListFn = func(*lockerImpl, context.Context, ...client.ListOption) (batchv1.JobList, error) {
		return batchv1.JobList{Items: jobs}, nil
	}
	nowFn = func() time.Time { return testNow }
}

func TestLockerImpl_TryRunExclusively(t *testing.T) {
	tests := map[string]struct {
		givenJobs   []batchv1.Job
		expectedRun bool
	}{
		"NoJobs_ExpectRun": {
			expectedRun: true,
		},
		"RunningBackup_ExpectDeferred": {
			givenJobs: []batchv1.Job{
				newTestJob("backup", k8upv1.BackupType, false, 1, ptr.To(int32(1)), time.Hour),
			},
			expectedRun: false,
		},
		"StartingBackup_ExpectDeferred": {
			givenJobs: []batchv1.Job{
				newTestJob("backup", k8upv1.BackupType, false, 1, ptr.To(int32(0)), 30*time.Second),
			},
			expectedRun: false,
		},
		"BackupPendingPastGracePeriod_ExpectRun": {
			givenJobs: []batchv1.Job{
				newTestJob("backup", k8upv1.BackupType, false, 1, ptr.To(int32(0)), time.Hour),
			},
			expectedRun: true,
		},
		"BackupPendingPastGracePeriodAndRunningBackup_ExpectDeferred": {
			givenJobs: []batchv1.Job{
				newTestJob("pending", k8upv1.BackupType, false, 1, ptr.To(int32(0)), time.Hour),
				newTestJob("running", k8upv1.BackupType, false, 1, ptr.To(int32(1)), time.Minute),
			},
			expectedRun: false,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fakeJobs(t, tc.givenJobs...)
			l := &lockerImpl{repository: "repository"}
			didRun := false
			ran, err := l.TryRunExclusively(quietCtx(), func(context.Context) error {
				didRun = true
				return nil
			})
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedRun, ran)
			assert.Equal(t, tc.expectedRun, didRun)
		})
	}
}

func TestLockerImpl_TryRun(t *testing.T) {
	tests := map[string]struct {
		givenJobs   []batchv1.Job
		givenLimit  int
		expectedRun bool
	}{
		"NoJobs_ExpectRun": {
			expectedRun: true,
		},
		"RunningPrune_ExpectDeferred": {
			givenJobs: []batchv1.Job{
				newTestJob("prune", k8upv1.PruneType, true, 1, ptr.To(int32(1)), time.Hour),
			},
			expectedRun: false,
		},
		"StartingPrune_ExpectDeferred": {
			givenJobs: []batchv1.Job{
				newTestJob("prune", k8upv1.PruneType, true, 1, ptr.To(int32(0)), 30*time.Second),
			},
			expectedRun: false,
		},
		"PrunePendingPastGracePeriod_ExpectRun": {
			givenJobs: []batchv1.Job{
				newTestJob("prune", k8upv1.PruneType, true, 1, ptr.To(int32(0)), time.Hour),
			},
			expectedRun: true,
		},
		"PendingBackupStillCountsTowardsConcurrencyLimit_ExpectDeferred": {
			givenJobs: []batchv1.Job{
				newTestJob("backup", k8upv1.BackupType, false, 1, ptr.To(int32(0)), time.Hour),
			},
			givenLimit:  1,
			expectedRun: false,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fakeJobs(t, tc.givenJobs...)
			l := &lockerImpl{repository: "repository"}
			config := job.Config{Obj: &k8upv1.Backup{}}
			didRun := false
			ran, err := l.TryRun(quietCtx(), config, tc.givenLimit, func(context.Context) error {
				didRun = true
				return nil
			})
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedRun, ran)
			assert.Equal(t, tc.expectedRun, didRun)
		})
	}
}
