//go:build integration

package envtest

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	// +kubebuilder:scaffold:imports

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/operator/cfg"
)

var InvalidNSNameCharacters = regexp.MustCompile("[^a-z0-9-]")

type Suite struct {
	suite.Suite

	NS     string
	Client client.Client
	Config *rest.Config
	Env    *envtest.Environment
	Logger logr.Logger
	Ctx    context.Context
	Scheme *runtime.Scheme
}

func (ts *Suite) SetupSuite() {
	ts.Logger = zapr.NewLogger(zaptest.NewLogger(ts.T()))
	log.SetLogger(ts.Logger)
	cfg.Config = defaultConfiguration()

	ts.Ctx = context.Background()

	envtestAssets, ok := os.LookupEnv("KUBEBUILDER_ASSETS")
	if !ok {
		ts.FailNow("The environment variable KUBEBUILDER_ASSETS is undefined. Configure your IDE to set this variable when running the integration test.")
	}

	info, err := os.Stat(envtestAssets)
	absEnvtestAssets, _ := filepath.Abs(envtestAssets)
	ts.Require().NoErrorf(err, "'%s' does not seem to exist. Check KUBEBUILDER_ASSETS and make sure you run `make integration-test` before you run this test in your IDE.", absEnvtestAssets)
	ts.Require().Truef(info.IsDir(), "'%s' does not seem to be a directory. Check KUBEBUILDER_ASSETS and make sure you run `make integration-test` before you run this test in your IDE.", absEnvtestAssets)

	crds := filepath.Join(Root, "config", "crd", "apiextensions.k8s.io", "v1")
	absCrds, _ := filepath.Abs(crds)
	info, err = os.Stat(crds)
	ts.Require().NoErrorf(err, "'%s' does not seem to exist. Make sure to set the working directory to the project root.", absCrds)
	ts.Require().Truef(info.IsDir(), "'%s' does not seem to be a directory. Make sure to set the working directory to the project root.", absCrds)

	ts.Logger.Info("envtest directories", "crd", absCrds, "binary assets", absEnvtestAssets)

	testEnv := &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		CRDDirectoryPaths:     []string{crds},
		BinaryAssetsDirectory: envtestAssets,
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

	ts.Env = testEnv
	ts.Config = config
	ts.Client = k8sClient
}

func registerCRDs(ts *Suite) {
	ts.Scheme = runtime.NewScheme()
	ts.Require().NoError(appsv1.AddToScheme(ts.Scheme))
	ts.Require().NoError(batchv1.AddToScheme(ts.Scheme))
	ts.Require().NoError(corev1.AddToScheme(ts.Scheme))
	ts.Require().NoError(k8upv1.AddToScheme(ts.Scheme))
	ts.Require().NoError(rbacv1.AddToScheme(ts.Scheme))

	// +kubebuilder:scaffold:scheme
}

func (ts *Suite) TearDownSuite() {
	err := ts.Env.Stop()
	ts.Require().NoErrorf(err, "error while stopping test environment")
	ts.Logger.Info("test environment stopped")
}

// NewNS instantiates a new Namespace object with the given name.
func (ts *Suite) NewNS(nsName string) *corev1.Namespace {
	ts.Assert().Emptyf(validation.IsDNS1123Label(nsName), "'%s' does not appear to be a valid name for a namespace", nsName)

	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
		},
	}
}

// EnsureNS creates a new Namespace object using Suite.Client.
func (ts *Suite) EnsureNS(nsName string) {
	ns := ts.NewNS(nsName)
	ts.T().Logf("creating namespace '%s'", nsName)
	ts.Require().NoError(ts.Client.Create(ts.Ctx, ns))
}

// EnsureResources ensures that the given resources are existing in the suite. Each error will fail the test.
func (ts *Suite) EnsureResources(resources ...client.Object) {
	for _, resource := range resources {
		ts.T().Logf("creating resource '%s/%s'", resource.GetNamespace(), resource.GetName())
		ts.Require().NoError(ts.Client.Create(ts.Ctx, resource))
	}
}

