package prunecontroller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/job"
)

func TestPruneExecutor_setupEnvVars_KeepDaily(t *testing.T) {
	tests := map[string]struct {
		retention         k8upv1.RetentionPolicy
		expectedKeepDaily string
		expectKeepDaily   bool
	}{
		"GivenNoRetention_ThenDefaultToKeepDaily14": {
			retention:         k8upv1.RetentionPolicy{},
			expectedKeepDaily: "14",
			expectKeepDaily:   true,
		},
		"GivenOnlyFilters_ThenDefaultToKeepDaily14": {
			retention: k8upv1.RetentionPolicy{
				Tags:      []string{"app"},
				Hostnames: []string{"namespace"},
			},
			expectedKeepDaily: "14",
			expectKeepDaily:   true,
		},
		"GivenKeepDaily_ThenUseKeepDaily": {
			retention:         k8upv1.RetentionPolicy{KeepDaily: 30},
			expectedKeepDaily: "30",
			expectKeepDaily:   true,
		},
		"GivenKeepLast_ThenOmitKeepDaily": {
			retention: k8upv1.RetentionPolicy{KeepLast: 5},
		},
		"GivenKeepHourly_ThenOmitKeepDaily": {
			retention: k8upv1.RetentionPolicy{KeepHourly: 24},
		},
		"GivenKeepWeekly_ThenOmitKeepDaily": {
			retention: k8upv1.RetentionPolicy{KeepWeekly: 4},
		},
		"GivenKeepMonthly_ThenOmitKeepDaily": {
			retention: k8upv1.RetentionPolicy{KeepMonthly: 6},
		},
		"GivenKeepYearly_ThenOmitKeepDaily": {
			retention: k8upv1.RetentionPolicy{KeepYearly: 1},
		},
		"GivenKeepTags_ThenOmitKeepDaily": {
			retention: k8upv1.RetentionPolicy{KeepTags: []string{"important"}},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			prune := &k8upv1.Prune{
				Spec: k8upv1.PruneSpec{
					Retention: tc.retention,
				},
			}
			e := NewPruneExecutor(job.NewConfig(nil, prune, ""))

			envVars := e.setupEnvVars(context.Background(), prune)

			keepDaily, found := findEnvVar(envVars, "KEEP_DAILY")
			assert.Equal(t, tc.expectKeepDaily, found)
			if tc.expectKeepDaily {
				assert.Equal(t, tc.expectedKeepDaily, keepDaily.Value)
			}
		})
	}
}

func findEnvVar(envVars []corev1.EnvVar, name string) (corev1.EnvVar, bool) {
	for _, envVar := range envVars {
		if envVar.Name == name {
			return envVar, true
		}
	}
	return corev1.EnvVar{}, false
}
