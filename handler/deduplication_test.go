package handler

import (
	"testing"

	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/job"
)

func TestScheduleHandler_searchExistingSchedulesForDeduplication(t *testing.T) {
	tests := map[string]struct {
		givenEffectiveSchedules   []k8upv1alpha1.EffectiveSchedule
		givenBackend              string
		givenJobType              k8upv1alpha1.JobType
		givenOriginalSchedule     k8upv1alpha1.ScheduleDefinition
		expectedEffectiveSchedule k8upv1alpha1.EffectiveSchedule
		expectedResult            bool
	}{
		"GivenNoExistingSchedules_WhenSearch_ThenReturnFalse": {
			givenEffectiveSchedules: []k8upv1alpha1.EffectiveSchedule{},
			expectedResult:          false,
		},
		"GivenMatchingExistingSchedules_WhenSearch_ThenReturnTrue": {
			givenEffectiveSchedules: []k8upv1alpha1.EffectiveSchedule{
				newTestEffectiveSchedule("schedule"),
			},
			expectedEffectiveSchedule: newTestEffectiveSchedule("schedule"),
			givenJobType:              k8upv1alpha1.CheckType,
			givenBackend:              "s3:https://endpoint/bucket",
			givenOriginalSchedule:     ScheduleDailyRandom,
			expectedResult:            true,
		},
		"GivenNonMatchingExistingSchedules_WhenSearch_ThenReturnFalse": {
			givenEffectiveSchedules: []k8upv1alpha1.EffectiveSchedule{
				newTestEffectiveSchedule("schedule"),
			},
			givenJobType:          k8upv1alpha1.PruneType,
			givenBackend:          "s3:https://endpoint/bucket",
			givenOriginalSchedule: ScheduleDailyRandom,
			expectedResult:        false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := &ScheduleHandler{
				Config: job.Config{Log: zapr.NewLogger(zaptest.NewLogger(t))},
			}
			ctx := &deduplicationContext{
				originalSchedule: tt.givenOriginalSchedule,
				jobType:          tt.givenJobType,
				backendString:    tt.givenBackend,
			}
			result, found := s.searchExistingSchedulesForDeduplication(tt.givenEffectiveSchedules, ctx)
			assert.Equal(t, tt.expectedResult, found)
			if tt.expectedResult {
				assert.Equal(t, tt.expectedEffectiveSchedule, result)
			}
		})
	}
}

