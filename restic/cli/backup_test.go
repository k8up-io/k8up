package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/go-logr/logr/funcr"
	"github.com/stretchr/testify/assert"

	"github.com/k8up-io/k8up/v2/restic/cfg"
)

type mockStatsHandler struct {
	webhookCalled bool
}

func (m *mockStatsHandler) SendWebhook(p WebhookProvider) error {
	m.webhookCalled = true
	return nil
}

func (m *mockStatsHandler) SendPrometheus(p PrometheusProvider) error {
	return nil
}

func TestSendSnapshotList_SkipSnapshotSync(t *testing.T) {
	tests := map[string]struct {
		skipSnapshotSync     bool
		expectLogContains    string
		notExpectLogContains string
	}{
		"GivenSkipSnapshotSyncEnabled_ThenSkipKubernetesSync": {
			skipSnapshotSync:     true,
			expectLogContains:    "snapshot CR synchronization is disabled, skipping",
			notExpectLogContains: "cannot sync snapshots to the cluster",
		},
		"GivenSkipSnapshotSyncDisabled_ThenAttemptKubernetesSync": {
			skipSnapshotSync:     false,
			expectLogContains:    "cannot sync snapshots to the cluster",
			notExpectLogContains: "snapshot CR synchronization is disabled, skipping",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			originalConfig := cfg.Config
			defer func() { cfg.Config = originalConfig }()

			cfg.Config = &cfg.Configuration{
				SkipSnapshotSync: tc.skipSnapshotSync,
				Hostname:         "test-host",
				KubeConfig:       "/nonexistent/kubeconfig",
			}

			var logMessages []string
			logger := funcr.New(func(prefix, args string) {
				logMessages = append(logMessages, prefix+" "+args)
			}, funcr.Options{})

			mock := &mockStatsHandler{}
			r := &Restic{
				ctx:          context.Background(),
				logger:       logger,
				statsHandler: mock,
				bucket:       "test-bucket",
			}

			r.sendSnapshotList()

			assert.True(t, mock.webhookCalled, "webhook should always be called regardless of SkipSnapshotSync setting")

			combinedLogs := strings.Join(logMessages, "\n")
			assert.Contains(t, combinedLogs, tc.expectLogContains,
				"expected log message not found")
			assert.NotContains(t, combinedLogs, tc.notExpectLogContains,
				"unexpected log message found")
		})
	}
}
