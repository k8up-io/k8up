package integration

import (
	"path/filepath"
	"testing"

	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"

	k8upv1a1 "github.com/vshn/k8up/api/v1alpha1"
	// +kubebuilder:scaffold:imports
)

type EnvTestSuite struct {
	suite.Suite

	GivenClient *client.Client
	GivenConfig *rest.Config
	GivenEnv    *envtest.Environment
}

func (ts *EnvTestSuite) SetupSuite() {
	log.SetLogger(zapr.NewLogger(zaptest.NewLogger(ts.T())))

	testEnv := &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		CRDDirectoryPaths:     []string{filepath.Join("..", "testbin", "bin", "apiextensions.k8s.io", "v1")},
	}

	config, err := testEnv.Start()
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), config)

	registerCRDs(ts.T())

	k8sClient, err := client.New(config, client.Options{
		Scheme: scheme.Scheme,
	})
	require.NoError(ts.T(), err)
	require.NotNil(ts.T(), k8sClient)

	ts.GivenEnv = testEnv
	ts.GivenConfig = config
	ts.GivenClient = &k8sClient
}

func registerCRDs(t *testing.T) {
	require.NoError(t, batchv1.AddToScheme(scheme.Scheme))
	require.NoError(t, k8upv1a1.AddToScheme(scheme.Scheme))

	// +kubebuilder:scaffold:scheme
}

func (ts *EnvTestSuite) TearDownSuite() {
	err := ts.GivenEnv.Stop()
	require.NoError(ts.T(), err)
}
