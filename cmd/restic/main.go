package restic

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"github.com/urfave/cli/v2"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/k8up-io/k8up/v2/cmd"
	"github.com/k8up-io/k8up/v2/restic/cfg"
	resticCli "github.com/k8up-io/k8up/v2/restic/cli"
	"github.com/k8up-io/k8up/v2/restic/kubernetes"
	"github.com/k8up-io/k8up/v2/restic/stats"
)

const (
	backupDirEnvKey             = "BACKUP_DIR"
	restoreDirEnvKey            = "RESTORE_DIR"
	caCertFileEnvKey            = "CA_CERT_FILE"
	clientCertFileEnvKey        = "CLIENT_CERT_FILE"
	clientKeyFileEnvKey         = "CLIENT_KEY_FILE"
	restoreCaCertFileEnvKey     = "RESTORE_CA_CERT_FILE"
	restoreClientCertFileEnvKey = "RESTORE_CLIENT_CERT_FILE"
	restoreClientKeyFileEnvKey  = "RESTORE_CLIENT_KEY_FILE"

	restoreTypeArg              = "restoreType"
	restoreS3EndpointArg        = "restoreS3Endpoint"
	restoreS3AccessKeyIDArg     = "restoreS3AccessKey"
	restoreS3SecretAccessKeyArg = "restoreS3SecretKey"
)

