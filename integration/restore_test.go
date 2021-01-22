// +build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/suite"

	k8upv1a1 "github.com/vshn/k8up/api/v1alpha1"
)

type RestoreTestSuite struct {
	EnvTestSuite

	GivenRestore *k8upv1a1.Restore
}

func Test_Restore(t *testing.T) {
	suite.Run(t, new(RestoreTestSuite))
}

func (r *RestoreTestSuite) TestFoo() {
	r.T().Log("Hilarious")
}
