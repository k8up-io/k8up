package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"git.vshn.net/vshn/wrestic/kubernetes"

	"git.vshn.net/vshn/wrestic/output"
	"git.vshn.net/vshn/wrestic/restic"
	"git.vshn.net/vshn/wrestic/s3"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	promURLEnv    = "PROM_URL"
	webhookURLEnv = "STATS_URL"
	commandEnv    = "BACKUPCOMMAND_ANNOTATION"
	fileextEnv    = "FILEEXTENSION_ANNOTATION"
)

type arrayOpts []string

func (a *arrayOpts) String() string {
	return "my string representation"
}

func (a *arrayOpts) Set(value string) error {
	*a = append(*a, value)
	return nil
}

var (
	check         = flag.Bool("check", false, "Set if the container should run a check")
	prune         = flag.Bool("prune", false, "Set if the container should run a prune")
	restore       = flag.Bool("restore", false, "Whether or not a restore should be done")
	restoreSnap   = flag.String("restoreSnap", "", "Snapshot ID, if empty takes the latest snapshot")
	verifyRestore = flag.Bool("verifyRestore", false, "If the restore should get verified, only for PVCs restore")
	restoreType   = flag.String("restoreType", "", "Type of this restore, folder or S3")
	restoreFilter = flag.String("restoreFilter", "", "Simple filter to define what should get restored. For example the PVC name")
	archive       = flag.Bool("archive", false, "")
	stdinOpts     arrayOpts
)

func main() {

	flag.Parse()

	finishC := make(chan error)
	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, syscall.SIGTERM, syscall.SIGINT)
	outputManager := output.New(os.Getenv(webhookURLEnv), os.Getenv(promURLEnv), os.Getenv(restic.Hostname))

	go run(finishC, outputManager)

	select {
	case err := <-finishC:
		outputManager.TriggerAll()
		if err != nil {
			fmt.Printf("Last reported error: %v", err)
			os.Exit(1)
		}

	case <-signalC:
		fmt.Println("Signal captured, removing locks and exiting...")
		outputManager.TriggerAll()
		os.Exit(1)
	}
}

