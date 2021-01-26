package integration

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"

	// +kubebuilder:scaffold:imports

	k8upv1a1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/executor"
)

type EnvTestSuite struct {
	suite.Suite

	Client client.Client
	Config *rest.Config
	Env    *envtest.Environment
	Logger logr.Logger
	Ctx    context.Context
}

const NS = "default"

func (ts *EnvTestSuite) SetupSuite() {
	ts.Logger = zapr.NewLogger(zaptest.NewLogger(ts.T()))
	log.SetLogger(ts.Logger)

	ts.Ctx = context.Background()

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

	executor.GetExecutor()

	ts.Env = testEnv
	ts.Config = config
	ts.Client = k8sClient
}

func registerCRDs(t *testing.T) {
	require.NoError(t, batchv1.AddToScheme(scheme.Scheme))
	require.NoError(t, k8upv1a1.AddToScheme(scheme.Scheme))

	// +kubebuilder:scaffold:scheme
}

func (ts *EnvTestSuite) TearDownSuite() {
	err := ts.Env.Stop()
	require.NoError(ts.T(), err)
}

func (ts *EnvTestSuite) DeleteAllJobs() {
	list := new(batchv1.JobList)
	err := ts.Client.List(ts.Ctx, list)
	assert.NoError(ts.T(), err)

	ts.T().Logf("Deleting %d Jobs", len(list.Items))

	for _, j := range list.Items {
		err := ts.Client.Delete(ts.Ctx, &j) // DeleteAllOf seems not implemented in envtest
		assert.NoError(ts.T(), err)
	}
}

type AssertFunc func(timedCtx context.Context) (done bool, err error)

func (ts *EnvTestSuite) RepeatedAssert(timeout time.Duration, interval time.Duration, failureMsg string, assertFunc AssertFunc) {
	timedCtx, cancel := context.WithTimeout(ts.Ctx, timeout)
	defer cancel()

	i := 0
	for {
		select {
		case <-time.After(interval):
			i++
			done, err := assertFunc(timedCtx)
			require.NoError(ts.T(), err)
			if done {
				return
			}
		case <-timedCtx.Done():
			if failureMsg == "" {
				failureMsg = timedCtx.Err().Error()
			}

			assert.Failf(ts.T(), failureMsg, "Failed after %s (%d attempts)", timeout, i)
			return
		}
	}
}