var (
	// Command is the definition of the command line interface of the restic module.
	Command = &cli.Command{
		Name:        "restic",
		Description: "Start k8up in restic mode",
		Action:      resticMain,
		Flags: []cli.Flag{
			&cli.BoolFlag{Destination: &cfg.Config.DoCheck, Name: "check", Usage: "Set, if the container should do a check"},
			&cli.BoolFlag{Destination: &cfg.Config.DoPrune, Name: "prune", Usage: "Set, if the container should do a prune"},
			&cli.BoolFlag{Destination: &cfg.Config.DoRestore, Name: "restore", Usage: "Set, if the container should attempt a restore"},
			&cli.BoolFlag{Destination: &cfg.Config.DoArchive, Name: "archive", Usage: "Set, if the container should do an archive"},

			&cli.StringSliceFlag{Name: "tag", Usage: "List of tags to consider for given operation"},

			&cli.StringFlag{Destination: &cfg.Config.BackupCommandAnnotation, Name: "backupCommandAnnotation", EnvVars: []string{"BACKUPCOMMAND_ANNOTATION"}, Usage: "Defines the command to invoke when doing a backup via STDOUT."},
			&cli.StringFlag{Destination: &cfg.Config.BackupFileExtensionAnnotation, Name: "fileExtensionAnnotation", EnvVars: []string{"FILEEXTENSION_ANNOTATION"}, Usage: "Defines the file extension to use for STDOUT backups."},
			&cli.StringFlag{Destination: &cfg.Config.BackupContainerAnnotation, Name: "backucontainerannotation", EnvVars: []string{"BACKUP_CONTAINERANNOTATION"}, Value: "k8up.io/backupcommand-container", Usage: "set the annotation name that specify the backup container inside the Pod"},
			&cli.BoolFlag{Destination: &cfg.Config.SkipPreBackup, Name: "skipPreBackup", EnvVars: []string{"SKIP_PREBACKUP"}, Usage: "If the job should skip the backup command and only backup volumes."},

			&cli.StringFlag{Destination: &cfg.Config.PromURL, Name: "promURL", EnvVars: []string{"PROM_URL"}, Usage: "Sets the URL of a prometheus push gateway to report metrics."},
			&cli.StringFlag{Destination: &cfg.Config.ClusterName, Name: "clusterName", EnvVars: []string{"CLUSTER_NAME"}, Usage: "Sets the Kubernetes cluster name for grouping metrics in push gateway"},
			&cli.StringFlag{Destination: &cfg.Config.WebhookURL, Name: "webhookURL", Aliases: []string{"statsURL"}, EnvVars: []string{"STATS_URL"}, Usage: "Sets the URL of a server which will retrieve a webhook after the action completes."},

			&cli.StringFlag{Destination: &cfg.Config.Hostname, Name: "hostname", EnvVars: []string{"HOSTNAME"}, Usage: "Sets the hostname to use in reports.", Hidden: true, Required: true},
			&cli.StringFlag{Destination: &cfg.Config.KubeConfig, Name: "kubeconfig", EnvVars: []string{"KUBECONFIG"}, Usage: "Overwrite the default kubernetes config to use.", Hidden: true, Value: clientcmd.RecommendedHomeFile},

			&cli.StringFlag{Destination: &cfg.Config.BackupDir, Name: "backupDir", EnvVars: []string{backupDirEnvKey}, Value: "/data", Usage: "Set from which directory the backup should be performed."},
			&cli.StringFlag{Destination: &cfg.Config.RestoreDir, Name: "restoreDir", EnvVars: []string{restoreDirEnvKey}, Value: "/data", Usage: "Set to which directory the restore should be performed."},

			&cli.StringFlag{Destination: &cfg.Config.RestoreFilter, Name: "restoreFilter", Usage: "Simple filter to define what should get restored. For example the PVC name"},
			&cli.StringFlag{Destination: &cfg.Config.RestoreSnap, Name: "restoreSnap", Usage: "Snapshot ID, if empty takes the latest snapshot"},
			&cli.StringFlag{Destination: &cfg.Config.RestoreType, Name: restoreTypeArg, Usage: "Type of this restore, 'folder' or 's3'"},
			&cli.StringFlag{Destination: &cfg.Config.RestoreS3AccessKey, Name: restoreS3AccessKeyIDArg, EnvVars: []string{"RESTORE_ACCESSKEYID"}, Usage: "S3 access key used to connect to the S3 endpoint when restoring"},
			&cli.StringFlag{Destination: &cfg.Config.RestoreS3SecretKey, Name: restoreS3SecretAccessKeyArg, EnvVars: []string{"RESTORE_SECRETACCESSKEY"}, Usage: "S3 secret key used to connect to the S3 endpoint when restoring"},
			&cli.StringFlag{Destination: &cfg.Config.RestoreS3Endpoint, Name: restoreS3EndpointArg, EnvVars: []string{"RESTORE_S3ENDPOINT"}, Usage: "S3 endpoint to connect to when restoring, e.g. 'https://minio.svc:9000/backup"},
			&cli.PathFlag{Destination: &cfg.Config.RestoreCACert, Name: "restoreCaCert", EnvVars: []string{restoreCaCertFileEnvKey}, Usage: "The certificate authority file path using for restore"},
			&cli.PathFlag{Destination: &cfg.Config.RestoreClientCert, Name: "restoreClientCert", EnvVars: []string{restoreClientCertFileEnvKey}, Usage: "The client certificate file path using for restore"},
			&cli.PathFlag{Destination: &cfg.Config.RestoreClientKey, Name: "restoreClientKey", EnvVars: []string{restoreClientKeyFileEnvKey}, Usage: "The client private key file path using for restore"},
			&cli.BoolFlag{Destination: &cfg.Config.VerifyRestore, Name: "verifyRestore", Usage: "If the restore should get verified, only for PVCs restore"},
			&cli.BoolFlag{Destination: &cfg.Config.RestoreTrimPath, Name: "trimRestorePath", EnvVars: []string{"TRIM_RESTOREPATH"}, Value: true, DefaultText: "enabled", Usage: "If set, strips the value of --restoreDir from the lefts side of the remote restore path value"},

			&cli.StringFlag{Destination: &cfg.Config.ResticBin, Name: "resticBin", EnvVars: []string{"RESTIC_BINARY"}, Usage: "The path to the restic binary.", Value: "/usr/local/bin/restic"},
			&cli.StringFlag{Destination: &cfg.Config.ResticRepository, Name: "resticRepository", EnvVars: []string{"RESTIC_REPOSITORY"}, Usage: "The restic repository to perform the action with", Required: true},
			&cli.StringFlag{Destination: &cfg.Config.ResticOptions, Name: "resticOptions", EnvVars: []string{"RESTIC_OPTIONS"}, Usage: "Additional options to pass to restic in the format 'key=value,key2=value2'"},

			&cli.StringSliceFlag{Name: "exclude", EnvVars: []string{"RESTIC_EXCLUDE"}, Usage: "In backup, passed to restic: exclude a `pattern` (can be specified multiple times)"},
			&cli.BoolFlag{Destination: &cfg.Config.ExcludeCaches, Name: "excludeCaches", EnvVars: []string{"RESTIC_EXCLUDE_CACHES"}, Usage: "In backup, passed to restic: excludes cache directories that are marked with a CACHEDIR.TAG file. See https://bford.info/cachedir/ for the Cache Directory Tagging Standard"},
			&cli.StringSliceFlag{Name: "excludeFile", EnvVars: []string{"RESTIC_EXCLUDE_FILE"}, Usage: "In backup, passed to restic: read exclude patterns from a `file` (can be specified multiple times). This file MUST be available in backup job container (e.g. in the directory being backed up)"},
			&cli.StringSliceFlag{Name: "excludeIfPresent", EnvVars: []string{"RESTIC_EXCLUDE_IF_PRESENT"}, Usage: "In backup, passed to restic: takes `filename[:header]`, exclude contents of directories containing filename (except filename itself) if header of that file is as provided (can be specified multiple times)"},
			&cli.StringFlag{Destination: &cfg.Config.ExcludeLargerThan, Name: "excludeLargerThan", EnvVars: []string{"RESTIC_EXCLUDE_LARGER_THAT"}, Usage: "In backup, passed to restic: max `size` of the files to be backed up (allowed suffixes: k/K, m/M, g/G, t/T)"},
			&cli.StringSliceFlag{Name: "filesFrom", EnvVars: []string{"RESTIC_FILES_FROM"}, Usage: "In backup, passed to restic: read the files to backup from `file` (can be combined with file args; can be specified multiple times)"},
			&cli.StringSliceFlag{Name: "filesFromRaw", EnvVars: []string{"RESTIC_FILES_FROM_RAW"}, Usage: "In backup, passed to restic: read the files to backup from `file` (can be combined with file args; can be specified multiple times)"},
			&cli.StringSliceFlag{Name: "filesFromVerbatim", EnvVars: []string{"RESTIC_FILES_FROM_VERBATIM"}, Usage: "In backup, passed to restic: read the files to backup from `file` (can be combined with file args; can be specified multiple times)"},
			&cli.StringSliceFlag{Name: "iExclude", EnvVars: []string{"RESTIC_IEXCLUDE"}, Usage: "In backup, passed to restic: same as --exclude `pattern` but ignores the casing of filenames"},
			&cli.StringSliceFlag{Name: "iExcludeFile", EnvVars: []string{"RESTIC_IEXCLUDE_FILE"}, Usage: "In backup, passed to restic: same as --exclude-file `pattern` but ignores the casing of filenames"},
			&cli.BoolFlag{Destination: &cfg.Config.OneFileSystem, Name: "oneFileSystem", EnvVars: []string{"RESTIC_ONE_FILESYSTEM"}, Usage: "In backup, passed to restic: exclude other file systems, don't cross filesystem boundaries and subvolumes"},

			&cli.IntFlag{Destination: &cfg.Config.PruneKeepLast, Name: "keepLatest", EnvVars: []string{"KEEP_LAST", "KEEP_LATEST"}, Usage: "While pruning, keep at the latest snapshot"},
			&cli.IntFlag{Destination: &cfg.Config.PruneKeepHourly, Name: "keepHourly", EnvVars: []string{"KEEP_HOURLY"}, Usage: "While pruning, keep hourly snapshots"},
			&cli.IntFlag{Destination: &cfg.Config.PruneKeepDaily, Name: "keepDaily", EnvVars: []string{"KEEP_DAILY"}, Usage: "While pruning, keep daily snapshots"},
			&cli.IntFlag{Destination: &cfg.Config.PruneKeepWeekly, Name: "keepWeekly", EnvVars: []string{"KEEP_WEEKLY"}, Usage: "While pruning, keep weekly snapshots"},
			&cli.IntFlag{Destination: &cfg.Config.PruneKeepMonthly, Name: "keepMonthly", EnvVars: []string{"KEEP_MONTHLY"}, Usage: "While pruning, keep monthly snapshots"},
			&cli.IntFlag{Destination: &cfg.Config.PruneKeepYearly, Name: "keepYearly", EnvVars: []string{"KEEP_YEARLY"}, Usage: "While pruning, keep yearly snapshots"},
			&cli.BoolFlag{Destination: &cfg.Config.PruneKeepTags, Name: "keepTags", EnvVars: []string{"KEEP_TAG", "KEEP_TAGS"}, Usage: "While pruning, keep tagged snapshots"},

			&cli.StringFlag{Destination: &cfg.Config.PruneKeepWithinHourly, Name: "keepWithinHourly", EnvVars: []string{"KEEP_WITHIN_HOURLY"}, Usage: "While pruning, keep hourly snapshots within the given duration, e.g. '2y5m7d3h'"},
			&cli.StringFlag{Destination: &cfg.Config.PruneKeepWithinDaily, Name: "keepWithinDaily", EnvVars: []string{"KEEP_WITHIN_DAILY"}, Usage: "While pruning, keep daily snapshots within the given duration, e.g. '2y5m7d3h'"},
			&cli.StringFlag{Destination: &cfg.Config.PruneKeepWithinWeekly, Name: "keepWithinWeekly", EnvVars: []string{"KEEP_WITHIN_WEEKLY"}, Usage: "While pruning, keep weekly snapshots within the given duration, e.g. '2y5m7d3h'"},
			&cli.StringFlag{Destination: &cfg.Config.PruneKeepWithinMonthly, Name: "keepWithinMonthly", EnvVars: []string{"KEEP_WITHIN_MONTHLY"}, Usage: "While pruning, keep monthly snapshots within the given duration, e.g. '2y5m7d3h'"},
			&cli.StringFlag{Destination: &cfg.Config.PruneKeepWithinYearly, Name: "keepWithinYearly", EnvVars: []string{"KEEP_WITHIN_YEARLY"}, Usage: "While pruning, keep yearly snapshots within the given duration, e.g. '2y5m7d3h'"},
			&cli.StringFlag{Destination: &cfg.Config.PruneKeepWithin, Name: "keepWithin", EnvVars: []string{"KEEP_WITHIN"}, Usage: "While pruning, keep tagged snapshots within the given duration, e.g. '2y5m7d3h'"},

			&cli.StringSliceFlag{Name: "targetPods", EnvVars: []string{"TARGET_PODS"}, Usage: "Filter list of pods by TARGET_PODS names"},
			&cli.DurationFlag{Destination: &cfg.Config.SleepDuration, Name: "sleepDuration", EnvVars: []string{"SLEEP_DURATION"}, Usage: "Sleep for specified amount until init starts"},

			&cli.PathFlag{Destination: &cfg.Config.VarDir, Name: "varDir", Value: "/k8up", Usage: "The var directory is stored k8up metadata files and temporary files"},
			&cli.PathFlag{Destination: &cfg.Config.CACert, Name: "caCert", EnvVars: []string{caCertFileEnvKey}, Usage: "The certificate authority file path"},
			&cli.PathFlag{Destination: &cfg.Config.ClientCert, Name: "clientCert", EnvVars: []string{clientCertFileEnvKey}, Usage: "The client certificate file path"},
			&cli.PathFlag{Destination: &cfg.Config.ClientKey, Name: "clientKey", EnvVars: []string{clientKeyFileEnvKey}, Usage: "The client private key file path"},
		},
	}
)

