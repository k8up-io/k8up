// +build integration

package controllers_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
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
	Scheme *runtime.Scheme
}

func (ts *EnvTestSuite) SetupSuite() {
	ts.Logger = zapr.NewLogger(zaptest.NewLogger(ts.T()))
	log.SetLogger(ts.Logger)

	ts.Ctx = context.Background()

	testbinDir := filepath.Join("..", "testbin", "bin")
	info, err := os.Stat(testbinDir)
	absTestbinDir, _ := filepath.Abs(testbinDir)
	ts.Require().NoErrorf(err, "'%s' does not seem to exist. Make sure you run `make integration-test` before you run this test in your IDE.", absTestbinDir)
	ts.Require().Truef(info.IsDir(), "'%s' does not seem to be a directory. Make sure you run `make integration-test` before you run this test in your IDE.", absTestbinDir)

	testEnv := &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "apiextensions.k8s.io", "v1", "base")},
		BinaryAssetsDirectory: testbinDir,
	}

	config, err := testEnv.Start()
	ts.Require().NoError(err)
	ts.Require().NotNil(config)

	registerCRDs(ts)

	k8sClient, err := client.New(config, client.Options{
		Scheme: ts.Scheme,
	})
	ts.Require().NoError(err)
	ts.Require().NotNil(k8sClient)

	executor.GetExecutor()

	ts.Env = testEnv
	ts.Config = config
	ts.Client = k8sClient
}

func registerCRDs(ts *EnvTestSuite) {
	ts.Scheme = runtime.NewScheme()
	ts.Require().NoError(corev1.AddToScheme(ts.Scheme))
	ts.Require().NoError(batchv1.AddToScheme(ts.Scheme))
	ts.Require().NoError(k8upv1a1.AddToScheme(ts.Scheme))

	// +kubebuilder:scaffold:scheme
}

func (ts *EnvTestSuite) TearDownSuite() {
	err := ts.Env.Stop()
	ts.Require().NoError(err)
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
			ts.Require().NoError(err)
			if done {
				return
			}
		case <-timedCtx.Done():
			if failureMsg == "" {
				failureMsg = timedCtx.Err().Error()
			}

			ts.Failf(failureMsg, "Failed after %s (%d attempts)", timeout, i)
			return
		}
	}
}

// NewNS instantiates a new Namespace object with the given name.
func (ts *EnvTestSuite) NewNS(nsName string) *corev1.Namespace {
	ts.Assert().Emptyf(validation.IsDNS1123Label(nsName), "'%s' does not appear to be a valid name for a namespace", nsName)

	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
		},
	}
}

// CreateNS creates a new Namespace object using EnvTestSuite.Client.
func (ts *EnvTestSuite) CreateNS(nsName string) error {
	ns := ts.NewNS(nsName)
	ts.T().Logf("creating namespace '%s'", nsName)
	return ts.Client.Create(ts.Ctx, ns)
}

// EnsureResources ensures that the given resources are existing in the suite. Each error will fail the test.
func (ts *EnvTestSuite) EnsureResources(resources ...client.Object) {
	for _, resource := range resources {
		ts.T().Logf("creating resource '%s/%s'", resource.GetNamespace(), resource.GetName())
		ts.Require().NoError(ts.Client.Create(ts.Ctx, resource))
	}
}

// UpdateResources ensures that the given resources are updated in the suite. Each error will fail the test.
func (ts *EnvTestSuite) UpdateResources(resources ...client.Object) {
	for _, resource := range resources {
		ts.T().Logf("updating resource '%s/%s'", resource.GetNamespace(), resource.GetName())
		ts.Require().NoError(ts.Client.Update(ts.Ctx, resource))
	}
}

// DeleteResources deletes the given resources are updated from the suite. Each error will fail the test.
func (ts *EnvTestSuite) DeleteResources(resources ...client.Object) {
	for _, resource := range resources {
		ts.T().Logf("deleting '%s/%s'", resource.GetNamespace(), resource.GetName())
		ts.Require().NoError(ts.Client.Delete(ts.Ctx, resource))
	}
}

// FetchResource fetches the given object name and stores the result in the given object.
// Test fails on errors.
func (ts *EnvTestSuite) FetchResource(name types.NamespacedName, object client.Object) {
	ts.Require().NoError(ts.Client.Get(ts.Ctx, name, object))
}

// FetchResource fetches resources and puts the items into the given list with the given list options.
// Test fails on errors.
func (ts *EnvTestSuite) FetchResources(objectList client.ObjectList, opts ...client.ListOption) {
	ts.Require().NoError(ts.Client.List(ts.Ctx, objectList, opts...))
}

// MapToRequest maps the given object into a reconcile Request.
func (ts *EnvTestSuite) MapToRequest(object metav1.Object) ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      object.GetName(),
			Namespace: object.GetNamespace(),
		},
	}
}

// BeforeTest is invoked just before every test starts
func (ts *EnvTestSuite) SetupTest() {
	ts.NS = rand.String(8)
	ts.Require().NoError(ts.CreateNS(ts.NS))
}

// SanitizeNameForNS first converts the given name to lowercase using strings.ToLower
// and then remove all characters but `a-z` (only lower case), `0-9` and the `-` (dash).
func (ts *EnvTestSuite) SanitizeNameForNS(name string) string {
	return InvalidNSNameCharacters.ReplaceAllString(strings.ToLower(name), "")
}
