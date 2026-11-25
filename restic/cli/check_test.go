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
)

// recordingResticScript writes its arguments to the file given in ARGS_FILE
// and exits with the code given in EXIT_CODE.
const recordingResticScript = `#!/bin/sh
echo "$@" > "ARGS_FILE"
exit EXIT_CODE
`

func TestRestic_Check(t *testing.T) {
	tests := map[string]struct {
		readDataSubset string
		exitCode       string
		expectedArgs   string
		expectErr      bool
	}{
		"GivenNoSubset_ThenOnlyCheckStructure": {
			expectedArgs: "check",
		},
		"GivenFraction_ThenPassReadDataSubset": {
			readDataSubset: "1/8",
			expectedArgs:   "check --read-data-subset=1/8",
		},
		"GivenPercentage_ThenPassReadDataSubset": {
			readDataSubset: "10%",
			expectedArgs:   "check --read-data-subset=10%",
		},
		"GivenSize_ThenPassReadDataSubset": {
			readDataSubset: "500M",
			expectedArgs:   "check --read-data-subset=500M",
		},
		"GivenResticCheckFails_ThenReturnError": {
			readDataSubset: "1/8",
			exitCode:       "1",
			expectedArgs:   "check --read-data-subset=1/8",
			expectErr:      true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			argsFile := filepath.Join(dir, "args")
			exitCode := tc.exitCode
			if exitCode == "" {
				exitCode = "0"
			}
			script := strings.NewReplacer("ARGS_FILE", argsFile, "EXIT_CODE", exitCode).Replace(recordingResticScript)
			bin := filepath.Join(dir, "restic")
			require.NoError(t, os.WriteFile(bin, []byte(script), 0o755))

			r := &Restic{
				resticPath:  bin,
				logger:      logr.Discard(),
				ctx:         context.Background(),
				globalFlags: Flags{},
			}

			err := r.Check(tc.readDataSubset)

			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			args, readErr := os.ReadFile(argsFile)
			require.NoError(t, readErr)
			assert.Equal(t, tc.expectedArgs, strings.TrimSpace(string(args)))
		})
	}
}
