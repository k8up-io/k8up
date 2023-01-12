package job

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSha256Hash(t *testing.T) {
	tests := map[string]struct {
		givenString  string
		goldenString string
	}{
		"EmptyString": {
			givenString:  "",
			goldenString: "",
		},
		"RepositoryS3": {
			givenString:  "s3:endpoint/bucket",
			goldenString: "03ae9513ea3ba4b6d7289c427503e85cb28c11da210f442f89a07093c22af8a",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			actual := Sha256Hash(tc.givenString)
			assert.Equal(t, tc.goldenString, actual)
			assert.LessOrEqual(t, len(actual), 63)
		})
	}
}
