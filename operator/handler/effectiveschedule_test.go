package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/job"
)

func TestScheduleHandler_findExistingSchedule(t *testing.T) {
	tests := map[string]struct {
		givenEffectiveSchedules []k8upv1.EffectiveSchedule
		givenJobType            k8upv1.JobType
		expectedSchedule        k8upv1.ScheduleDefinition
		expectFind              bool
	}{
		"GivenNoExistingSchedule_WhenFind_ThenReturnEmptySchedule": {
			givenJobType:            k8upv1.PruneType,
			givenEffectiveSchedules: []k8upv1.EffectiveSchedule{},
			expectedSchedule:        "",
			expectFind:              false,
		},
		"GivenWrongSchedule_WhenFind_ThenReturnEmptySchedule": {
			givenJobType: k8upv1.PruneType,
			givenEffectiveSchedules: []k8upv1.EffectiveSchedule{
				{JobType: k8upv1.BackupType},
			},
			expectedSchedule: "",
			expectFind:       false,
		},
		"GivenCorrectSchedule_WhenFind_ThenReturnSchedule": {
			givenJobType: k8upv1.BackupType,
			givenEffectiveSchedules: []k8upv1.EffectiveSchedule{
				{JobType: k8upv1.BackupType, GeneratedSchedule: "1 * * * *"},
			},
			expectedSchedule: "1 * * * *",
			expectFind:       true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := &ScheduleHandler{
				schedule: &k8upv1.Schedule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "schedule",
						Namespace: "default",
					},
					Status: k8upv1.ScheduleStatus{
						EffectiveSchedules: tt.givenEffectiveSchedules,
					},
				},
				Config: job.Config{Log: zap.New(zap.UseDevMode(true))},
			}
			schedule, found := s.findExistingSchedule(tt.givenJobType)

			assert.Equal(t, tt.expectedSchedule, schedule)
			assert.Equal(t, tt.expectFind, found)
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