func run(finishC chan error, outputManager *output.Output) {

	var dir string
	if os.Getenv(restic.BackupDirEnv) == "" {
		dir = "/data"
	} else {
		dir = os.Getenv(restic.BackupDirEnv)
	}
	resticCli := restic.New(dir)

	var commandRun bool
	var snapshots []restic.Snapshot

	s3BackupClient := s3.New(getS3Repo(), os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"))
	connErr := s3BackupClient.Connect()
	if connErr != nil {
		finishC <- fmt.Errorf("Connection to S3 endpoint not possible: %v", connErr)
		return
	}
	resticCli.InitRepository(s3BackupClient)

	if resticCli.Initrepo.GetError() != nil {
		finishC <- fmt.Errorf("error contacting the repository: %v", resticCli.Initrepo.GetError())
		return
	}

	var criticalError error

	resticCli.Unlock(false)
	if resticCli.UnlockStruct.GetError() != nil {
		finishC <- fmt.Errorf("could not unlock repository: %v", resticCli.UnlockStruct.GetError())
		return
	}
	defer resticCli.Unlock(false)

	if *prune {
		fmt.Println("Removing all locks to clear stale locks")
		resticCli.Prune()
		resticCli.ListSnapshots(false)
		outputManager.Register(resticCli.PruneStruct)
		outputManager.Register(resticCli.ListSnapshotsStruct)
		criticalError = resticCli.PruneStruct.GetError()
		commandRun = true
	}
	if *check {
		resticCli.Check()
		resticCli.ListSnapshots(false)
		outputManager.Register(resticCli.CheckStruct)
		commandRun = true
	}

	if *restore && criticalError == nil {
		snapshots = resticCli.ListSnapshots(false)
		criticalError = resticCli.ListSnapshotsStruct.GetError()
		resticCli.Restore(*restoreSnap, *restoreType, snapshots, os.Getenv(restic.RestoreDirEnv), *restoreFilter, *verifyRestore)
		criticalError = resticCli.RestoreStruct.GetError()
		commandRun = true
		outputManager.Register(resticCli.RestoreStruct)
	}
	if *archive && criticalError == nil {
		snapshots = resticCli.ListSnapshots(true)
		criticalError = resticCli.ListSnapshotsStruct.GetError()
		resticCli.Archive(snapshots, *restoreType, os.Getenv(restic.RestoreDirEnv), *restoreFilter, *verifyRestore)
		criticalError = resticCli.RestoreStruct.GetError()
		commandRun = true
		outputManager.Register(resticCli.RestoreStruct)
	}

	// Backup should run without any params but should also not run when
	// something else is desired. Also it will always try to list pods. If
	// wrestic is run on non kubernetes hosts it will just do file backups.
	if !commandRun {
		go startMetrics(outputManager)

		// the k8up operator passes the namespace as the hostname. So we can
		// use that information to determine in what namespace we currently are.
		namespace := os.Getenv(restic.Hostname)

		if namespace != "" {

			commandAnnotation := os.Getenv(commandEnv)
			if commandAnnotation == "" {
				commandAnnotation = "appuio.ch/backupcommand"
			}
			fileextAnnotation := os.Getenv(fileextEnv)
			if fileextAnnotation == "" {
				fileextAnnotation = "backup.appuio.ch/file-extension"
			}

			podLister := kubernetes.NewPodLister(commandAnnotation, fileextAnnotation, namespace)

			podList, err := podLister.ListPods()

			if err == nil {

				// List the snapshots before doing any stdIn backups. This will
				// ensure that the cache is already pupulated before any stdIn
				// connections over TCP/IP are opened.
				resticCli.ListSnapshots(false)

				for _, pod := range podList {
					resticCli.StdinBackup(pod.Command, pod.PodName, pod.ContainerName, pod.Namespace, pod.FileExtension)
					criticalError = resticCli.BackupStruct.GetError()
				}
			} else {
				newErr := fmt.Errorf("error listing pods: %v", err)
				fmt.Printf("%v\nSkipping backup commands\n", newErr)
				criticalError = newErr
			}

		}

		resticCli.Backup()
		criticalError = resticCli.BackupStruct.GetError()
		outputManager.Register(resticCli.BackupStruct)
		stopMetrics(outputManager)
	}

	finishC <- criticalError
}

func startMetrics(outputManager output.Trigger) {
	backupStartTimestamp := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: restic.Namespace,
		Subsystem: restic.Subsystem,
		Name:      "last_start_backup_timestamp",
		Help:      "Timestamp when the last backup was started",
	})

	backupDuration := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: restic.Namespace,
		Subsystem: restic.Subsystem,
		Name:      "running_backup_duration",
		Help:      "How long the current backup is taking",
	})

	backupStartTimestamp.SetToCurrentTime()
	outputManager.TriggerProm(backupStartTimestamp)

	tick := time.NewTicker(1 * time.Second)

	for {
		select {
		case <-tick.C:
			backupDuration.Inc()
			outputManager.TriggerProm(backupDuration)
			time.Sleep(1 * time.Second)
		}
	}

}

func stopMetrics(outputManager output.Trigger) {
	backupEndTimestamp := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: restic.Namespace,
		Subsystem: restic.Subsystem,
		Name:      "last_end_backup_timestamp",
		Help:      "Timestamp when the last backup was finished",
	})

	backupEndTimestamp.SetToCurrentTime()
	outputManager.TriggerProm(backupEndTimestamp)
}

func getS3Repo() string {
	resticString := os.Getenv("RESTIC_REPOSITORY")
	resticString = strings.ToLower(resticString)

	return strings.Replace(resticString, "s3:", "", -1)
}
