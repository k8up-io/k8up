package cfg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Configuration_ValidateSyntax(t *testing.T) {
	tests := map[string]struct {
		givenConfig       *Configuration
		expectErr         bool
		containErrMessage string
	}{
		"GivenDefaultConfig_WhenOperatorNamespaceEmpty_ThenExpectError": {
			givenConfig:       NewDefaultConfig(),
			expectErr:         true,
			containErrMessage: "operator namespace",
		},
		"GivenConfig_WhenResourceInvalid_ThenExpectError": {
			givenConfig: NewDefaultConfig().WithOptions(func(c *Configuration) {
				c.GlobalCPUResourceLimit = "invalid"
			}),
			expectErr:         true,
			containErrMessage: "cpu limit",
		},
		"GivenConfig_WhenResourceValid_ThenExpectError": {
			givenConfig: NewDefaultConfig().WithOptions(func(c *Configuration) {
				c.GlobalCPUResourceLimit = "20m"
				c.OperatorNamespace = "something-to-not-trigger-error"
			}),
			expectErr: false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tt.givenConfig.ValidateSyntax()
			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.containErrMessage)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func Test_Configuration_DefaultConfig(t *testing.T) {
	c := NewDefaultConfig()
	err := c.ValidateSyntax()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "operator namespace")
}

func Test_Configuration_GetGlobalFailedJobsHistoryLimit(t *testing.T) {
	t.Run("GlobalKeepJobsIfNotSet", func(t *testing.T) {
		c := NewDefaultConfig()
		c.GlobalFailedJobsHistoryLimit = -1
		c.GlobalKeepJobs = 12
		assert.Equal(t, c.GlobalKeepJobs, c.GetGlobalFailedJobsHistoryLimit())
	})
	t.Run("ReturnsGlobalFailedJobsHistoryLimitIfSet", func(t *testing.T) {
		c := NewDefaultConfig()
		c.GlobalFailedJobsHistoryLimit = 12
		assert.Equal(t, c.GlobalFailedJobsHistoryLimit, c.GetGlobalFailedJobsHistoryLimit())
	})
	t.Run("LimitsNegativeValuesToZero", func(t *testing.T) {
		c := NewDefaultConfig()
		c.GlobalFailedJobsHistoryLimit = -23
		c.GlobalKeepJobs = -23
		assert.Equal(t, 0, c.GetGlobalFailedJobsHistoryLimit())
	})
}

func Test_Configuration_GetGlobalSuccessfulJobsHistoryLimit(t *testing.T) {
	t.Run("GlobalKeepJobsIfNotSet", func(t *testing.T) {
		c := NewDefaultConfig()
		c.GlobalSuccessfulJobsHistoryLimit = -1
		c.GlobalKeepJobs = 17
		assert.Equal(t, c.GlobalKeepJobs, c.GetGlobalSuccessfulJobsHistoryLimit())
	})
	t.Run("ReturnsGlobalSuccessfulJobsHistoryLimitIfSet", func(t *testing.T) {
		c := NewDefaultConfig()
		c.GlobalSuccessfulJobsHistoryLimit = 17
		assert.Equal(t, c.GlobalSuccessfulJobsHistoryLimit, c.GetGlobalSuccessfulJobsHistoryLimit())
	})
	t.Run("LimitsNegativeValuesToZero", func(t *testing.T) {
		c := NewDefaultConfig()
		c.GlobalSuccessfulJobsHistoryLimit = -23
		c.GlobalKeepJobs = -23
		assert.Equal(t, 0, c.GetGlobalSuccessfulJobsHistoryLimit())
	})
}

func (c *Configuration) WithOptions(f func(c *Configuration)) *Configuration {
	f(c)
	return c
}
