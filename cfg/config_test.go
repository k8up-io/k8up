package cfg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Configuration_ValidateSyntax(t *testing.T) {
	type assertConfig = func(t *testing.T, c *Configuration)
	tests := map[string]struct {
		givenConfig       *Configuration
		assertConfig      assertConfig
		expectErr         bool
		containErrMessage string
	}{
		"GivenDefaultConfig_WhenOperatorNamespaceEmpty_ThenExpectError": {
			givenConfig:       NewDefaultConfig(),
			expectErr:         true,
			containErrMessage: "operator namespace",
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
			tt.assertConfig(t, tt.givenConfig)
		})
	}
}

func Test_Configuration_DefaultConfig(t *testing.T) {
	c := NewDefaultConfig()
	err := c.ValidateSyntax()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "operator namespace")
}
