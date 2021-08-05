package operator

import (
	"fmt"
	"os"
	"strings"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/providers/env"
	"github.com/urfave/cli/v2"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	k8upv1alpha1 "github.com/vshn/k8up/api/v1alpha1"
	"github.com/vshn/k8up/cmd"
	"github.com/vshn/k8up/controllers"
	"github.com/vshn/k8up/operator/cfg"
	"github.com/vshn/k8up/operator/executor"
	// +kubebuilder:scaffold:imports
)

const leaderElectionID = "d2ab61da.syn.tools"

var (
	Command = &cli.Command{
		Name:        "operator",
		Description: "Start k8up in operator mode",
		Action:      operatorMain,
	}
)

func operatorMain(c *cli.Context) error {
	operatorLog := cmd.Logger(c, "operator")
	operatorLog.Info("initializing")

	executor.GetExecutor()
	loadEnvironmentVariables()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             k8upScheme(),
		MetricsBindAddress: cfg.Config.MetricsBindAddress,
		Port:               9443,
		LeaderElection:     cfg.Config.EnableLeaderElection,
		LeaderElectionID:   leaderElectionID,
	})
	if err != nil {
		operatorLog.Error(err, "unable to initialize operator mode", "step", "manager")
		return err
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
			operatorLog.Error(err, "unable to initialize operator mode", "step", "controller", "controller", name)
			return err
		}
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		operatorLog.Error(err, "unable to initialize operator mode", "step", "signal_handler")
		return err
	}

	return nil
}

func k8upScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(batchv1.AddToScheme(scheme))
	utilruntime.Must(k8upv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
	return scheme
}

func loadEnvironmentVariables() {
	operatorKoanf := koanf.New(".")
	prefix := "BACKUP_"
	// Load environment variables
	err := operatorKoanf.Load(env.Provider(prefix, ".", func(s string) string {
		s = strings.TrimPrefix(s, prefix)
		s = strings.ToLower(s)
		s = strings.Replace(s, "_", "-", -1)
		return s
	}), nil)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "could not load environment variables: %v\n", err)
	}

	if err := operatorKoanf.UnmarshalWithConf("", &cfg.Config, koanf.UnmarshalConf{Tag: "koanf", FlatPaths: true}); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "could not merge defaults with settings from environment variables: %v\n", err)
	}
	if err := cfg.Config.ValidateSyntax(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "settings invalid: %v\n", err)
		os.Exit(2)
	}
}
