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

func (c *Configuration) WithOptions(f func(c *Configuration)) *Configuration {
	f(c)
	return c
}
