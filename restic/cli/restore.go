package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/go-logr/logr"

	"github.com/k8up-io/k8up/v2/common"
	"github.com/k8up-io/k8up/v2/restic/cfg"
	"github.com/k8up-io/k8up/v2/restic/dto"
	"github.com/k8up-io/k8up/v2/restic/logging"
	"github.com/k8up-io/k8up/v2/restic/s3"
)

const (
	// FolderRestore indicates that a restore to a folder should be performed.
	FolderRestore RestoreType = cfg.RestoreTypeFolder
	// S3Restore indicates that a restore to a S3 endpoint should be performed.
	S3Restore RestoreType = cfg.RestoreTypeS3
)

// RestoreType defines the type for a restore.
type RestoreType string

// RestoreOptions holds options for a single restore, like type and destination.
type RestoreOptions struct {
	RestoreType       RestoreType
	RestoreDir        string
	RestoreFilter     string
	RestoreTimeFilter string
	Delete            bool
	Verify            bool
	S3Destination     S3Bucket
}

type S3Bucket struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Cert      S3Cert
}

type S3Cert struct {
	CACert     string
	ClientCert string
	ClientKey  string
}

type fileNode struct {
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Path       string    `json:"path"`
	UID        int       `json:"uid"`
	GID        int       `json:"gid"`
	Size       int64     `json:"size"`
	Mode       int       `json:"mode"`
	Mtime      time.Time `json:"mtime"`
	Atime      time.Time `json:"atime"`
	Ctime      time.Time `json:"ctime"`
	StructType string    `json:"struct_type"`
}

// Restore triggers a restore of a snapshot
func (r *Restic) Restore(snapshotID string, options RestoreOptions, tags ArrayOpts, paths ArrayOpts) error {
	restorelogger := r.logger.WithName("restore")

	restorelogger.Info("restore initialised")

	if len(tags) > 0 && len(paths) > 0 {
		restorelogger.Info("loading snapshots", "tags", tags.String, "paths", paths.String)
	} else if len(tags) > 0 {
		restorelogger.Info("loading snapshots", "tags", tags.String)
	} else if len(paths) > 0 {
		restorelogger.Info("loading snapshots", "paths", paths.String)
	} else {
		restorelogger.Info("loading all snapshots from repository")
	}

	err := r.Snapshots(tags, paths)
	if err != nil {
		return err
	}

	latestSnap, err := r.getLatestSnapshot(snapshotID, options.RestoreTimeFilter, restorelogger)
	if err != nil {
		return err
	}

	var stats *RestoreStats
	switch options.RestoreType {
	case FolderRestore:
		err = r.folderRestore(options.RestoreDir, latestSnap, options.RestoreFilter, options.Delete, options.Verify, restorelogger)
		stats = &RestoreStats{
			RestoreLocation: options.RestoreDir,
			RestoredFiles:   []string{"not supported for folder restores"},
			SnapshotID:      latestSnap.ID,
		}

	case S3Restore:
		stats = &RestoreStats{}
		err = r.s3Restore(restorelogger, options.S3Destination, latestSnap, stats)
	default:
		err = fmt.Errorf("no valid restore type")
	}

	if stats != nil && err == nil {
		err = r.statsHandler.SendWebhook(stats)
		if err != nil {
			return err
		}
	}

	return err
}

func (r *Restic) getLatestSnapshot(snapshotID string, timeFilter string, log logr.Logger) (dto.Snapshot, error) {
	snapshot := dto.Snapshot{}

	if len(r.snapshots) == 0 {
		err := fmt.Errorf("no snapshots available")
		log.Error(err, "no snapshots available")
		return snapshot, err
	}

	if snapshotID == "" {
		if timeFilter != "" {
			log.Info("no snapshot defined, but time filter specified; using filter match or latest")

			for i := len(r.snapshots) - 1; i >= 0; i-- {
				snapshot = r.snapshots[i]
				timeString := snapshot.Time.String()

				if strings.HasPrefix(timeString, timeFilter) {
					log.Info("Found snapshot matching timeFilter", "id", snapshot.ID, "created at", timeString)
					return snapshot, nil
				}
			}
		}

		log.Info("no snapshot defined, using latest one")
		snapshot = r.snapshots[len(r.snapshots)-1]
		log.Info("found snapshot", "date", snapshot.Time)
		return snapshot, nil
	}

	for i := range r.snapshots {
		// Doing substrings so we can also use short IDs here.
		if strings.HasPrefix(r.snapshots[i].ID, snapshotID) {
			return r.snapshots[i], nil
		}
	}

	err := fmt.Errorf("no Snapshot found with ID %v", snapshotID)
	log.Error(err, "the snapshot does not exist")
	return snapshot, err
}

