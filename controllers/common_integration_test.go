// +build integration

package controllers_test

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"

	// +kubebuilder:scaffold:imports

	k8upv1a1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/executor"
)

var InvalidNSNameCharacters = regexp.MustCompile("[^a-z0-9-]")

type EnvTestSuite struct {
	suite.Suite

	NS     string
	Client client.Client
	Config *rest.Config
	Env    *envtest.Environment
	Logger logr.Logger
	Ctx    context.Context
}

func (ts *EnvTestSuite) SetupSuite() {
	ts.Logger = zapr.NewLogger(zaptest.NewLogger(ts.T()))
	log.SetLogger(ts.Logger)

	ts.Ctx = context.Background()

	testEnv := &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		CRDDirectoryPaths:     []string{filepath.Join("..", "testbin", "bin", "apiextensions.k8s.io", "v1")},
		BinaryAssetsDirectory: filepath.Join("..", "testbin", "bin"),
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

// NewNS instantiates a new Namespace object with the given name.
func (ts *EnvTestSuite) NewNS(nsName string) *corev1.Namespace {
	require.Emptyf(ts.T(), validation.IsDNS1123Label(nsName), "'%s' does not appear to be a valid name for a namespace", nsName)

	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
		},
	}
}

// NewNS creates a new Namespace object using EnvTestSuite.Client.
func (ts *EnvTestSuite) CreateNS(nsName string) error {
	ns := ts.NewNS(nsName)
	err := ts.Client.Create(ts.Ctx, ns)
	return err
}

// SanitizeNameForNS first converts the given name to lowercase using strings.ToLower
// and then remove all characters but `a-z` (only lower case), `0-9` and the `-` (dash).
func (ts *EnvTestSuite) SanitizeNameForNS(name string) string {
	return InvalidNSNameCharacters.ReplaceAllString(strings.ToLower(name), "")
}

// BeforeTest is invoked just before every test starts
func (ts *EnvTestSuite) BeforeTest(suiteName, testName string) {
	ts.NS = ts.SanitizeNameForNS(fmt.Sprintf("%s-%s", suiteName, testName))

	err := ts.CreateNS(ts.NS)
	require.NoError(ts.T(), err)
	ts.T().Logf("Created NS '%s'", ts.NS)
}
