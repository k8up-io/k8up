package scheduler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduler_SetSchedule(t *testing.T) {
	tests := map[string]struct {
		givenScheduleRef *scheduleRef
	}{
		"NoExistingSchedule": {
			givenScheduleRef: nil,
		},
		"UpdateSchedule": {
			givenScheduleRef: &scheduleRef{Schedule: "1 * * * *", EntryID: 1},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			s := newScheduler()
			defer s.cron.Stop()
			hasRun := false
			runnable := func(ctx context.Context) {
				hasRun = true
			}
			if tc.givenScheduleRef != nil {
				s.schedules.Store("key", tc.givenScheduleRef)
				entry, err := s.cron.AddFunc(tc.givenScheduleRef.Schedule.String(), func() {
					// irrelevant
				})
				assert.NoError(t, err)
				assert.Equal(t, tc.givenScheduleRef.EntryID, entry)
			}
			err := s.SetSchedule(context.TODO(), "key", "* * * * *", runnable)
			assert.NoError(t, err)
			rawResult, loaded := s.schedules.Load("key")
			require.True(t, loaded, "schedule present in map")
			result := rawResult.(*scheduleRef)
			result.Runnable(context.TODO())
			assert.True(t, hasRun, "function executed is the same")
			assert.Equal(t, "* * * * *", result.Schedule.String())
			assert.Len(t, s.cron.Entries(), 1, "amount of entries in crontab")
			assert.NotEqual(t, 1, s.cron.Entries()[0].ID)
		})
	}
}

func TestScheduler_RemoveSchedule(t *testing.T) {
	s := newScheduler()
	defer s.cron.Stop()
	s.schedules.Store("key", &scheduleRef{EntryID: 1})
	_, err := s.cron.AddFunc("1 * * * *", func() {})
	assert.NoError(t, err)
	s.RemoveSchedule(context.TODO(), "key")
	rawResult, loaded := s.schedules.Load("key")
	assert.False(t, loaded, "schedule present in map")
	assert.Nil(t, rawResult)
	assert.Len(t, s.cron.Entries(), 0, "no entry in crontab")
}
