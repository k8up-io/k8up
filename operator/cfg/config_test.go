package cfg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Configuration_GetGlobalFailedJobsHistoryLimit(t *testing.T) {
	t.Run("GlobalKeepJobsIfNotSet", func(t *testing.T) {
		c := Configuration{
			GlobalFailedJobsHistoryLimit: -5,
			GlobalKeepJobs:               17,
		}
		assert.Equal(t, 17, c.GetGlobalFailedJobsHistoryLimit())
	})
	t.Run("ReturnsGlobalFailedJobsHistoryLimitIfSet", func(t *testing.T) {
		c := Configuration{
			GlobalFailedJobsHistoryLimit: 17,
		}
		assert.Equal(t, 17, c.GetGlobalFailedJobsHistoryLimit())
	})
	t.Run("LimitsNegativeValuesToZero", func(t *testing.T) {
		c := Configuration{
			GlobalFailedJobsHistoryLimit: -5,
			GlobalKeepJobs:               -17,
		}
		assert.Equal(t, 0, c.GetGlobalFailedJobsHistoryLimit())
	})
}

func Test_Configuration_GetGlobalSuccessfulJobsHistoryLimit(t *testing.T) {
	t.Run("GlobalKeepJobsIfNotSet", func(t *testing.T) {
		c := Configuration{
			GlobalSuccessfulJobsHistoryLimit: -2,
			GlobalKeepJobs:                   17,
		}
		assert.Equal(t, 17, c.GetGlobalSuccessfulJobsHistoryLimit())
	})
	t.Run("ReturnsGlobalSuccessfulJobsHistoryLimitIfSet", func(t *testing.T) {
		c := Configuration{
			GlobalSuccessfulJobsHistoryLimit: 17,
		}
		assert.Equal(t, 17, c.GetGlobalSuccessfulJobsHistoryLimit())
	})
	t.Run("LimitsNegativeValuesToZero", func(t *testing.T) {
		c := Configuration{
			GlobalSuccessfulJobsHistoryLimit: -2,
			GlobalKeepJobs:                   -17,
		}
		assert.Equal(t, 0, c.GetGlobalSuccessfulJobsHistoryLimit())
	})
}
