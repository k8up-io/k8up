package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vshn/k8up/operator/cfg"
)

func Test_loadEnvironmentVariables(t *testing.T) {
	tests := map[string]struct {
		givenKey     string
		givenValue   string
		assertConfig func(t *testing.T, c *cfg.Configuration)
	}{
		"GivenStringVariable_WhenLoading_ThenParseStringVar": {
			givenKey:   "BACKUP_PROMURL",
			givenValue: "https://prometheus:9090/metrics",
			assertConfig: func(t *testing.T, c *cfg.Configuration) {
				assert.Equal(t, "https://prometheus:9090/metrics", c.PromURL)
			},
		},
		"GivenNumericEnvironmentVariable_WhenLoading_ThenParseIntVar": {
			givenKey:   "BACKUP_GLOBALKEEPJOBS",
			givenValue: "2",
			assertConfig: func(t *testing.T, c *cfg.Configuration) {
				assert.Equal(t, 2, c.GlobalKeepJobs)
			},
		},
		"GivenBooleanEnvironmentVariable_WhenLoading_ThenParseBoolVar": {
			givenKey:   "BACKUP_ENABLE_LEADER_ELECTION",
			givenValue: "false",
			assertConfig: func(t *testing.T, c *cfg.Configuration) {
				assert.True(t, cfg.NewDefaultConfig().EnableLeaderElection) // To ensure it's not asserting the default
				assert.False(t, c.EnableLeaderElection)
			},
		},
	}
	cfg.Config.OperatorNamespace = "not-part-of-this-test"
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, os.Setenv(tt.givenKey, tt.givenValue))
			loadEnvironmentVariables()
			tt.assertConfig(t, cfg.Config)
		})
	}
}
