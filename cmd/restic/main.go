package restic

import (
	"context"
	"errors"
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
	check   bool
	prune   bool
	restore bool
	archive bool

	verifyRestore bool
	restoreSnap   string
	restoreType   string
	restoreFilter string

	tags resticCli.ArrayOpts
)

var (
	Command = &cli.Command{
		Name:        "restic",
		Description: "Start k8up in restic mode",
		Category:    "backup",
		Action:      resticMain,
		Flags: []cli.Flag{
			&cli.BoolFlag{Destination: &check, Name: "check", Usage: "Set if the container should run a check"},
			&cli.BoolFlag{Destination: &prune, Name: "prune", Usage: "Set if the container should run a prune"},
			&cli.BoolFlag{Destination: &restore, Name: "restore", Usage: "Whether or not a restore should be done"},
			&cli.BoolFlag{Destination: &verifyRestore, Name: "verifyRestore", Usage: "If the restore should get verified, only for PVCs restore"},
			&cli.BoolFlag{Destination: &archive, Name: "archive"},
			&cli.StringFlag{Destination: &restoreSnap, Name: "restoreSnap", Usage: "Snapshot ID, if empty takes the latest snapshot"},
			&cli.StringFlag{Destination: &restoreType, Name: "restoreType", Usage: "Type of this restore, folder or S3"},
			&cli.StringFlag{Destination: &restoreFilter, Name: "restoreFilter", Usage: "Simple filter to define what should get restored. For example the PVC name"},
			&cli.StringSliceFlag{Name: "tag", Usage: "List of tags to consider for given operation"},
		},
	}
)

func resticMain(c *cli.Context) error {
	resticLog := cmd.Logger(c, "wrestic")
	resticLog.Info("initializing")

	tags = c.StringSlice("tag")

	ctx, cancel := context.WithCancel(c.Context)
	cancelOnTermination(cancel, resticLog)

	statHandler := stats.NewHandler(os.Getenv(promURLEnv), os.Getenv(resticCli.Hostname), os.Getenv(webhookURLEnv), resticLog)

	resticCLI := resticCli.New(ctx, resticLog, statHandler)

	return run(c.Context, resticCLI, resticLog)
}

func run(ctx context.Context, resticCLI *resticCli.Restic, mainLogger logr.Logger) error {
	if err := resticInitialization(resticCLI, mainLogger); err != nil {
		return err
	}

	if err := waitForEndOfConcurringOperations(resticCLI, mainLogger); err != nil {
		return err
	}

	err := doNonBackupTasks(resticCLI, mainLogger)
	if err != nil {
		return err
	}

	if prune || check || restore || archive {
		return nil
	}

	return doBackup(ctx, resticCLI, mainLogger)
}

func resticInitialization(resticCLI *resticCli.Restic, mainLogger logr.Logger) error {
	if err := resticCLI.Init(); err != nil {
		mainLogger.Error(err, "failed to initialise the repository")
		return err
	}

	if err := resticCLI.Unlock(false); err != nil {
		mainLogger.Error(err, "failed to remove stale locks from the repository, continuing anyway")
	}

	// This builds up the cache without any other side effect. So it won't block
	// during any stdin backups or such.
	if err := resticCLI.Snapshots(nil); err != nil {
		mainLogger.Error(err, "failed to list snapshots")
		os.Exit(1)
	}
	return nil
}

func waitForEndOfConcurringOperations(resticCLI *resticCli.Restic, mainLogger logr.Logger) error {
	if prune || check {
		if err := resticCLI.Wait(); err != nil {
			mainLogger.Error(err, "failed to list repository locks")
			return err
		}
	}
	return nil
}

func doNonBackupTasks(resticCLI *resticCli.Restic, mainLogger logr.Logger) error {
	if err := doPrune(resticCLI, mainLogger); err != nil {
		return err
	}

	if err := doCheck(resticCLI, mainLogger); err != nil {
		return err
	}

	if err := doRestore(resticCLI, mainLogger); err != nil {
		return err
	}

	if err := doArchive(resticCLI, mainLogger); err != nil {
		return err
	}
	return nil
}

func doPrune(resticCLI *resticCli.Restic, mainLogger logr.Logger) error {
	if prune {
		if err := resticCLI.Prune(tags); err != nil {
			mainLogger.Error(err, "prune job failed")
			return err
		}
	}
	return nil
}

func doCheck(resticCLI *resticCli.Restic, mainLogger logr.Logger) error {
	if check {
		if err := resticCLI.Check(); err != nil {
			mainLogger.Error(err, "check job failed")
			return err
		}
	}
	return nil
}

func doRestore(resticCLI *resticCli.Restic, mainLogger logr.Logger) error {
	if restore {
		if err := resticCLI.Restore(restoreSnap, resticCli.RestoreOptions{
			RestoreType:   resticCli.RestoreType(restoreType),
			RestoreDir:    os.Getenv(resticCli.RestoreDirEnv),
			RestoreFilter: restoreFilter,
			Verify:        verifyRestore,
			S3Destination: resticCli.S3Bucket{
				Endpoint:  os.Getenv(resticCli.RestoreS3EndpointEnv),
				AccessKey: os.Getenv(resticCli.RestoreS3AccessKeyIDEnv),
				SecretKey: os.Getenv(resticCli.RestoreS3AccessKeyIDEnv),
			},
		}, tags); err != nil {
			mainLogger.Error(err, "restore job failed")
			return err
		}
	}
	return nil
}

func doArchive(resticCLI *resticCli.Restic, mainLogger logr.Logger) error {
	if archive {
		if err := resticCLI.Archive(restoreFilter, verifyRestore, tags); err != nil {
			mainLogger.Error(err, "archive job failed")
			return err
		}
	}
	return nil
}

func doBackup(ctx context.Context, resticCLI *resticCli.Restic, mainLogger logr.Logger) error {
	err := backupAnnotatedPods(ctx, resticCLI, mainLogger)
	if err != nil {
		mainLogger.Error(err, "backup job failed", "step", "backup of annotated pods")
		return err
	}
	mainLogger.Info("backups of annotated jobs have finished successfully")

	err = resticCLI.Backup(getBackupDir(), tags)
	if err != nil {
		mainLogger.Error(err, "backup job failed", "step", "backup of dir failed", "dir", getBackupDir())
		return err
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
		return nil
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
		mainLogger.Error(errors.New("error occurred during data stream from k8s"), "pod execution was interrupted")
		return err
	}
	filename := fmt.Sprintf("/%s-%s", hostname, pod.ContainerName)
	err = resticCLI.StdinBackup(data, filename, pod.FileExtension, tags)
	if err != nil {
		mainLogger.Error(err, "backup commands failed")
		return err
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
