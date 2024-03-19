package operator

import (
	"fmt"
	"strings"

	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	"github.com/k8up-io/k8up/v2/cmd"
	"github.com/k8up-io/k8up/v2/operator/archivecontroller"
	"github.com/k8up-io/k8up/v2/operator/backupcontroller"
	"github.com/k8up-io/k8up/v2/operator/cfg"
	"github.com/k8up-io/k8up/v2/operator/checkcontroller"
	"github.com/k8up-io/k8up/v2/operator/prunecontroller"
	"github.com/k8up-io/k8up/v2/operator/restorecontroller"
	"github.com/k8up-io/k8up/v2/operator/schedulecontroller"
	"github.com/urfave/cli/v2"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

const (
	leaderElectionID = "d2ab61da.syn.tools"
	argCommandRestic = "command-restic"
	argResticOptions = "restic-options"
)

var (
	// Command is the definition of the command line interface of the operator module.
	Command = &cli.Command{
		Name:        "operator",
		Description: "Start k8up in operator mode",
		Action:      operatorMain,
		Flags: []cli.Flag{
			&cli.StringFlag{Destination: &cfg.Config.BackupAnnotation, Name: "annotation", EnvVars: []string{"BACKUP_ANNOTATION"}, Value: "k8up.io/backup", Usage: "the annotation to be used for filtering"},
			&cli.StringFlag{Destination: &cfg.Config.BackupCommandAnnotation, Name: "backupcommandannotation", EnvVars: []string{"BACKUP_BACKUPCOMMANDANNOTATION"}, Value: "k8up.io/backupcommand", Usage: "set the annotation name that identify the backup commands on Pods"},
			&cli.StringFlag{Destination: &cfg.Config.FileExtensionAnnotation, Name: "fileextensionannotation", EnvVars: []string{"BACKUP_FILEEXTENSIONANNOTATION"}, Value: "k8up.io/file-extension", Usage: "set the annotation name where the file extension is stored for backup commands"},

			&cli.IntFlag{Destination: &cfg.Config.GlobalKeepJobs, Hidden: true, Name: "globalkeepjobs", EnvVars: []string{"BACKUP_GLOBALKEEPJOBS"}, Value: -1, DefaultText: "unlimited", Usage: "set the number of old jobs to keep when cleaning up, applies to all job types"},
			&cli.IntFlag{Destination: &cfg.Config.GlobalFailedJobsHistoryLimit, Name: "global-failed-jobs-history-limit", EnvVars: []string{"BACKUP_GLOBAL_FAILED_JOBS_HISTORY_LIMIT"}, Value: 3, Usage: "set the number of old, failed jobs to keep when cleaning up, applies to all job types"},
			&cli.IntFlag{Destination: &cfg.Config.GlobalSuccessfulJobsHistoryLimit, Name: "global-successful-jobs-history-limit", EnvVars: []string{"BACKUP_GLOBAL_SUCCESSFUL_JOBS_HISTORY_LIMIT"}, Value: 3, Usage: "set the number of old, successful jobs to keep when cleaning up, applies to all job types"},
			&cli.IntFlag{Destination: &cfg.Config.GlobalConcurrentArchiveJobsLimit, Name: "global-concurrent-archive-jobs-limit", EnvVars: []string{"BACKUP_GLOBAL_CONCURRENT_ARCHIVE_JOBS_LIMIT"}, DefaultText: "unlimited", Usage: "set the limit of concurrent archive jobs"},
			&cli.IntFlag{Destination: &cfg.Config.GlobalConcurrentBackupJobsLimit, Name: "global-concurrent-backup-jobs-limit", EnvVars: []string{"BACKUP_GLOBAL_CONCURRENT_BACKUP_JOBS_LIMIT"}, DefaultText: "unlimited", Usage: "set the limit of concurrent backup jobs"},
			&cli.IntFlag{Destination: &cfg.Config.GlobalConcurrentCheckJobsLimit, Name: "global-concurrent-check-jobs-limit", EnvVars: []string{"BACKUP_GLOBAL_CONCURRENT_CHECK_JOBS_LIMIT"}, DefaultText: "unlimited", Usage: "set the limit of concurrent check jobs"},
			&cli.IntFlag{Destination: &cfg.Config.GlobalConcurrentPruneJobsLimit, Name: "global-concurrent-prune-jobs-limit", EnvVars: []string{"BACKUP_GLOBAL_CONCURRENT_PRUNE_JOBS_LIMIT"}, DefaultText: "unlimited", Usage: "set the limit of concurrent prune jobs"},
			&cli.IntFlag{Destination: &cfg.Config.GlobalConcurrentRestoreJobsLimit, Name: "global-concurrent-restore-jobs-limit", EnvVars: []string{"BACKUP_GLOBAL_CONCURRENT_RESTORE_JOBS_LIMIT"}, DefaultText: "unlimited", Usage: "set the limit of concurrent restore jobs"},

			&cli.StringFlag{Destination: &cfg.Config.GlobalRestoreS3AccessKey, Name: "globalrestores3accesskeyid", EnvVars: []string{"BACKUP_GLOBALRESTORES3ACCESKEYID", "BACKUP_GLOBALRESTORES3ACCESSKEYID"}, Usage: "set the global restore S3 accessKeyID for restores"},
			&cli.StringFlag{Destination: &cfg.Config.GlobalRestoreS3Bucket, Name: "globalrestores3bucket", EnvVars: []string{"BACKUP_GLOBALRESTORES3BUCKET"}, Usage: "set the global restore S3 bucket for restores"},
			&cli.StringFlag{Destination: &cfg.Config.GlobalRestoreS3Endpoint, Name: "globalrestores3endpoint", EnvVars: []string{"BACKUP_GLOBALRESTORES3ENDPOINT"}, Usage: "set the global restore S3 endpoint for the restores (needs the scheme 'http' or 'https')"},
			&cli.StringFlag{Destination: &cfg.Config.GlobalRestoreS3SecretAccessKey, Name: "globalrestores3secretaccesskey", EnvVars: []string{"BACKUP_GLOBALRESTORES3SECRETACCESSKEY"}, Usage: "set the global restore S3 SecretAccessKey for restores"},

			&cli.StringFlag{Destination: &cfg.Config.GlobalRepoPassword, Name: "globalrepopassword", EnvVars: []string{"BACKUP_GLOBALREPOPASSWORD"}, Usage: "set the restic repository password to be used globally"},
			&cli.StringFlag{Destination: &cfg.Config.GlobalAccessKey, Name: "globalaccesskeyid", EnvVars: []string{"BACKUP_GLOBALACCESSKEYID"}, Usage: "set the S3 access key id to be used globally"},
			&cli.StringFlag{Destination: &cfg.Config.GlobalSecretAccessKey, Name: "globalsecretaccesskey", EnvVars: []string{"BACKUP_GLOBALSECRETACCESSKEY"}, Usage: "set the S3 secret access key to be used globally"},
			&cli.StringFlag{Destination: &cfg.Config.GlobalS3Bucket, Name: "globals3bucket", EnvVars: []string{"BACKUP_GLOBALS3BUCKET"}, Usage: "set the S3 bucket to be used globally"},
			&cli.StringFlag{Destination: &cfg.Config.GlobalS3Endpoint, Name: "globals3endpoint", EnvVars: []string{"BACKUP_GLOBALS3ENDPOINT"}, Usage: "set the S3 endpoint to be used globally"},

			&cli.StringFlag{Destination: &cfg.Config.GlobalCPUResourceRequest, Name: "global-cpu-request", EnvVars: []string{"BACKUP_GLOBAL_CPU_REQUEST"}, Usage: "set the CPU request for scheduled jobs"},
			&cli.StringFlag{Destination: &cfg.Config.GlobalCPUResourceLimit, Name: "global-cpu-limit", EnvVars: []string{"BACKUP_GLOBAL_CPU_LIMIT"}, Usage: "set the CPU limit for scheduled jobs"},
			&cli.StringFlag{Destination: &cfg.Config.GlobalMemoryResourceRequest, Name: "global-memory-request", EnvVars: []string{"BACKUP_GLOBAL_MEMORY_REQUEST"}, Usage: "set the memory request for scheduled jobs"},
			&cli.StringFlag{Destination: &cfg.Config.GlobalMemoryResourceLimit, Name: "global-memory-limit", EnvVars: []string{"BACKUP_GLOBAL_MEMORY_LIMIT"}, Usage: "set the memory limit for scheduled jobs"},

			&cli.StringFlag{Destination: &cfg.Config.BackupImage, Name: "image", EnvVars: []string{"BACKUP_IMAGE"}, Value: "ghcr.io/k8up-io/k8up:latest", Usage: "URL of the restic image"},
			&cli.StringSliceFlag{Name: argCommandRestic, EnvVars: []string{"BACKUP_COMMAND_RESTIC"}, Value: cli.NewStringSlice("/usr/local/bin/k8up", "restic"), Usage: "The command that is executed for restic backups."},
			&cli.StringSliceFlag{Name: argResticOptions, EnvVars: []string{"BACKUP_RESTIC_OPTIONS"}, Usage: "Pass custom restic options in the form 'key=value,key2=value2'. See https://restic.readthedocs.io/en/stable/manual_rest.html?highlight=--option#usage-help"},
			&cli.StringFlag{Destination: &cfg.Config.MountPath, Name: "datapath", Aliases: []string{"mountpath"}, EnvVars: []string{"BACKUP_DATAPATH"}, Value: "/data", Usage: "to which path the PVCs should get mounted in the backup container"},

			&cli.StringFlag{Destination: &cfg.Config.GlobalStatsURL, Name: "globalstatsurl", EnvVars: []string{"BACKUP_GLOBALSTATSURL"}, Usage: "set the URL to post metrics globally"},
			&cli.StringFlag{Destination: &cfg.Config.MetricsBindAddress, Name: "metrics-bindaddress", EnvVars: []string{"BACKUP_METRICS_BINDADDRESS"}, Value: ":8080", Usage: "set the bind address for the prometheus endpoint"},
			&cli.StringFlag{Destination: &cfg.Config.PromURL, Name: "promurl", EnvVars: []string{"BACKUP_PROMURL"}, Value: "http://127.0.0.1/", Usage: "set the operator wide default prometheus push gateway"},

			&cli.StringFlag{Destination: &cfg.Config.RestartPolicy, Name: "restartpolicy", EnvVars: []string{"BACKUP_RESTARTPOLICY"}, Value: "OnFailure", Usage: "set the RestartPolicy for the backup jobs. According to https://kubernetes.io/docs/concepts/workloads/controllers/jobs-run-to-completion/, this should be 'OnFailure' for jobs that terminate"},
			&cli.StringFlag{Destination: &cfg.Config.PodFilter, Name: "podfilter", EnvVars: []string{"BACKUP_PODFILTER"}, Value: "backupPod=true", Usage: "the filter used to find the backup pods"},
			&cli.StringFlag{Destination: &cfg.Config.ServiceAccount, Name: "podexecaccountname", Aliases: []string{"serviceaccount"}, EnvVars: []string{"BACKUP_PODEXECACCOUNTNAME"}, Value: "pod-executor", Usage: "set the service account name that should be used for the pod command execution"},
			&cli.StringFlag{Destination: &cfg.Config.PodExecRoleName, Name: "podexecrolename", EnvVars: []string{"BACKUP_PODEXECROLENAME"}, Value: "pod-executor", Usage: "set the role name that should be used for pod command execution"},

			&cli.BoolFlag{Destination: &cfg.Config.EnableLeaderElection, Name: "enable-leader-election", EnvVars: []string{"BACKUP_ENABLE_LEADER_ELECTION"}, Value: true, DefaultText: "enabled", Usage: "enable leader election within the operator Pod"},
			&cli.BoolFlag{Destination: &cfg.Config.SkipWithoutAnnotation, Name: "skip-pvcs-without-annotation", EnvVars: []string{"BACKUP_SKIP_WITHOUT_ANNOTATION"}, Value: false, DefaultText: "disabled", Usage: "skip selecting PVCs that don't have the BACKUP_ANNOTATION"},
			&cli.StringFlag{Destination: &cfg.Config.BackupCheckSchedule, Name: "checkschedule", EnvVars: []string{"BACKUP_CHECKSCHEDULE"}, Value: "0 0 * * 0", Usage: "the default check schedule"},
			&cli.StringFlag{Destination: &cfg.Config.OperatorNamespace, Name: "operator-namespace", EnvVars: []string{"BACKUP_OPERATOR_NAMESPACE"}, Required: true, Usage: "set the namespace in which the K8up operator itself runs"},
		},
	}
)

func operatorMain(c *cli.Context) error {
	operatorLog := cmd.AppLogger(c).WithName("operator")
	operatorLog.Info("initializing")
	ctrl.SetLogger(operatorLog)

	cfg.Config.BackupCommandRestic = c.StringSlice(argCommandRestic)
	cfg.Config.ResticOptions = strings.Join(c.StringSlice(argResticOptions), ",")

	err := validateQuantityFlags(c)
	if err != nil {
		return err
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: k8upScheme(),
		// MetricsBindAddress: cfg.Config.MetricsBindAddress,
		// Port:               9443,
		LeaderElection:   cfg.Config.EnableLeaderElection,
		LeaderElectionID: leaderElectionID,
		Metrics: server.Options{
			BindAddress: cfg.Config.MetricsBindAddress,
		},
		WebhookServer: &webhook.DefaultServer{
			Options: webhook.Options{
				Port: 9443,
			},
		},
	})
	if err != nil {
		operatorLog.Error(err, "unable to initialize operator mode", "step", "manager")
		return fmt.Errorf("unable to initialize controller runtime: %w", err)
	}

	for name, setupFn := range map[string]func(mgr ctrl.Manager) error{
		"Schedule": schedulecontroller.SetupWithManager,
		"Backup":   backupcontroller.SetupWithManager,
		"Restore":  restorecontroller.SetupWithManager,
		"Archive":  archivecontroller.SetupWithManager,
		"Check":    checkcontroller.SetupWithManager,
		"Prune":    prunecontroller.SetupWithManager,
	} {
		if setupErr := setupFn(mgr); setupErr != nil {
			operatorLog.Error(setupErr, "unable to initialize operator mode", "step", "controller", "controller", name)
			return fmt.Errorf("unable to setup reconciler: %w", setupErr)
		}
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		operatorLog.Error(err, "unable to initialize operator mode", "step", "signal_handler")
		return fmt.Errorf("unable to setup signal handler: %w", err)
	}

	return nil
}

func k8upScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(k8upv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
	return scheme
}

func validateQuantityFlags(ctx *cli.Context) error {
	quantityFlags := []string{
		"global-cpu-request",
		"global-cpu-limit",
		"global-memory-request",
		"global-memory-limit",
	}

	for _, f := range quantityFlags {
		if !ctx.IsSet(f) {
			continue
		}

		v := ctx.String(f)
		_, err := resource.ParseQuantity(v)
		if err != nil {
			return fmt.Errorf("the value '%s' of flag '%s' is not a valid Kubernetes quantity: %w", v, f, err)
		}
	}

	return nil
}