// UpdateResources ensures that the given resources are updated in the suite. Each error will fail the test.
func (ts *Suite) UpdateResources(resources ...client.Object) {
	for _, resource := range resources {
		ts.T().Logf("updating resource '%s/%s'", resource.GetNamespace(), resource.GetName())
		ts.Require().NoError(ts.Client.Update(ts.Ctx, resource))
	}
}

// UpdateStatus ensures that the Status property of the given resources are updated in the suite. Each error will fail the test.
func (ts *Suite) UpdateStatus(resources ...client.Object) {
	for _, resource := range resources {
		ts.T().Logf("updating status '%s/%s'", resource.GetNamespace(), resource.GetName())
		ts.Require().NoError(ts.Client.Status().Update(ts.Ctx, resource))
	}
}

// SetCondition sets the given condition and updates the status.
// Errors will fail the test.
func (ts *Suite) SetCondition(
	resource client.Object,
	conditions *[]metav1.Condition,
	cType k8upv1.ConditionType,
	status metav1.ConditionStatus,
	reason k8upv1.ConditionReason) {

	meta.SetStatusCondition(conditions, metav1.Condition{
		Type:    cType.String(),
		Status:  status,
		Reason:  reason.String(),
		Message: reason.String(),
	})
	ts.UpdateStatus(resource)
}

// DeleteResources deletes the given resources are updated from the suite. Each error will fail the test.
func (ts *Suite) DeleteResources(resources ...client.Object) {
	for _, resource := range resources {
		ts.T().Logf("deleting '%s/%s'", resource.GetNamespace(), resource.GetName())
		ts.Require().NoError(ts.Client.Delete(ts.Ctx, resource))
	}
}

// FetchResource fetches the given object name and stores the result in the given object.
// Test fails on errors.
func (ts *Suite) FetchResource(name types.NamespacedName, object client.Object) {
	ts.Require().NoError(ts.Client.Get(ts.Ctx, name, object))
}

// FetchResources fetches resources and puts the items into the given list with the given list options.
// Test fails on errors.
func (ts *Suite) FetchResources(objectList client.ObjectList, opts ...client.ListOption) {
	ts.Require().NoError(ts.Client.List(ts.Ctx, objectList, opts...))
}

// SetupTest is invoked just before every test starts
func (ts *Suite) SetupTest() {
	ts.NS = rand.String(8)
	ts.EnsureNS(ts.NS)
}

// SanitizeNameForNS first converts the given name to lowercase using strings.ToLower
// and then remove all characters but `a-z` (only lower case), `0-9` and the `-` (dash).
func (ts *Suite) SanitizeNameForNS(name string) string {
	return InvalidNSNameCharacters.ReplaceAllString(strings.ToLower(name), "")
}

// IsResourceExisting tries to fetch the given resource and returns true if it exists.
// It will consider still-existing object with a deletion timestamp as non-existing.
// Any other errors will fail the test.
func (ts *Suite) IsResourceExisting(ctx context.Context, obj client.Object) bool {
	err := ts.Client.Get(ctx, k8upv1.MapToNamespacedName(obj), obj)
	if apierrors.IsNotFound(err) {
		return false
	}
	ts.Assert().NoError(err)
	return obj.GetDeletionTimestamp() == nil
}

// defaultConfiguration retrieves the config with sane defaults
func defaultConfiguration() *cfg.Configuration {
	return &cfg.Configuration{
		MountPath:                        "/data",
		BackupAnnotation:                 "k8up.io/backup",
		BackupCommandAnnotation:          "k8up.io/backupcommand",
		FileExtensionAnnotation:          "k8up.io/file-extension",
		ServiceAccount:                   "pod-executor",
		BackupCheckSchedule:              "0 0 * * 0",
		GlobalFailedJobsHistoryLimit:     3,
		GlobalSuccessfulJobsHistoryLimit: 3,
		BackupImage:                      "ghcr.io/k8up-io/k8up:latest",
		BackupCommandRestic:              []string{"/usr/local/bin/k8up", "restic"},
		PodExecRoleName:                  "pod-executor",
		RestartPolicy:                    "OnFailure",
		MetricsBindAddress:               ":8080",
		PodFilter:                        "backupPod=true",
		EnableLeaderElection:             true,
	}
}
