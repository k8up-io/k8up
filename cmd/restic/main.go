package restic

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/urfave/cli/v2"

	"github.com/vshn/k8up/cmd"
	resticCli "github.com/vshn/k8up/restic/cli"
	"github.com/vshn/k8up/restic/kubernetes"
	"github.com/vshn/k8up/restic/stats"
)

const (
	commandEnv    = "BACKUPCOMMAND_ANNOTATION"
	fileextEnv    = "FILEEXTENSION_ANNOTATION"
	promURLEnv    = "PROM_URL"
	webhookURLEnv = "STATS_URL"
)

var (
	// Config contains the values of the user-provided configuration of the restic module.
	Config = &Configuration{}
	// Command is the definition of the command line interface of the restic module.
	Command = &cli.Command{
		Name:        "restic",
		Description: "Start k8up in restic mode",
		Category:    "backup",
		Action:      resticMain,
		Flags: []cli.Flag{
			&cli.BoolFlag{Destination: &Config.doCheck, Name: "check", Usage: "Set if the container should run a check"},
			&cli.BoolFlag{Destination: &Config.doPrune, Name: "prune", Usage: "Set if the container should run a prune"},
			&cli.BoolFlag{Destination: &Config.doRestore, Name: "restore", Usage: "Whether or not a restore should be done"},
			&cli.BoolFlag{Destination: &Config.verifyRestore, Name: "verifyRestore", Usage: "If the restore should get verified, only for PVCs restore"},
			&cli.BoolFlag{Destination: &Config.doArchive, Name: "archive"},
			&cli.StringFlag{Destination: &Config.restoreSnap, Name: "restoreSnap", Usage: "Snapshot ID, if empty takes the latest snapshot"},
			&cli.StringFlag{Destination: &Config.restoreType, Name: "restoreType", Usage: "Type of this restore, folder or S3"},
			&cli.StringFlag{Destination: &Config.restoreFilter, Name: "restoreFilter", Usage: "Simple filter to define what should get restored. For example the PVC name"},
			&cli.StringSliceFlag{Name: "tag", Usage: "List of tags to consider for given operation"},
		},
	}
)

// Configuration contains all the configurable values for the restic module.
type Configuration struct {
	doCheck   bool
	doPrune   bool
	doRestore bool
	doArchive bool

	verifyRestore bool
	restoreSnap   string
	restoreType   string
	restoreFilter string

	tags resticCli.ArrayOpts
}

func resticMain(c *cli.Context) error {
	resticLog := cmd.AppLogger(c).WithName("wrestic")
	resticLog.Info("initializing")

	Config.tags = c.StringSlice("tag")

	ctx, cancel := context.WithCancel(c.Context)
	cancelOnTermination(cancel, resticLog)

	statHandler := stats.NewHandler(os.Getenv(promURLEnv), os.Getenv(resticCli.Hostname), os.Getenv(webhookURLEnv), resticLog)

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

	if Config.doPrune || Config.doCheck || Config.doRestore || Config.doArchive {
		return doNonBackupTasks(resticCLI)
	}

	return doBackup(ctx, resticCLI, mainLogger)
}

func resticInitialization(resticCLI *resticCli.Restic, mainLogger logr.Logger) error {
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
	if Config.doPrune || Config.doCheck {
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
	if Config.doPrune {
		if err := resticCLI.Prune(Config.tags); err != nil {
			return fmt.Errorf("prune job failed: %w", err)
		}
	}
	return nil
}

func doCheck(resticCLI *resticCli.Restic) error {
	if Config.doCheck {
		if err := resticCLI.Check(); err != nil {
			return fmt.Errorf("check job failed: %w", err)
		}
	}
	return nil
}

func doRestore(resticCLI *resticCli.Restic) error {
	if Config.doRestore {
		if err := resticCLI.Restore(Config.restoreSnap, resticCli.RestoreOptions{
			RestoreType:   resticCli.RestoreType(Config.restoreType),
			RestoreDir:    os.Getenv(resticCli.RestoreDirEnv),
			RestoreFilter: Config.restoreFilter,
			Verify:        Config.verifyRestore,
			S3Destination: resticCli.S3Bucket{
				Endpoint:  os.Getenv(resticCli.RestoreS3EndpointEnv),
				AccessKey: os.Getenv(resticCli.RestoreS3AccessKeyIDEnv),
				SecretKey: os.Getenv(resticCli.RestoreS3AccessKeyIDEnv),
			},
		}, Config.tags); err != nil {
			return fmt.Errorf("restore job failed: %w", err)
		}
	}
	return nil
}

func doArchive(resticCLI *resticCli.Restic) error {
	if Config.doArchive {
		if err := resticCLI.Archive(Config.restoreFilter, Config.verifyRestore, Config.tags); err != nil {
			return fmt.Errorf("archive job failed: %w", err)
		}
	}
	return nil
}

func doBackup(ctx context.Context, resticCLI *resticCli.Restic, mainLogger logr.Logger) error {
	err := backupAnnotatedPods(ctx, resticCLI, mainLogger)
	if err != nil {
		return fmt.Errorf("backup of annotated pods failed: %w", err)
	}
	mainLogger.Info("backups of annotated jobs have finished successfully")

	backupDir := getBackupDir()
	err = resticCLI.Backup(backupDir, Config.tags)
	if err != nil {
		return fmt.Errorf("backup job failed in dir '%s': %w", backupDir, err)
	}
	return nil
}

func backupAnnotatedPods(ctx context.Context, resticCLI *resticCli.Restic, mainLogger logr.Logger) error {
	commandAnnotation, fileextAnnotation, hostname := getVarsFromEnvOrDefault()

	_, serviceErr := os.Stat("/var/run/secrets/kubernetes.io")
	_, kubeconfigErr := os.Stat(kubernetes.Kubeconfig)

	if serviceErr != nil && kubeconfigErr != nil {
		mainLogger.Info("No kubernetes credentials configured: Can't check for annotated Pods.")
		return nil
	}

	podLister := kubernetes.NewPodLister(ctx, commandAnnotation, fileextAnnotation, hostname, mainLogger)
	podList, err := podLister.ListPods()
	if err != nil {
		mainLogger.Error(err, "could not list pods", "namespace", hostname)
		return fmt.Errorf("could not list pods: %w", err)
	}

	for _, pod := range podList {
		if err := backupAnnotatedPod(pod, mainLogger, hostname, resticCLI); err != nil {
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
	err = resticCLI.StdinBackup(data, filename, pod.FileExtension, Config.tags)
	if err != nil {
		return fmt.Errorf("backup commands failed: %w", err)
	}
	return nil
}

func getVarsFromEnvOrDefault() (string, string, string) {
	commandAnnotation, ok := os.LookupEnv(commandEnv)
	if !ok {
		commandAnnotation = "k8up.syn.tools/backupcommand"
	}
	fileextAnnotation, ok := os.LookupEnv(fileextEnv)
	if !ok {
		fileextAnnotation = "k8up.syn.tools/file-extension"
	}

	var hostname string
	hostname, ok = os.LookupEnv(resticCli.Hostname)
	if !ok {
		h, err := os.Hostname()
		if err != nil {
			hostname = "unknown-hostname"
		}
		hostname = h
	}
	return commandAnnotation, fileextAnnotation, hostname
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

func getBackupDir() string {
	backupDir := os.Getenv(resticCli.BackupDirEnv)
	if backupDir == "" {
		return "/data"
	}
	return backupDir
}
