package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/go-logr/glogr"
	"github.com/go-logr/logr"
	"github.com/golang/glog"
	"github.com/vshn/wrestic/kubernetes"
	"github.com/vshn/wrestic/restic"
	"github.com/vshn/wrestic/stats"
)

const (
	commandEnv    = "BACKUPCOMMAND_ANNOTATION"
	fileextEnv    = "FILEEXTENSION_ANNOTATION"
	promURLEnv    = "PROM_URL"
	webhookURLEnv = "STATS_URL"
)

var (
	Version       = "unreleased"
	BuildDate     = "now"
	check         = flag.Bool("check", false, "Set if the container should run a check")
	prune         = flag.Bool("prune", false, "Set if the container should run a prune")
	restore       = flag.Bool("restore", false, "Whether or not a restore should be done")
	restoreSnap   = flag.String("restoreSnap", "", "Snapshot ID, if empty takes the latest snapshot")
	verifyRestore = flag.Bool("verifyRestore", false, "If the restore should get verified, only for PVCs restore")
	restoreType   = flag.String("restoreType", "", "Type of this restore, folder or S3")
	restoreFilter = flag.String("restoreFilter", "", "Simple filter to define what should get restored. For example the PVC name")
	archive       = flag.Bool("archive", false, "")
	tags          restic.ArrayOpts
)

func printVersion(log logr.Logger) {
	log.Info(fmt.Sprintf("Wrestic Version: %s", Version))
	log.Info(fmt.Sprintf("Operator Build Date: %s", BuildDate))
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
}

func main() {
	if err := flag.Set("v", "3"); err != nil {
		fmt.Printf("error setting flag: %s", err)
		os.Exit(1)
	}
	if err := flag.Set("alsologtostderr", "true"); err != nil {
		fmt.Printf("error setting flag: %s", err)
		os.Exit(1)
	}
	flag.Var(&tags, "tag", "List of tags to consider for given operation")
	flag.Parse()

	mainLogger := glogr.New().WithName("wrestic")
	defer glog.Flush()

	printVersion(mainLogger)

	ctx, cancel := context.WithCancel(context.Background())
	cancelOnTermination(cancel, mainLogger)

	statHandler := stats.NewHandler(os.Getenv(promURLEnv), os.Getenv(restic.Hostname), os.Getenv(webhookURLEnv), mainLogger)

	resticCLI := restic.New(ctx, mainLogger, statHandler)

	err := run(resticCLI, mainLogger)
	if err != nil {
		os.Exit(1)
	}

}

func run(resticCLI *restic.Restic, mainLogger logr.Logger) error {
	if err := resticCLI.Init(); err != nil {
		mainLogger.Error(err, "failed to inialise the repository")
		return err
	}

	// This builds up the cache without any other side effect. So it won't block
	// during any stdin backups or such.
	if err := resticCLI.Snapshots(nil); err != nil {
		mainLogger.Error(err, "failed to list snapshots")
		os.Exit(1)
	}

	if *prune || *check {
		if err := resticCLI.Wait(); err != nil {
			mainLogger.Error(err, "failed to list repository locks")
			return err
		}
	}

	commandRun := false

	if *prune {
		commandRun = true
		if err := resticCLI.Prune(tags); err != nil {
			mainLogger.Error(err, "prune job failed")
			return err
		}
	}

	if *check {
		commandRun = true
		if err := resticCLI.Check(); err != nil {
			mainLogger.Error(err, "check job failed")
			return err
		}
	}

	if *restore {
		commandRun = true
		if err := resticCLI.Restore(*restoreSnap, restic.RestoreOptions{
			RestoreType:   restic.RestoreType(*restoreType),
			RestoreDir:    os.Getenv(restic.RestoreDirEnv),
			RestoreFilter: *restoreFilter,
			Verify:        *verifyRestore,
			S3Destination: restic.S3Bucket{
				Endpoint:  os.Getenv(restic.RestoreS3EndpointEnv),
				AccessKey: os.Getenv(restic.RestoreS3AccessKeyIDEnv),
				SecretKey: os.Getenv(restic.RestoreS3AccessKeyIDEnv),
			},
		}, tags); err != nil {
			mainLogger.Error(err, "restore job failed")
			return err
		}
	}

	if *archive {
		commandRun = true
		if err := resticCLI.Archive(*restoreFilter, *verifyRestore, tags); err != nil {
			mainLogger.Error(err, "archive job failed")
			return err
		}
	}

	if !commandRun {
		commandAnnotation := os.Getenv(commandEnv)
		if commandAnnotation == "" {
			commandAnnotation = "k8up.syn.tools/backupcommand"
		}
		fileextAnnotation := os.Getenv(fileextEnv)
		if fileextAnnotation == "" {
			fileextAnnotation = "k8up.syn.tools/file-extension"
		}

		_, serviceErr := os.Stat("/var/run/secrets/kubernetes.io")
		_, kubeconfigErr := os.Stat(kubernetes.Kubeconfig)

		if serviceErr == nil || kubeconfigErr == nil {

			podLister := kubernetes.NewPodLister(commandAnnotation, fileextAnnotation, os.Getenv(restic.Hostname), mainLogger)

			podList, err := podLister.ListPods()

			if err == nil {
				for _, pod := range podList {
					data, stdErr, err := kubernetes.PodExec(pod, mainLogger)
					if err != nil {
						mainLogger.Error(fmt.Errorf("error occured during data stream from k8s"), stdErr.String())
						return err
					}
					filename := fmt.Sprintf("/%s-%s", os.Getenv(restic.Hostname), pod.ContainerName)
					err = resticCLI.StdinBackup(data, filename, pod.FileExtension, tags)
					if err != nil {
						mainLogger.Error(err, "backup commands failed")
						return err
					}
				}
				mainLogger.Info("all pod commands have finished successfully")
			} else {
				mainLogger.Error(err, "could not list pods", "namespace", os.Getenv(restic.Hostname))
			}
		}

		err := resticCLI.Backup(getBackupDir(), tags)
		if err != nil {
			mainLogger.Error(err, "backup job failed")
			return err
		}

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

func getBackupDir() string {
	backupDir := os.Getenv(restic.BackupDirEnv)
	if backupDir == "" {
		return "/data"
	}
	return backupDir
}
