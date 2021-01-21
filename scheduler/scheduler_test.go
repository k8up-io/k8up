package scheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/job"
)

func TestScheduler_SyncSchedules(t *testing.T) {
	ns := "test-namespace"
	obj := &k8upv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      "my-backup",
		},
	}
	nsName := types.NamespacedName{
		Namespace: obj.Namespace,
		Name:      obj.Name,
	}
	tests := map[string]struct {
		expectErr            bool
		existingJobs         []Job
		expectedScheduleRefs []scheduleRef
		newJobs              []Job
	}{
		"GivenExistingSchedule_WhenReplacingScheduleWithExistingJobType_ThenReplaceTheSchedule": {
			existingJobs: []Job{
				{JobType: k8upv1alpha1.PruneType, Schedule: "1 * * * *"},
			},
			newJobs: []Job{
				{JobType: k8upv1alpha1.PruneType, Schedule: "* * * * *"},
			},
			expectedScheduleRefs: []scheduleRef{
				{JobType: k8upv1alpha1.PruneType, Schedule: "* * * * *"},
			},
		},
		"GivenInexistentSchedule_WhenAddingWithNewSchedule_ThenAddSchedule": {
			newJobs: []Job{
				{JobType: k8upv1alpha1.PruneType, Schedule: "* * * * *"},
			},
			expectedScheduleRefs: []scheduleRef{
				{JobType: k8upv1alpha1.PruneType, Schedule: "* * * * *"},
			},
		},
		"GivenExistingSchedule_WhenReplacingUnmatchedSchedule_ThenAddSchedule": {
			existingJobs: []Job{
				{JobType: k8upv1alpha1.PruneType, Schedule: "1 * * * *"},
			},
			newJobs: []Job{
				{JobType: k8upv1alpha1.PruneType, Schedule: "1 * * * *"},
				{JobType: k8upv1alpha1.ArchiveType, Schedule: "* * * * *"},
			},
			expectedScheduleRefs: []scheduleRef{
				{JobType: k8upv1alpha1.PruneType, Schedule: "1 * * * *"},
				{JobType: k8upv1alpha1.ArchiveType, Schedule: "* * * * *"},
			},
		},
		"GivenExistingSchedule_WhenSyncingSchedules_ThenRemoveInexistentSchedule": {
			existingJobs: []Job{
				{JobType: k8upv1alpha1.PruneType, Schedule: "1 * * * *"},
				{JobType: k8upv1alpha1.ArchiveType, Schedule: "* * * * *"},
			},
			newJobs: []Job{
				{JobType: k8upv1alpha1.PruneType, Schedule: "1 * * * *"},
			},
			expectedScheduleRefs: []scheduleRef{
				{JobType: k8upv1alpha1.PruneType, Schedule: "1 * * * *"},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := newScheduler()
			jobList := JobList{
				Jobs: tt.newJobs,
				Config: job.Config{
					Log: zap.New(zap.UseDevMode(true)),
					Obj: obj,
				},
			}
			for _, jb := range tt.existingJobs {
				require.NoError(t, s.addSchedule(jb, nsName, func() {

				}))
			}
			err := s.SyncSchedules(jobList)
			if tt.expectErr {
				assert.Error(t, err)
				return
			} else {
				assert.NoError(t, err)
			}
			scheduleRefs := s.registeredSchedules[types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}.String()]
			require.Len(t, scheduleRefs, len(tt.expectedScheduleRefs))
			for i, ref := range tt.expectedScheduleRefs {
				assert.Equal(t, ref.Schedule, scheduleRefs[i].Schedule)
				assert.Equal(t, ref.JobType, scheduleRefs[i].JobType)
			}
		})
	}
}

func Test_generateName(t *testing.T) {
	tests := map[string]struct {
		jobType        k8upv1alpha1.JobType
		prefix         string
		expectedPrefix string
	}{
		"GivenShortPrefix_WhenGenerate_ThenUseFullPrefix": {
			jobType:        k8upv1alpha1.ArchiveType,
			prefix:         "my-schedule",
			expectedPrefix: "my-schedule-archive-",
		},
		"GivenLongPrefix_WhenGenerate_ThenShortenPrefix": {
			jobType:        k8upv1alpha1.ArchiveType,
			prefix:         "my-schedule-with-a-really-long-name-that-could-clash-with-max-length",
			expectedPrefix: "my-schedule-with-a-really-long-name-that-could-cl-archive-",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			name := generateName(tt.jobType, tt.prefix)
			assert.Contains(t, name, tt.expectedPrefix)
			assert.LessOrEqual(t, len(name), 63)
			assert.Equal(t, len(name), len(tt.expectedPrefix)+5)
		})
	}
}
