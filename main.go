package main

import (
	"flag"
	"os"
	"strings"

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

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	var enableLeaderElection bool
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	loadEnvironmentVariables()

	executor.GetExecutor()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: cfg.Config.MetricsBindAddress,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "d2ab61da.syn.tools",
	})
	if err != nil {
		setupLog.Error(err, "unable to start K8up operator")
		os.Exit(1)
	}

	if err = (&controllers.ScheduleReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Schedule"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Schedule")
		os.Exit(1)
	}
	if err = (&controllers.BackupReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Backup"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Backup")
		os.Exit(1)
	}
	if err = (&controllers.RestoreReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Restore"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Restore")
		os.Exit(1)
	}
	if err = (&controllers.ArchiveReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Archive"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Archive")
		os.Exit(1)
	}
	if err = (&controllers.CheckReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Check"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Check")
		os.Exit(1)
	}
	if err = (&controllers.PruneReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Prune"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Prune")
		os.Exit(1)
	}
	if err = (&controllers.JobReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("Job"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Job")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.WithValues("version", version, "date", date, "commit", commit).Info("Starting K8up operator")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running K8up")
		os.Exit(1)
	}
}

func loadEnvironmentVariables() {
	prefix := "BACKUP_"
	// Load environment variables
	err := koanfInstance.Load(env.Provider(prefix, ".", func(s string) string {
		s = strings.TrimLeft(s, prefix)
		s = strings.Replace(strings.ToLower(s), "_", "-", -1)
		return s
	}), nil)
	if err != nil {
		setupLog.Error(err, "could not load environment variables")
	}

	if err := koanfInstance.UnmarshalWithConf("", &cfg.Config, koanf.UnmarshalConf{Tag: "koanf", FlatPaths: true}); err != nil {
		setupLog.Error(err, "could not merge defaults with settings from environment variables")
	}
	if err := cfg.Config.ValidateSyntax(); err != nil {
		setupLog.Error(err, "settings invalid")
		os.Exit(2)
	}
}