func resticMain(c *cli.Context) error {
	resticLog := cmd.AppLogger(c).WithName("restic")
	resticLog.Info("initializing")

	cfg.Config.Tags = c.StringSlice("tag")
	cfg.Config.TargetPods = c.StringSlice("targetPods")

	cfg.Config.Exclude = c.StringSlice("exclude")
	cfg.Config.ExcludeFile = c.StringSlice("excludeFile")
	cfg.Config.ExcludeIfPresent = c.StringSlice("excludeIfPresent")
	cfg.Config.FilesFrom = c.StringSlice("filesFrom")
	cfg.Config.FilesFromRaw = c.StringSlice("filesFromRaw")
	cfg.Config.FilesFromVerbatim = c.StringSlice("filesFromVerbatim")
	cfg.Config.IExclude = c.StringSlice("iExclude")
	cfg.Config.IExcludeFile = c.StringSlice("iExcludeFile")

	err := cfg.Config.Validate()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(c.Context)
	cancelOnTermination(cancel, resticLog)

	statHandler := stats.NewHandler(cfg.Config.PromURL, cfg.Config.ClusterName, cfg.Config.Hostname, cfg.Config.WebhookURL, resticLog)

	resticCLI := resticCli.New(ctx, resticLog.WithName("restic"), statHandler)

	return run(c.Context, resticCLI, resticLog)
}

