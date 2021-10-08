package executor

import (
	"testing"

	"github.com/go-logr/zapr"
	v1 "github.com/k8up-io/k8up/api/v1"
	"github.com/k8up-io/k8up/operator/cfg"
	"github.com/k8up-io/k8up/operator/job"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBackupExecutor_setupEnvVars(t *testing.T) {
	tests := map[string]struct {
		givenSpec       v1.BackupSpec
		givenConfig     *cfg.Configuration
		expectedEnvVars []corev1.EnvVar
	}{
		"GivenEmptySpec_ThenExpectEmptyVariables": {
			givenSpec: v1.BackupSpec{},
			expectedEnvVars: []corev1.EnvVar{
				{Name: "STATS_URL", Value: ""},
				{Name: "PROM_URL", Value: ""},
			},
		},
		"GivenSpec_WhenGlobalDefined_ThenExpectGlobalVariable": {
			givenSpec: v1.BackupSpec{},
			givenConfig: &cfg.Configuration{
				GlobalStatsURL: "https://hostname:port/stats",
				PromURL:        "https://hostname:port/prom",
			},
			expectedEnvVars: []corev1.EnvVar{
				{Name: "STATS_URL", Value: "https://hostname:port/stats"},
				{Name: "PROM_URL", Value: "https://hostname:port/prom"},
			},
		},
		"GivenSpecWithSpecificValue_WhenGlobalDefined_ThenExpectSpecificVariable": {
			givenSpec: v1.BackupSpec{
				StatsURL: "https://custom:port/stats",
				PromURL:  "https://custom:port/prom",
			},
			givenConfig: &cfg.Configuration{
				GlobalStatsURL: "https://hostname:port/stats",
				PromURL:        "https://hostname:port/prom",
			},
			expectedEnvVars: []corev1.EnvVar{
				{Name: "STATS_URL", Value: "https://custom:port/stats"},
				{Name: "PROM_URL", Value: "https://custom:port/prom"},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			currentConfig := cfg.Config
			defer func() {
				cfg.Config = currentConfig
			}()
			backup := &v1.Backup{
				Spec: tt.givenSpec,
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "testNamespace",
				},
			}
			if tt.givenConfig != nil {
				cfg.Config = tt.givenConfig
			} else {
				cfg.Config = &cfg.Configuration{}
			}
			exec := NewBackupExecutor(job.Config{
				Log: zapr.NewLogger(zaptest.NewLogger(t)),
				Obj: backup,
			})
			exec.backup = backup
			result := exec.setupEnvVars()
			for _, expectedEnv := range tt.expectedEnvVars {
				assert.Contains(t, result, expectedEnv)
			}
		})
	}
}
