package locker

import (
	"context"
	"testing"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
