package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBackupScheduleGetScheduleWithNilScheduleCommon(t *testing.T) {
	// When a user specifies backup: with fields but no schedule:,
	// the embedded *ScheduleCommon is nil.
	// GetSchedule() must not panic in this case.
	bs := &BackupSchedule{
		ScheduleCommon: nil,
	}

	assert.NotPanics(t, func() {
		result := bs.GetSchedule()
		assert.Empty(t, string(result))
	})
}

func TestRestoreScheduleGetScheduleWithNilScheduleCommon(t *testing.T) {
	rs := &RestoreSchedule{
		ScheduleCommon: nil,
	}

	assert.NotPanics(t, func() {
		result := rs.GetSchedule()
		assert.Empty(t, string(result))
	})
}

func TestArchiveScheduleGetScheduleWithNilScheduleCommon(t *testing.T) {
	as := &ArchiveSchedule{
		ScheduleCommon: nil,
	}

	assert.NotPanics(t, func() {
		result := as.GetSchedule()
		assert.Empty(t, string(result))
	})
}

func TestCheckScheduleGetScheduleWithNilScheduleCommon(t *testing.T) {
	cs := &CheckSchedule{
		ScheduleCommon: nil,
	}

	assert.NotPanics(t, func() {
		result := cs.GetSchedule()
		assert.Empty(t, string(result))
	})
}

func TestPruneScheduleGetScheduleWithNilScheduleCommon(t *testing.T) {
	ps := &PruneSchedule{
		ScheduleCommon: nil,
	}

	assert.NotPanics(t, func() {
		result := ps.GetSchedule()
		assert.Empty(t, string(result))
	})
}