func TestScheduleHandler_isDeduplicated(t *testing.T) {
	tests := map[string]struct {
		givenSchedule           *k8upv1alpha1.Schedule
		givenEffectiveSchedules map[k8upv1alpha1.JobType]k8upv1alpha1.EffectiveSchedule
		expectedResult          bool
	}{
		"GivenNoEffectiveSchedulesWithStandardSchedule_ThenReturnFalse": {
			givenSchedule:           newTestSchedule("* * * * *"),
			givenEffectiveSchedules: map[k8upv1alpha1.JobType]k8upv1alpha1.EffectiveSchedule{},
			expectedResult:          false,
		},
		"GivenNoEffectiveSchedules_ThenReturnFalse": {
			givenSchedule:           newTestSchedule(ScheduleDailyRandom),
			givenEffectiveSchedules: map[k8upv1alpha1.JobType]k8upv1alpha1.EffectiveSchedule{},
			expectedResult:          false,
		},
		"GivenMatchingEffectiveSchedules_WhenOnlyOneScheduleRef_ThenReturnFalse": {
			givenSchedule: newTestSchedule(ScheduleDailyRandom),
			givenEffectiveSchedules: map[k8upv1alpha1.JobType]k8upv1alpha1.EffectiveSchedule{
				k8upv1alpha1.CheckType: newTestEffectiveSchedule("test-schedule"),
			},
			expectedResult: false,
		},
		"GivenMatchingEffectiveSchedules_WhenRefInList_ThenReturnTrue": {
			givenSchedule: newTestSchedule(ScheduleDailyRandom),
			givenEffectiveSchedules: map[k8upv1alpha1.JobType]k8upv1alpha1.EffectiveSchedule{
				k8upv1alpha1.CheckType: newTestEffectiveSchedule("ignore-ref", "test-schedule"),
			},
			expectedResult: true,
		},
		"GivenNonMatchingEffectiveSchedules_ThenReturnFalse": {
			givenSchedule: newTestSchedule(ScheduleDailyRandom),
			givenEffectiveSchedules: map[k8upv1alpha1.JobType]k8upv1alpha1.EffectiveSchedule{
				k8upv1alpha1.CheckType: newTestEffectiveSchedule("test-schedule"),
			},
			expectedResult: false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := &ScheduleHandler{
				Config:             job.Config{Log: zapr.NewLogger(zaptest.NewLogger(t))},
				schedule:           tt.givenSchedule,
				effectiveSchedules: tt.givenEffectiveSchedules,
			}
			ctx := &deduplicationContext{
				originalSchedule: tt.givenSchedule.Spec.Check.Schedule,
				backendString:    tt.givenSchedule.Spec.Backend.String(),
				jobType:          k8upv1alpha1.CheckType,
			}
			result := s.isDeduplicated(ctx)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestScheduleHandler_isFirstSchedule(t *testing.T) {
	tests := map[string]struct {
		givenEffectiveSchedules map[k8upv1alpha1.JobType]k8upv1alpha1.EffectiveSchedule
		givenBackend            string
		givenOriginalSchedule   k8upv1alpha1.ScheduleDefinition
		expectedResult          bool
	}{
		"GivenNoEffectiveSchedules_ThenReturnFalse": {
			givenEffectiveSchedules: map[k8upv1alpha1.JobType]k8upv1alpha1.EffectiveSchedule{},
			expectedResult:          false,
		},
		"GivenMatchingEffectiveSchedules_WhenRefNotPresent_ThenReturnFalse": {
			givenEffectiveSchedules: map[k8upv1alpha1.JobType]k8upv1alpha1.EffectiveSchedule{
				k8upv1alpha1.CheckType: newTestEffectiveSchedule(),
			},
			expectedResult: false,
		},
		"GivenMatchingEffectiveSchedules_WhenRefFirst_ThenReturnTrue": {
			givenEffectiveSchedules: map[k8upv1alpha1.JobType]k8upv1alpha1.EffectiveSchedule{
				k8upv1alpha1.CheckType: newTestEffectiveSchedule("test-schedule"),
			},
			givenBackend:          "s3:https://endpoint/bucket",
			givenOriginalSchedule: ScheduleDailyRandom,
			expectedResult:        true,
		},
		"GivenMatchingEffectiveSchedules_WhenRefSecond_ThenReturnFalse": {
			givenEffectiveSchedules: map[k8upv1alpha1.JobType]k8upv1alpha1.EffectiveSchedule{
				k8upv1alpha1.CheckType: newTestEffectiveSchedule("ignore-ref", "test-schedule"),
			},
			givenBackend:          "s3:https://endpoint/bucket",
			givenOriginalSchedule: ScheduleDailyRandom,
			expectedResult:        false,
		},
		"GivenNonMatchingEffectiveSchedules_WhenRefFirst_ThenReturnFalse": {
			givenEffectiveSchedules: map[k8upv1alpha1.JobType]k8upv1alpha1.EffectiveSchedule{
				k8upv1alpha1.CheckType: newTestEffectiveSchedule("test-schedule"),
			},
			givenBackend:          "not-matching",
			givenOriginalSchedule: ScheduleDailyRandom,
			expectedResult:        false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := &ScheduleHandler{
				effectiveSchedules: tt.givenEffectiveSchedules,
				schedule:           newTestSchedule(ScheduleDailyRandom),
			}
			ctx := &deduplicationContext{
				backendString:    tt.givenBackend,
				originalSchedule: tt.givenOriginalSchedule,
				jobType:          k8upv1alpha1.CheckType,
			}
			result := s.isFirstSchedule(ctx)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func newTestSchedule(schedule k8upv1alpha1.ScheduleDefinition) *k8upv1alpha1.Schedule {
	return &k8upv1alpha1.Schedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-schedule",
			Namespace: "default",
		},
		Spec: k8upv1alpha1.ScheduleSpec{
			Check: &k8upv1alpha1.CheckSchedule{
				CheckSpec: k8upv1alpha1.CheckSpec{},
				ScheduleCommon: &k8upv1alpha1.ScheduleCommon{
					Schedule: schedule,
				},
			},
			Backend: &k8upv1alpha1.Backend{
				S3: &k8upv1alpha1.S3Spec{
					Endpoint: "https://endpoint",
					Bucket:   "bucket",
				},
			},
		},
	}
}

func newTestEffectiveSchedule(refs ...string) k8upv1alpha1.EffectiveSchedule {
	es := k8upv1alpha1.EffectiveSchedule{
		Spec: k8upv1alpha1.EffectiveScheduleSpec{
			GeneratedSchedule: "1 2 * * *",
			OriginalSchedule:  ScheduleDailyRandom,
			JobType:           k8upv1alpha1.CheckType,
			BackendString:     "s3:https://endpoint/bucket",
		},
	}
	for _, ref := range refs {
		es.Spec.ScheduleRefs = append(es.Spec.ScheduleRefs, k8upv1alpha1.ScheduleRef{
			Name:      ref,
			Namespace: "default",
		})
	}
	return es
}
