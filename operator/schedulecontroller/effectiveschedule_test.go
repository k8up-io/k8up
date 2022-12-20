package schedulecontroller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
)

func TestScheduleHandler_setEffectiveSchedule(t *testing.T) {
	tests := map[string]struct {
		givenEffectiveSchedules    []k8upv1.EffectiveSchedule
		givenJobType               k8upv1.JobType
		givenNewSchedule           k8upv1.ScheduleDefinition
		expectedEffectiveSchedules []k8upv1.EffectiveSchedule
	}{
		"GivenEmptyList_ThenAddNewEntry": {
			givenEffectiveSchedules: nil,
			givenJobType:            k8upv1.PruneType,
			givenNewSchedule:        "23 * * * *",
			expectedEffectiveSchedules: []k8upv1.EffectiveSchedule{
				{JobType: k8upv1.PruneType, GeneratedSchedule: "23 * * * *"},
			},
		},
		"WhenAlreadyExists_ThenUpdateEntry": {
			givenEffectiveSchedules: []k8upv1.EffectiveSchedule{
				{JobType: k8upv1.PruneType, GeneratedSchedule: "23 * * * *"},
				{JobType: k8upv1.BackupType, GeneratedSchedule: "⏰"},
			},
			givenJobType:     k8upv1.PruneType,
			givenNewSchedule: "* 2 * * *",
			expectedEffectiveSchedules: []k8upv1.EffectiveSchedule{
				{JobType: k8upv1.PruneType, GeneratedSchedule: "* 2 * * *"},
				{JobType: k8upv1.BackupType, GeneratedSchedule: "⏰"},
			},
		},
		"WhenListHasEntries_ThenAddNewEntry": {
			givenEffectiveSchedules: []k8upv1.EffectiveSchedule{
				{JobType: k8upv1.BackupType, GeneratedSchedule: "⏰"},
			},
			givenJobType:     k8upv1.PruneType,
			givenNewSchedule: "* 2 * * *",
			expectedEffectiveSchedules: []k8upv1.EffectiveSchedule{
				{JobType: k8upv1.BackupType, GeneratedSchedule: "⏰"},
				{JobType: k8upv1.PruneType, GeneratedSchedule: "* 2 * * *"},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			s := &ScheduleHandler{}
			s.schedule = &k8upv1.Schedule{Status: k8upv1.ScheduleStatus{EffectiveSchedules: tc.givenEffectiveSchedules}}
			s.Log = zap.New(zap.UseDevMode(true))
			s.setEffectiveSchedule(tc.givenJobType, tc.givenNewSchedule)
			assert.Equal(t, tc.expectedEffectiveSchedules, s.schedule.Status.EffectiveSchedules, "effective schedules")
		})
	}
}

func TestScheduleHandler_cleanupEffectiveSchedules(t *testing.T) {
	tests := map[string]struct {
		givenEffectiveSchedules    []k8upv1.EffectiveSchedule
		givenJobType               k8upv1.JobType
		givenNewSchedule           k8upv1.ScheduleDefinition
		expectedEffectiveSchedules []k8upv1.EffectiveSchedule
	}{
		"GivenEmptyList_DoNothing": {
			givenEffectiveSchedules:    nil,
			givenJobType:               k8upv1.PruneType,
			givenNewSchedule:           "",
			expectedEffectiveSchedules: []k8upv1.EffectiveSchedule{},
		},
		"GivenList_WhenTypeNotMatching_ThenKeepExisting": {
			givenEffectiveSchedules: []k8upv1.EffectiveSchedule{
				{JobType: k8upv1.BackupType, GeneratedSchedule: "1 * * * *"},
			},
			givenJobType:     k8upv1.PruneType,
			givenNewSchedule: "⏰",
			expectedEffectiveSchedules: []k8upv1.EffectiveSchedule{
				{JobType: k8upv1.BackupType, GeneratedSchedule: "1 * * * *"},
			},
		},
		"GivenList_WhenScheduleNotRandom_ThenRemoveIt": {
			givenEffectiveSchedules: []k8upv1.EffectiveSchedule{
				{JobType: k8upv1.BackupType, GeneratedSchedule: "1 * * * *"},
				{JobType: k8upv1.PruneType, GeneratedSchedule: "3 * * * *"},
			},
			givenJobType:     k8upv1.BackupType,
			givenNewSchedule: "2 * * * *",
			expectedEffectiveSchedules: []k8upv1.EffectiveSchedule{
				{JobType: k8upv1.PruneType, GeneratedSchedule: "3 * * * *"},
			},
		},
		"GivenList_WhenScheduleRandom_ThenKeepIt": {
			givenEffectiveSchedules: []k8upv1.EffectiveSchedule{
				{JobType: k8upv1.BackupType, GeneratedSchedule: "1 * * * *"},
				{JobType: k8upv1.PruneType, GeneratedSchedule: "3 * * * *"},
			},
			givenJobType:     k8upv1.BackupType,
			givenNewSchedule: "@daily-random",
			expectedEffectiveSchedules: []k8upv1.EffectiveSchedule{
				{JobType: k8upv1.BackupType, GeneratedSchedule: "1 * * * *"},
				{JobType: k8upv1.PruneType, GeneratedSchedule: "3 * * * *"},
			},
		},
		"GivenList_WhenScheduleEmpty_ThenRemoveIt": {
			givenEffectiveSchedules: []k8upv1.EffectiveSchedule{
				{JobType: k8upv1.BackupType, GeneratedSchedule: "1 * * * *"},
				{JobType: k8upv1.PruneType, GeneratedSchedule: "3 * * * *"},
			},
			givenJobType:     k8upv1.BackupType,
			givenNewSchedule: "",
			expectedEffectiveSchedules: []k8upv1.EffectiveSchedule{
				{JobType: k8upv1.PruneType, GeneratedSchedule: "3 * * * *"},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			s := &ScheduleHandler{}
			s.schedule = &k8upv1.Schedule{Status: k8upv1.ScheduleStatus{EffectiveSchedules: tc.givenEffectiveSchedules}}
			s.Log = zap.New(zap.UseDevMode(true))
			s.cleanupEffectiveSchedules(tc.givenJobType, tc.givenNewSchedule)
			assert.Equal(t, tc.expectedEffectiveSchedules, s.schedule.Status.EffectiveSchedules)
		})
	}
}
