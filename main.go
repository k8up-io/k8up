package main

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	batchv1 "k8s.io/api/batch/v1"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/providers/env"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/cfg"
	"github.com/vshn/k8up/controllers"
	"github.com/vshn/k8up/executor"
	// +kubebuilder:scaffold:imports
)

var (
	// These will be populated by Goreleaser
	version string
	commit  string
	date    string

	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
	// Global koanfInstance instance. Use . as the key path delimiter.
	koanfInstance = koanf.New(".")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(k8upv1alpha1.AddToScheme(scheme))
	utilruntime.Must(batchv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {

	loadEnvironmentVariables()
	level := zapcore.InfoLevel
	if strings.EqualFold(cfg.Config.LogLevel, "debug") {
		level = zapcore.DebugLevel
	}
	ctrl.SetLogger(zap.New(zap.UseDevMode(true), zap.Level(level)))

	setupLog.WithValues("version", version, "date", date, "commit", commit).Info("Starting K8up operator")
	executor.GetExecutor()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: cfg.Config.MetricsBindAddress,
		Port:               9443,
		LeaderElection:     cfg.Config.EnableLeaderElection,
		LeaderElectionID:   "d2ab61da.syn.tools",
		Namespace:          cfg.Config.WatchNamespace,
	})
	if err != nil {
		setupLog.Error(err, "unable to start K8up operator")
		os.Exit(1)
	}

	for name, reconciler := range map[string]controllers.ReconcilerSetup{
		"Schedule": &controllers.ScheduleReconciler{},
		"Backup":   &controllers.BackupReconciler{},
		"Restore":  &controllers.RestoreReconciler{},
		"Archive":  &controllers.ArchiveReconciler{},
		"Check":    &controllers.CheckReconciler{},
		"Prune":    &controllers.PruneReconciler{},
		"Job":      &controllers.JobReconciler{},
	} {
		if err := reconciler.SetupWithManager(mgr, ctrl.Log.WithName("controllers").WithName(name)); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", name)
			os.Exit(1)
		}
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running K8up")
		os.Exit(1)
	}
}

func loadEnvironmentVariables() {
	prefix := "BACKUP_"
	// Load environment variables
	err := koanfInstance.Load(env.Provider(prefix, ".", func(s string) string {
		s = strings.TrimPrefix(s, prefix)
		s = strings.Replace(strings.ToLower(s), "_", "-", -1)
		return s
	}), nil)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "could not load environment variables: %v\n", err)
	}

	if err := koanfInstance.UnmarshalWithConf("", &cfg.Config, koanf.UnmarshalConf{Tag: "koanf", FlatPaths: true}); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "could not merge defaults with settings from environment variables: %v\n", err)
	}
	if err := cfg.Config.ValidateSyntax(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "settings invalid: %v\n", err)
		os.Exit(2)
	}
}
