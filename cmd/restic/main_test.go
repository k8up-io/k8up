package restic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

// A required flag must show up in `k8up restic --help`, otherwise users hit
// `Required flag "hostname" not set` without any hint that the flag exists.
func TestCommand_HostnameFlagIsVisibleAndRequired(t *testing.T) {
	for _, f := range Command.Flags {
		sf, ok := f.(*cli.StringFlag)
		if !ok || sf.Name != "hostname" {
			continue
		}
		assert.False(t, sf.Hidden, "required flag --hostname must not be hidden")
		assert.True(t, sf.Required)
		assert.Contains(t, sf.EnvVars, "HOSTNAME")
		return
	}
	require.Fail(t, "flag hostname not found")
}
