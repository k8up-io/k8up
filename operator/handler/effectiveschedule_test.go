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
		givenEffectiveSchedules map[k8upv1.JobType]k8upv1.EffectiveSchedule
		givenJobType            k8upv1.JobType
		expectedSchedule        k8upv1.ScheduleDefinition
		expectFind              bool
	}{
		"GivenNoExistingSchedule_WhenFind_ThenReturnEmptySchedule": {
			givenJobType:            k8upv1.PruneType,
			givenEffectiveSchedules: map[k8upv1.JobType]k8upv1.EffectiveSchedule{},
			expectedSchedule:        "",
			expectFind:              false,
		},
		"GivenWrongSchedule_WhenFind_ThenReturnEmptySchedule": {
			givenJobType: k8upv1.PruneType,
			givenEffectiveSchedules: map[k8upv1.JobType]k8upv1.EffectiveSchedule{
				k8upv1.BackupType: {},
			},
			expectedSchedule: "",
			expectFind:       false,
		},
		"GivenCorrectSchedule_WhenFind_ThenReturnSchedule": {
			givenJobType: k8upv1.BackupType,
			givenEffectiveSchedules: map[k8upv1.JobType]k8upv1.EffectiveSchedule{
				k8upv1.BackupType: {
					Spec: k8upv1.EffectiveScheduleSpec{
						JobType:           k8upv1.BackupType,
						GeneratedSchedule: "1 * * * *",
						ScheduleRefs: []k8upv1.ScheduleRef{
							{Name: "schedule", Namespace: "default"},
						},
						OriginalSchedule: ScheduleDailyRandom,
					},
				},
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
				},
				Config:             job.Config{Log: zap.New(zap.UseDevMode(true))},
				effectiveSchedules: tt.givenEffectiveSchedules,
			}
			schedule, found := s.findExistingSchedule(tt.givenJobType, ScheduleDailyRandom)

			assert.Equal(t, tt.expectedSchedule, schedule)
			assert.Equal(t, tt.expectFind, found)
		})
	}
}
