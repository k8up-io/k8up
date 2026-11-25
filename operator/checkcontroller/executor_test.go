package checkcontroller

import (
	"testing"

	"github.com/stretchr/testify/assert"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/job"
)

func TestCheck_setupArgs(t *testing.T) {
	originalConfig := cfg.Config
	defer func() { cfg.Config = originalConfig }()
	cfg.Config = &cfg.Configuration{PodVarDir: "/k8up"}

	tests := map[string]struct {
		givenSpec    k8upv1.CheckSpec
		expectedArgs []string
	}{
		"GivenEmptySpec_ThenOnlyCheckArgs": {
			givenSpec:    k8upv1.CheckSpec{},
			expectedArgs: []string{"-varDir", "/k8up", "-check"},
		},
		"GivenReadDataSubset_ThenPassReadDataSubsetArg": {
			givenSpec:    k8upv1.CheckSpec{ReadDataSubset: "1/8"},
			expectedArgs: []string{"-varDir", "/k8up", "-check", "-readDataSubset", "1/8"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			config := job.NewConfig(nil, &k8upv1.Check{Spec: tc.givenSpec}, "")
			e := NewCheckExecutor(config)

			assert.Equal(t, tc.expectedArgs, e.setupArgs())
		})
	}
}
