package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
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
	RestoreType   RestoreType
	RestoreDir    string
	RestoreFilter string
	Verify        bool
	S3Destination S3Bucket
}

type S3Bucket struct {
	Endpoint  string
	AccessKey string
	SecretKey string
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
func (r *Restic) Restore(snapshotID string, options RestoreOptions, tags ArrayOpts) error {
	restorelogger := r.logger.WithName("restore")

	restorelogger.Info("restore initialised")

	if len(tags) > 0 {
		restorelogger.Info("loading snapshots", "tags", tags.String)
	} else {
		restorelogger.Info("loading all snapshots from repositoy")
	}

	err := r.Snapshots(tags)
	if err != nil {
		return err
	}

	latestSnap, err := r.getLatestSnapshot(snapshotID, restorelogger)
	if err != nil {
		return err
	}

	var stats *RestoreStats
	switch options.RestoreType {
	case FolderRestore:
		err = r.folderRestore(options.RestoreDir, latestSnap, options.RestoreFilter, options.Verify, restorelogger)
		stats = &RestoreStats{
			RestoreLocation: options.RestoreDir,
			RestoredFiles:   []string{"not supported for folder restores"},
			SnapshotID:      latestSnap.ID,
		}

	case S3Restore:
		stats = &RestoreStats{}
		err = r.s3Restore(restorelogger, latestSnap, stats)
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

func (r *Restic) getLatestSnapshot(snapshotID string, log logr.Logger) (dto.Snapshot, error) {
	snapshot := dto.Snapshot{}

	if len(r.snapshots) == 0 {
		err := fmt.Errorf("no snapshots available")
		log.Error(err, "no snapshots available")
		return snapshot, err
	}

	if snapshotID == "" {
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

func (r *Restic) folderRestore(restoreDir string, snapshot dto.Snapshot, restoreFilter string, verify bool, log logr.Logger) error {
	var linkedDir string

	singleFile, err := r.isRestoreSingleFile(log, snapshot)
	if err != nil {
		return err
	}

	if !singleFile && cfg.Config.RestoreTrimPath {
		restoreRoot, err := r.linkRestorePaths(snapshot, restoreDir)
		if err != nil {
			return err
		}

		defer func(path string) {
			err := os.RemoveAll(path)
			if err != nil {
				log.Error(err, "unable to clean up the files", "path", path)
			}
		}(restoreRoot)
		linkedDir = restoreRoot
	} else {
		linkedDir = restoreDir
	}

	log.Info("folder restore",
		"restoreDir", restoreDir,
		"trimPath", cfg.Config.RestoreTrimPath,
		"linkedDir", linkedDir,
		"restoreFilter", restoreFilter,
		"snapshotID", snapshot.ID)

	args := []string{snapshot.ID, "--target", linkedDir}
	if restoreFilter != "" {
		args = append(args, "--include", restoreFilter)
	}

	if verify {
		args = append(args, "--verify")
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

// linkRestorePaths will trim away the first two levels of the snapshotpath
// then create the first level as a folder in the temp dir and the second
// level as a symlink pointing to the mounted volume (usually /restore). It
// returns that temp path as the string used for the actual restore.This way the
// root of the backed up PVC will be the root of the restored PVC thus creating
// a carbon copy of the original and ready to be used again.
func (r *Restic) linkRestorePaths(snapshot dto.Snapshot, restoreDir string) (string, error) {
	// restic snapshots only every contain exactly one path
	snapshotPath := snapshot.Paths[0]
	splitted := strings.Split(snapshotPath, "/")
	joined := filepath.Join(splitted[:3]...)
	restoreRoot := filepath.Join(os.TempDir(), "restore")

	absolute := filepath.Join(restoreRoot, joined)
	makePath := filepath.Dir(absolute)

	err := os.MkdirAll(restoreDir, os.ModeDir+os.ModePerm)
	if err != nil {
		return "", err
	}
	err = os.MkdirAll(makePath, os.ModeDir+os.ModePerm)
	if err != nil {
		return "", err
	}

	err = os.Symlink(restoreDir, absolute)
	if err != nil {
		return "", err
	}

	return restoreRoot, nil
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

func (r *Restic) s3Restore(log logr.Logger, snapshot dto.Snapshot, stats *RestoreStats) error {
	log.Info("S3 chosen as restore destination")
	cleanupCtx, cleanup := context.WithCancel(r.ctx)
	defer cleanup()

	snapDate := snapshot.Time.Format(time.RFC3339)
	PVCName := r.parsePath(snapshot.Paths)
	fileName := fmt.Sprintf("backup-%v-%v-%v.tar.gz", snapshot.Hostname, PVCName, snapDate)

	stats.RestoreLocation = fmt.Sprintf("%s/%s", cfg.Config.RestoreS3Endpoint, fileName)
	stats.SnapshotID = snapshot.ID

	s3TransmissionErrorChannel, s3writer, err := r.s3Connect(r.ctx, fileName)
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

	err = r.s3Transmission(log, stats, s3writer)
	if err != nil {
		return err
	}

	cleanup()
	return <-s3TransmissionErrorChannel
}

func (r *Restic) s3Transmission(log logr.Logger, stats *RestoreStats, s3writer *io.PipeWriter) error {
	latestSnap, err := r.getLatestSnapshot(stats.SnapshotID, log)
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

func (r *Restic) s3Connect(ctx context.Context, fileName string) (chan error, *io.PipeWriter, error) {
	s3Client := s3.New(cfg.Config.RestoreS3Endpoint, cfg.Config.RestoreS3AccessKey, cfg.Config.RestoreS3SecretKey)
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