func (r *Restic) folderRestore(restoreDir string, snapshot dto.Snapshot, restoreFilter string, delete bool, verify bool, log logr.Logger) error {
	var snap string

	singleFile, err := r.isRestoreSingleFile(log, snapshot)
	if err != nil {
		return err
	}

	if !singleFile && cfg.Config.RestoreTrimPath {
		restoreRoot := r.trimRestorePath(snapshot)

		snap = fmt.Sprintf("%s:%s", snapshot.ID, restoreRoot)
	} else {
		snap = snapshot.ID
	}

	log.Info("folder restore",
		"restoreDir", restoreDir,
		"trimPath", cfg.Config.RestoreTrimPath,
		"restoreFilter", restoreFilter,
		"snapshotID", snapshot.ID)

	args := []string{snap, "--target", restoreDir}
	if restoreFilter != "" {
		args = append(args, "--include", restoreFilter)
	}

	if verify {
		args = append(args, "--verify")
	}

	if delete {
		args = append(args, "--delete")
	}

	resticRestoreLogger := log.WithName("restic")
	opts := CommandOptions{
		Path:   r.resticPath,
		Args:   r.globalFlags.ApplyToCommand("restore", args...),
		StdOut: logging.NewInfoWriter(resticRestoreLogger),
		StdErr: logging.NewErrorWriter(resticRestoreLogger),
	}

	cmd := NewCommand(r.ctx, log, opts)
	cmd.Run()

	return nil
}

// trimRestorePath will trim away the first two levels of the snapshotpath.
// Previously k8up did a symlink to the correct path to trim away the full path.
// However as of restic >= 0.16.0 this won't work anymore, but it added a feature
// to trim the paths.
func (r *Restic) trimRestorePath(snapshot dto.Snapshot) string {
	// restic snapshots only every contain exactly one path
	return snapshot.Paths[0]
}

func (r *Restic) isRestoreSingleFile(log logr.Logger, snapshot dto.Snapshot) (bool, error) {
	buf := bytes.Buffer{}

	opts := CommandOptions{
		Path:   r.resticPath,
		Args:   r.globalFlags.ApplyToCommand("ls", "--json", snapshot.ID),
		StdOut: &buf,
	}

	cmd := NewCommand(r.ctx, log, opts)
	cmd.Run()
	capturedStdOut := buf.String()

	stdOutLines := strings.Split(capturedStdOut, "\n")

	if len(stdOutLines) == 0 {
		err := fmt.Errorf("no list exist for snapshot %v", snapshot.ID)
		log.Error(err, "the snapshot list is empty")
		return false, err
	}

	err := json.Unmarshal([]byte(stdOutLines[0]), &dto.Snapshot{})
	if err != nil {
		return false, err
	}

	count := 0
	for i := 1; i < len(stdOutLines); i++ {
		if len(stdOutLines[i]) == 0 {
			continue
		}

		node := &fileNode{}
		err := json.Unmarshal([]byte(stdOutLines[i]), node)
		if err != nil {
			return false, err
		}
		if node.Type == "file" {
			count++
		}
		if node.Type == "dir" {
			count = 0
			break
		}
		if count >= 2 {
			break
		}
	}

	if count == 1 {
		return true, nil
	}

	return false, nil
}

func (r *Restic) parsePath(paths []string) string {
	return path.Base(paths[len(paths)-1])
}

func (r *Restic) s3Restore(log logr.Logger, s3Options S3Bucket, snapshot dto.Snapshot, stats *RestoreStats) error {
	log.Info("S3 chosen as restore destination")
	cleanupCtx, cleanup := context.WithCancel(r.ctx)
	defer cleanup()

	snapDate := snapshot.Time.Format(time.RFC3339)
	PVCName := r.parsePath(snapshot.Paths)
	fileName := fmt.Sprintf("backup-%v-%v-%v.tar.gz", snapshot.Hostname, PVCName, snapDate)

	stats.RestoreLocation = fmt.Sprintf("%s/%s", s3Options.Endpoint, fileName)
	stats.SnapshotID = snapshot.ID

	s3TransmissionErrorChannel, s3writer, err := r.s3Connect(r.ctx, s3Options, fileName)
	if err != nil {
		return err
	}
	go func(ctx context.Context, log logr.Logger, s3writer *io.PipeWriter) {
		<-ctx.Done() // whenever cleanup() is called or the parent context is cancelled
		err := s3writer.Close()
		if err != nil {
			log.Error(err, "unable to close the s3writer")
		}
	}(cleanupCtx, log, s3writer)

	go func(log logr.Logger, stats *RestoreStats, s3writer *io.PipeWriter) {
		err = r.s3Transmission(log, stats, s3writer)
		if err != nil {
			s3TransmissionErrorChannel <- err
			return
		}

		cleanup()
	}(log, stats, s3writer)

	return <-s3TransmissionErrorChannel
}

