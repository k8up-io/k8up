package v1_test

import (
	"testing"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"

	"github.com/stretchr/testify/assert"
)

type limiter interface {
	GetSuccessfulJobsHistoryLimit() *int
	GetFailedJobsHistoryLimit() *int
}

var historyLimitTestCases = map[string]func(successful, failed, deprecatedKeep *int) limiter{
	"Archive": func(successful, failed, deprecatedKeep *int) limiter {
		return &k8upv1.Archive{Spec: k8upv1.ArchiveSpec{RestoreSpec: &k8upv1.RestoreSpec{SuccessfulJobsHistoryLimit: successful, FailedJobsHistoryLimit: failed, KeepJobs: deprecatedKeep}}}
	},
	"Backup": func(successful, failed, deprecatedKeep *int) limiter {
		return &k8upv1.Backup{Spec: k8upv1.BackupSpec{SuccessfulJobsHistoryLimit: successful, FailedJobsHistoryLimit: failed, KeepJobs: deprecatedKeep}}
	},
	"Check": func(successful, failed, deprecatedKeep *int) limiter {
		return &k8upv1.Check{Spec: k8upv1.CheckSpec{SuccessfulJobsHistoryLimit: successful, FailedJobsHistoryLimit: failed, KeepJobs: deprecatedKeep}}
	},
	"Prune": func(successful, failed, deprecatedKeep *int) limiter {
		return &k8upv1.Prune{Spec: k8upv1.PruneSpec{SuccessfulJobsHistoryLimit: successful, FailedJobsHistoryLimit: failed, KeepJobs: deprecatedKeep}}
	},
	"Restore": func(successful, failed, deprecatedKeep *int) limiter {
		return &k8upv1.Restore{Spec: k8upv1.RestoreSpec{SuccessfulJobsHistoryLimit: successful, FailedJobsHistoryLimit: failed, KeepJobs: deprecatedKeep}}
	},
}

func TestHistoryLimits(t *testing.T) {
	for name, createSpec := range historyLimitTestCases {
		t.Run(name, func(t *testing.T) {
			failedLimit := 1
			successLimit := 2
			keepJobs := 3

			t.Run("JobsHistoryLimit", func(t *testing.T) {
				limits := createSpec(&successLimit, &failedLimit, &keepJobs)
				assert.Equal(t, *limits.GetFailedJobsHistoryLimit(), failedLimit)
				assert.Equal(t, *limits.GetSuccessfulJobsHistoryLimit(), successLimit)
			})
			t.Run("fallback to deprecated KeepJobs", func(t *testing.T) {
				limits := createSpec(nil, nil, &keepJobs)
				assert.Equal(t, *limits.GetFailedJobsHistoryLimit(), keepJobs)
				assert.Equal(t, *limits.GetSuccessfulJobsHistoryLimit(), keepJobs)
			})
			t.Run("no fallback value", func(t *testing.T) {
				limits := createSpec(nil, nil, nil)
				var nilInt *int
				assert.Equal(t, limits.GetFailedJobsHistoryLimit(), nilInt)
				assert.Equal(t, limits.GetSuccessfulJobsHistoryLimit(), nilInt)
			})
		})
	}
}
