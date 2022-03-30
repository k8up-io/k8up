package scheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	k8upv1 "github.com/k8up-io/k8up/api/v1"
	"github.com/k8up-io/k8up/operator/job"
)

func TestScheduler_SyncSchedules(t *testing.T) {
	tests := map[string]struct {
		expectErr            bool
		existingJobs         []Job
		expectedScheduleRefs []scheduleRef
		newJobs              []Job
	}{
		"GivenExistingSchedule_WhenReplacingScheduleWithExistingJobType_ThenReplaceTheSchedule": {
			existingJobs: []Job{
				{JobType: k8upv1.PruneType, Schedule: "1 * * * *"},
			},
			newJobs: []Job{
				{JobType: k8upv1.PruneType, Schedule: "* * * * *"},
			},
			expectedScheduleRefs: []scheduleRef{
				{JobType: k8upv1.PruneType, Schedule: "* * * * *"},
			},
		},
		"GivenNonExistentSchedule_WhenAddingWithNewSchedule_ThenAddSchedule": {
			newJobs: []Job{
				{JobType: k8upv1.PruneType, Schedule: "* * * * *"},
			},
			expectedScheduleRefs: []scheduleRef{
				{JobType: k8upv1.PruneType, Schedule: "* * * * *"},
			},
		},
		"GivenExistingSchedule_WhenReplacingUnmatchedSchedule_ThenAddSchedule": {
			existingJobs: []Job{
				{JobType: k8upv1.PruneType, Schedule: "1 * * * *"},
			},
			newJobs: []Job{
				{JobType: k8upv1.PruneType, Schedule: "1 * * * *"},
				{JobType: k8upv1.ArchiveType, Schedule: "* * * * *"},
			},
			expectedScheduleRefs: []scheduleRef{
				{JobType: k8upv1.PruneType, Schedule: "1 * * * *"},
				{JobType: k8upv1.ArchiveType, Schedule: "* * * * *"},
			},
		},
		"GivenExistingSchedule_WhenSyncingSchedules_ThenRemoveNonExistentSchedule": {
			existingJobs: []Job{
				{JobType: k8upv1.PruneType, Schedule: "1 * * * *"},
				{JobType: k8upv1.ArchiveType, Schedule: "* * * * *"},
			},
			newJobs: []Job{
				{JobType: k8upv1.PruneType, Schedule: "1 * * * *"},
			},
			expectedScheduleRefs: []scheduleRef{
				{JobType: k8upv1.PruneType, Schedule: "1 * * * *"},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := newScheduler()
			nsName, obj := newBackup()
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
			scheduleRefs := s.registeredSchedules[nsName.String()]
			require.Len(t, scheduleRefs, len(tt.expectedScheduleRefs))
			for i, ref := range tt.expectedScheduleRefs {
				assert.Equal(t, ref.Schedule, scheduleRefs[i].Schedule, "cron schedule")
				assert.Equal(t, ref.JobType, scheduleRefs[i].JobType, "job type")
			}
		})
	}
}

func Test_generateName(t *testing.T) {
	tests := map[string]struct {
		jobType        k8upv1.JobType
		prefix         string
		expectedPrefix string
	}{
		"GivenShortPrefix_WhenGenerate_ThenUseFullPrefix": {
			jobType:        k8upv1.ArchiveType,
			prefix:         "my-schedule",
			expectedPrefix: "my-schedule-archive-",
		},
		"GivenLongPrefix_WhenGenerate_ThenShortenPrefix": {
			jobType:        k8upv1.ArchiveType,
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

func newBackup() (types.NamespacedName, *k8upv1.Backup) {
	ns := "test-namespace-" + rand.String(5)
	obj := &k8upv1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      "my-backup",
		},
	}
	return types.NamespacedName{
		Namespace: obj.Namespace,
		Name:      obj.Name,
	}, obj
}