func run(ctx context.Context, resticCLI *resticCli.Restic, mainLogger logr.Logger) error {
	if err := resticInitialization(resticCLI, mainLogger); err != nil {
		return err
	}

	if err := waitForEndOfConcurrentOperations(resticCLI); err != nil {
		return err
	}

	if cfg.Config.DoPrune || cfg.Config.DoCheck || cfg.Config.DoRestore || cfg.Config.DoArchive {
		return doNonBackupTasks(resticCLI)
	}

	return doBackup(ctx, resticCLI, mainLogger)
}

func resticInitialization(resticCLI *resticCli.Restic, mainLogger logr.Logger) error {
	if cfg.Config.SleepDuration > 0 {
		mainLogger.Info("sleeping until init", "duration", cfg.Config.SleepDuration)
		time.Sleep(cfg.Config.SleepDuration)
	}
	if err := resticCLI.Init(); err != nil {
		return fmt.Errorf("failed to initialise the restic repository: %w", err)
	}

	if err := resticCLI.Unlock(false); err != nil {
		mainLogger.Error(err, "failed to remove stale locks from the repository, continuing anyway")
	}

	// This builds up the cache without any other side effect. So it won't block
	// during any stdin backups or such.
	if err := resticCLI.Snapshots(nil); err != nil {
		return fmt.Errorf("failed to list snapshots: %w", err)
	}
	return nil
}

