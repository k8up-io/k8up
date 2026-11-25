package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k8up-io/k8up/v2/restic/cfg"
)

// fakeResticScript answers "snapshots" and "ls" like restic does and lets the
// "restore" subcommand exit with the given code.
const fakeResticScript = `#!/bin/sh
case "$1" in
snapshots)
	echo '[{"id":"abc123","time":"2026-10-01T00:00:00Z","paths":["/data/pvc"]}]'
	;;
ls)
	echo '{"id":"abc123","paths":["/data/pvc"],"struct_type":"snapshot"}'
	echo '{"type":"dir","path":"/data/pvc","struct_type":"node"}'
	;;
restore)
	echo "restore failed" >&2
	exit RESTORE_EXIT_CODE
	;;
esac
`

// failingVerifyScript behaves like restic when the restored data does not match
// the snapshot: "restore --verify" exits non-zero, a plain "restore" succeeds.
const failingVerifyScript = `#!/bin/sh
case "$1" in
snapshots)
	echo '[{"id":"abc123","time":"2026-10-01T00:00:00Z","paths":["/data/pvc"]}]'
	;;
ls)
	echo '{"id":"abc123","paths":["/data/pvc"],"struct_type":"snapshot"}'
	echo '{"type":"dir","path":"/data/pvc","struct_type":"node"}'
	;;
restore)
	for arg in "$@"; do
		if [ "$arg" = "--verify" ]; then
			echo "verification failed" >&2
			exit 1
		fi
	done
	;;
esac
`

func newFakeRestic(t *testing.T, restoreExitCode string, stats StatsHandler) *Restic {
	t.Helper()
	return newFakeResticFromScript(t, strings.ReplaceAll(fakeResticScript, "RESTORE_EXIT_CODE", restoreExitCode), stats)
}

func newFakeResticFromScript(t *testing.T, script string, stats StatsHandler) *Restic {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "restic")
	scriptBytes := []byte(script)
	require.NoError(t, os.WriteFile(bin, scriptBytes, 0o755))

	return &Restic{
		resticPath:   bin,
		logger:       logr.Discard(),
		ctx:          context.Background(),
		globalFlags:  Flags{},
		statsHandler: stats,
	}
}

func TestRestore_FolderRestore(t *testing.T) {
	tests := map[string]struct {
		restoreExitCode string
		expectErr       bool
		expectWebhook   bool
	}{
		"GivenResticRestoreSucceeds_ThenReturnNilAndSendWebhook": {
			restoreExitCode: "0",
			expectErr:       false,
			expectWebhook:   true,
		},
		"GivenResticRestoreFails_ThenReturnErrorAndSkipWebhook": {
			restoreExitCode: "1",
			expectErr:       true,
			expectWebhook:   false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			originalConfig := cfg.Config
			defer func() { cfg.Config = originalConfig }()
			cfg.Config = &cfg.Configuration{RestoreTrimPath: true}

			stats := &mockStatsHandler{}
			r := newFakeRestic(t, tc.restoreExitCode, stats)

			err := r.Restore("abc123", RestoreOptions{
				RestoreType: FolderRestore,
				RestoreDir:  t.TempDir(),
			}, nil, nil)

			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expectWebhook, stats.webhookCalled)
		})
	}
}

func TestRestore_FolderRestoreVerify(t *testing.T) {
	tests := map[string]struct {
		verify        bool
		expectErr     bool
		expectWebhook bool
	}{
		"GivenVerifyDisabled_ThenRestoreSucceeds": {
			verify:        false,
			expectErr:     false,
			expectWebhook: true,
		},
		"GivenVerifyEnabledAndVerificationFails_ThenReturnErrorAndSkipWebhook": {
			verify:        true,
			expectErr:     true,
			expectWebhook: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			originalConfig := cfg.Config
			defer func() { cfg.Config = originalConfig }()
			cfg.Config = &cfg.Configuration{RestoreTrimPath: true}

			stats := &mockStatsHandler{}
			r := newFakeResticFromScript(t, failingVerifyScript, stats)

			err := r.Restore("abc123", RestoreOptions{
				RestoreType: FolderRestore,
				RestoreDir:  t.TempDir(),
				Verify:      tc.verify,
			}, nil, nil)

			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expectWebhook, stats.webhookCalled)
		})
	}
}
