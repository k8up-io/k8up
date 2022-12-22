package schedulecontroller

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
)

func Test_randomizeSchedule_VerifyCronSyntax(t *testing.T) {
	formatString := "%d in '%s' is not between %d and %d"
	arr := []int{0, 0, 0, 0, 0}
	t.Run("YearlySyntax", func(t *testing.T) {
		for i := 0; i < 200; i++ {
			seed := "namespace/name-" + strconv.Itoa(i) + "@backup"
			schedule, err := randomizeSchedule(seed, "@yearly-random")
			assert.NoError(t, err)
			fields := strings.Split(schedule.String(), " ")
			for j, f := range fields {
				number, _ := strconv.Atoi(f)
				arr[j] = number
			}
			assert.InDelta(t, 0, arr[0], 59.0, formatString, arr[0], "minute", 0, 59)
			assert.InDelta(t, 0, arr[1], 23.0, formatString, arr[1], "hour", 0, 23)
			assert.InDelta(t, 1, arr[2], 26.0, formatString, arr[2], "dayOfMonth", 1, 27)
			assert.InDelta(t, 1, arr[3], 11.0, formatString, arr[3], "month", 1, 12)
		}
	})
	t.Run("WeeklySyntax", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			seed := "namespace/name-" + strconv.Itoa(i) + "@backup"
			schedule, err := randomizeSchedule(seed, "@weekly-random")
			assert.NoError(t, err)
			fields := strings.Split(schedule.String(), " ")
			for j, f := range fields {
				number, _ := strconv.Atoi(f)
				arr[j] = number
			}
			assert.InDelta(t, 0, arr[0], 59.0, formatString, arr[0], "minute", 0, 59)
			assert.InDelta(t, 0, arr[1], 23.0, formatString, arr[1], "hour", 0, 23)
			assert.InDelta(t, 1, arr[4], 5.0, formatString, arr[4], "weekday", 0, 6)
		}
	})
}

func Test_randomizeSchedule_VerifySchedules(t *testing.T) {
	seed := "k8up-system/my-scheduled-backup@backup"
	tests := map[string]struct {
		schedule         k8upv1.ScheduleDefinition
		expectedSchedule k8upv1.ScheduleDefinition
	}{
		"WhenScheduleRandomHourlyGiven_ThenReturnStableRandomizedSchedule": {
			schedule:         "@hourly-random",
			expectedSchedule: "52 * * * *",
		},
		"WhenScheduleRandomDailyGiven_ThenReturnStableRandomizedSchedule": {
			schedule:         "@daily-random",
			expectedSchedule: "52 4 * * *",
		},
		"WhenScheduleRandomWeeklyGiven_ThenReturnStableRandomizedSchedule": {
			schedule:         "@weekly-random",
			expectedSchedule: "52 4 * * 4",
		},
		"WhenScheduleRandomMonthlyGiven_ThenReturnStableRandomizedSchedule": {
			schedule:         "@monthly-random",
			expectedSchedule: "52 4 26 * *",
		},
		"WhenScheduleRandomYearlyGiven_ThenReturnStableRandomizedSchedule": {
			schedule:         "@yearly-random",
			expectedSchedule: "52 4 26 5 *",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := randomizeSchedule(seed, tt.schedule)
			assert.Equal(t, tt.expectedSchedule, result)
			assert.NoError(t, err)
		})
	}
}