func waitForEndOfConcurrentOperations(resticCLI *resticCli.Restic) error {
	if cfg.Config.DoPrune || cfg.Config.DoCheck {
		if err := resticCLI.Wait(); err != nil {
			return fmt.Errorf("failed to list repository locks: %w", err)
		}
	}
	return nil
}

func doNonBackupTasks(resticCLI *resticCli.Restic) error {
	if err := doPrune(resticCLI); err != nil {
		return err
	}

	if err := doCheck(resticCLI); err != nil {
		return err
	}

	if err := doRestore(resticCLI); err != nil {
		return err
	}

	if err := doArchive(resticCLI); err != nil {
		return err
	}
	return nil
}

func doPrune(resticCLI *resticCli.Restic) error {
	if cfg.Config.DoPrune {
		if err := resticCLI.Prune(cfg.Config.Tags); err != nil {
			return fmt.Errorf("prune job failed: %w", err)
		}
	}
	return nil
}

func doCheck(resticCLI *resticCli.Restic) error {
	if cfg.Config.DoCheck {
		if err := resticCLI.Check(); err != nil {
			return fmt.Errorf("check job failed: %w", err)
		}
	}
	return nil
}

func doRestore(resticCLI *resticCli.Restic) error {
	if !cfg.Config.DoRestore {
		return nil
	}

	restoreOptions := resticCli.RestoreOptions{
		RestoreType:   resticCli.RestoreType(cfg.Config.RestoreType),
		RestoreDir:    cfg.Config.RestoreDir,
		RestoreFilter: cfg.Config.RestoreFilter,
		Verify:        cfg.Config.VerifyRestore,
		S3Destination: resticCli.S3Bucket{
			Endpoint:  cfg.Config.RestoreS3Endpoint,
			AccessKey: cfg.Config.RestoreS3AccessKey,
			SecretKey: cfg.Config.RestoreS3SecretKey,
			Cert:      fillRestoreS3Cert(),
		},
	}

	if err := resticCLI.Restore(cfg.Config.RestoreSnap, restoreOptions, cfg.Config.Tags); err != nil {
		return fmt.Errorf("restore job failed: %w", err)
	}

	return nil
}

