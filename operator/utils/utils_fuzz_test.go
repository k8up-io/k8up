package utils

import (
	"testing"
)

func FuzzJsonArgsArrayUnmarshalJSON(f *testing.F) {
	// Seed with realistic inputs
	f.Add([]byte(`"--verbose"`))
	f.Add([]byte(`["--verbose", "--dry-run"]`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`""`))
	f.Add([]byte(`null`))
	f.Add([]byte(`123`))
	f.Add([]byte(`{"key": "value"}`))
	f.Add([]byte(`[1, 2, 3]`))
	f.Add([]byte(`[null]`))
	f.Add([]byte(``))

	f.Fuzz(func(t *testing.T, data []byte) {
		var arr JsonArgsArray
		// Must not panic — errors are acceptable
		_ = arr.UnmarshalJSON(data)
	})
}