func (r *Restic) s3Transmission(log logr.Logger, stats *RestoreStats, s3writer *io.PipeWriter) error {
	latestSnap, err := r.getLatestSnapshot(stats.SnapshotID, "", log)
	if err != nil {
		return err
	}

	snapRoot, tarHeader := r.getSnapshotRoot(latestSnap, log, stats)

	tgzWriter, err := r.tgzWriter(s3writer, tarHeader)
	if err != nil {
		return err
	}
	defer func(log logr.Logger, tgzWriter io.WriteCloser) {
		err := tgzWriter.Close()
		if err != nil {
			log.Error(err, "Unable to close the TarGzipWriter")
		}
	}(log, tgzWriter)

	log.Info("starting restore", "s3 filename", stats.RestoreLocation)
	r.doRestore(log, latestSnap, snapRoot, tgzWriter)
	log.Info("restore finished")

	return nil
}

func (r *Restic) tgzWriter(uploadWritePipe *io.PipeWriter, tarHeader *tar.Header) (io.WriteCloser, error) {
	if tarHeader == nil {
		gzipWriter := gzip.NewWriter(uploadWritePipe)
		return gzipWriter, nil
	}

	tgzWriter := common.NewTarGzipWriter(uploadWritePipe)
	err := tgzWriter.WriteHeader(tarHeader)
	if err != nil {
		_ = tgzWriter.Close()
		return nil, fmt.Errorf("unable to write the given tar header: %w", err)
	}

	return tgzWriter, nil
}

func (r *Restic) doRestore(log logr.Logger, latestSnap dto.Snapshot, snapRoot string, finalWriter io.WriteCloser) {
	opts := CommandOptions{
		Path:   r.resticPath,
		Args:   r.globalFlags.ApplyToCommand("dump", latestSnap.ID, snapRoot),
		StdOut: finalWriter,
		StdErr: logging.NewErrorWriter(log.WithName("restic")),
	}

	cmd := NewCommand(r.ctx, log, opts)
	cmd.Run()
}

func (r *Restic) s3Connect(ctx context.Context, s3Options S3Bucket, fileName string) (chan error, *io.PipeWriter, error) {
	s3Client := s3.New(
		s3Options.Endpoint,
		s3Options.AccessKey,
		s3Options.SecretKey,
		s3.Cert(s3Options.Cert),
	)
	err := s3Client.Connect(ctx)
	if err != nil {
		return nil, nil, err
	}

	uploadReadPipe, uploadWritePipe := io.Pipe()

	errorChannel := make(chan error)
	go func() {
		errorChannel <- s3Client.Upload(
			ctx,
			s3.UploadObject{
				Name:         fileName,
				ObjectStream: uploadReadPipe,
			})
	}()
	return errorChannel, uploadWritePipe, nil
}

// getSnapshotRoot will return the correct root to trigger the restore. If the
// snapshot was done as a stdin backup it will also return a tar header.
func (r *Restic) getSnapshotRoot(snapshot dto.Snapshot, log logr.Logger, stats *RestoreStats) (string, *tar.Header) {
	buf := bytes.Buffer{}

	opts := CommandOptions{
		Path:   r.resticPath,
		Args:   r.globalFlags.ApplyToCommand("ls", "--json", snapshot.ID),
		StdOut: &buf,
	}

	cmd := NewCommand(r.ctx, log, opts)
	cmd.Run()
	capturedStdOut := buf.String()

	amountOfFiles, lastNode := r.extractFileNodes(capturedStdOut, stats)
	if amountOfFiles == 1 {
		return lastNode.Path, r.getTarHeaderFromFileNode(lastNode)
	}
	return snapshot.Paths[len(snapshot.Paths)-1], nil
}

func (r *Restic) getTarHeaderFromFileNode(file *fileNode) *tar.Header {
	filePath := strings.Replace(file.Path, "/", "", 1)
	return &tar.Header{
		Name:       filePath,
		Size:       file.Size,
		Mode:       int64(file.Mode),
		Uid:        file.UID,
		Gid:        file.GID,
		ModTime:    file.Mtime,
		AccessTime: file.Atime,
		ChangeTime: file.Ctime,
	}
}

func (r *Restic) extractFileNodes(capturedStdOut string, stats *RestoreStats) (int, *fileNode) {
	// A backup from stdin will always contain exactly one file when executing ls.
	// This logic will also work if it's not a stdin backup. For the sake of the
	// dump this is the same case.
	stdOutLines := strings.Split(capturedStdOut, "\n")

	count := 0
	lastNode := &fileNode{}
	for _, fileJSON := range stdOutLines {
		node := &fileNode{}
		err := json.Unmarshal([]byte(fileJSON), node)
		if err != nil {
			continue
		}
		if node.Type == "file" {
			count++
			lastNode = node
			stats.RestoredFiles = append(stats.RestoredFiles, node.Path)
		}
	}
	return count, lastNode
}