func doArchive(resticCLI *resticCli.Restic) error {
	if !cfg.Config.DoArchive {
		return nil
	}

	restoreOptions := resticCli.RestoreOptions{
		RestoreType:   resticCli.RestoreType(cfg.Config.RestoreType),
		RestoreDir:    cfg.Config.RestoreDir,
		RestoreFilter: cfg.Config.RestoreFilter,
		Verify:        cfg.Config.VerifyRestore,
		S3Destination: resticCli.S3Bucket{
			Endpoint:  cfg.Config.RestoreS3Endpoint,
			AccessKey: cfg.Config.RestoreS3AccessKey,
			SecretKey: cfg.Config.RestoreS3SecretKey,
			Cert:      fillRestoreS3Cert(),
		},
	}

	if err := resticCLI.Archive(restoreOptions, cfg.Config.Tags); err != nil {
		return fmt.Errorf("archive job failed: %w", err)
	}

	return nil
}

func doBackup(ctx context.Context, resticCLI *resticCli.Restic, mainLogger logr.Logger) error {
	err := backupAnnotatedPods(ctx, resticCLI, mainLogger)
	if err != nil {
		return fmt.Errorf("backup of annotated pods failed: %w", err)
	}
	mainLogger.Info("backups of annotated jobs have finished successfully")

	err = resticCLI.Backup(cfg.Config.BackupDir, cfg.Config.Tags)
	if err != nil {
		return fmt.Errorf("backup job failed in dir '%s': %w", cfg.Config.BackupDir, err)
	}
	return nil
}

func backupAnnotatedPods(ctx context.Context, resticCLI *resticCli.Restic, mainLogger logr.Logger) error {
	_, serviceErr := os.Stat("/var/run/secrets/kubernetes.io")
	_, kubeconfigErr := os.Stat(cfg.Config.KubeConfig)

	if serviceErr != nil && kubeconfigErr != nil {
		mainLogger.Info("No kubernetes credentials configured: Can't check for annotated Pods.", "KUBECONFIG", cfg.Config.KubeConfig)
		return nil
	}

	k8cli, err := kubernetes.NewTypedClient(mainLogger)
	if err != nil {
		return fmt.Errorf("could not create kubernetes client: %w", err)
	}
	podLister := kubernetes.NewPodLister(ctx, k8cli, cfg.Config.BackupCommandAnnotation, cfg.Config.BackupFileExtensionAnnotation, cfg.Config.BackupContainerAnnotation, cfg.Config.Hostname, cfg.Config.TargetPods, cfg.Config.SkipPreBackup, mainLogger)
	podList, err := podLister.ListPods()
	if err != nil {
		mainLogger.Error(err, "could not list pods", "namespace", cfg.Config.Hostname)
		return fmt.Errorf("could not list pods: %w", err)
	}

	for _, pod := range podList {
		if err := backupAnnotatedPod(pod, mainLogger, cfg.Config.Hostname, resticCLI); err != nil {
			return err
		}
	}

	return nil
}

func backupAnnotatedPod(pod kubernetes.BackupPod, mainLogger logr.Logger, hostname string, resticCLI *resticCli.Restic) error {
	data, err := kubernetes.PodExec(pod, mainLogger)
	if err != nil {
		return fmt.Errorf("error occurred during data stream from k8s: %w", err)
	}
	filename := fmt.Sprintf("/%s-%s", hostname, pod.ContainerName)
	err = resticCLI.StdinBackup(data, filename, pod.FileExtension, cfg.Config.Tags)
	if err != nil {
		return fmt.Errorf("backup commands failed: %w", err)
	}
	return nil
}

func cancelOnTermination(cancel context.CancelFunc, mainLogger logr.Logger) {
	mainLogger.Info("setting up a signal handler")
	s := make(chan os.Signal, 1)
	signal.Notify(s, syscall.SIGTERM)
	go func() {
		mainLogger.Info("received signal", "signal", <-s)
		cancel()
	}()
}

func fillRestoreS3Cert() (cert resticCli.S3Cert) {
	if cfg.Config.RestoreCACert != "" {
		cert.CACert = cfg.Config.RestoreCACert
	}
	if cfg.Config.RestoreClientCert != "" && cfg.Config.RestoreClientKey != "" {
		cert.ClientCert = cfg.Config.RestoreClientCert
		cert.ClientKey = cfg.Config.RestoreClientKey
	}

	return cert
}
