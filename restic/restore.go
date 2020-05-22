package restic

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/vshn/wrestic/s3"
)

const (
	FolderRestore RestoreType = "folder"
	S3Restore     RestoreType = "s3"
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

	switch options.RestoreType {
	case FolderRestore:
		return r.folderRestore(options.RestoreDir, latestSnap, options.RestoreFilter, options.Verify, restorelogger)
	case S3Restore:
		return r.s3Restore(restorelogger, latestSnap)
	default:
		return fmt.Errorf("no valid restore type")
	}

}

func (r *Restic) getLatestSnapshot(snapshotID string, log logr.Logger) (Snapshot, error) {

	snapshot := Snapshot{}

	if snapshotID == "" {
		log.Info("no snapshot defined, using latest one")
		snapshot = r.snapshots[len(r.snapshots)-1]
		log.Info("found snapshot", "date", snapshot.Time)
	} else {
		for i := range r.snapshots {
			// Doing substrings so we can also use short IDs here.
			if r.snapshots[i].ID[0:len(snapshotID)] == snapshotID {
				snapshot = r.snapshots[i]
				break
			}
		}
		if snapshot.ID == "" {
			log.Error(fmt.Errorf("no Snapshot found with ID %v", snapshotID), "the snapshot does not exist")
			return snapshot, fmt.Errorf("no Snapshot found with ID %v", snapshotID)
		}
	}

	return snapshot, nil
}

func (r *Restic) folderRestore(restoreDir string, snapshot Snapshot, restoreFilter string, verify bool, log logr.Logger) error {
	var linkedDir string
	var trim bool
	trim, err := strconv.ParseBool(os.Getenv("TRIM_RESTOREPATH"))
	if err != nil {
		trim = true
	}
	if trim {
		linkedDir, err = r.linkRestorePaths(snapshot, restoreDir)
		if err != nil {
			return err
		}
		defer os.RemoveAll(linkedDir)
	} else {
		linkedDir = restoreDir
	}

	args := []string{"restore", snapshot.ID, "--target", linkedDir}

	if restoreFilter != "" {
		args = append(args, "--include", restoreFilter)
	}

	if verify {
		args = append(args, "--verify")
	}

	opts := CommandOptions{
		Path: r.resticPath,
		Args: args,
		StdOut: &outputWrapper{
			parser: &logOutParser{
				log: log.WithName("restic"),
			},
		},
		StdErr: &outputWrapper{
			parser: &logErrParser{
				log: log.WithName("restic"),
			},
		},
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
func (r *Restic) linkRestorePaths(snapshot Snapshot, restoreDir string) (string, error) {
	// wrestic snapshots only every contain exactly one path
	splitted := strings.Split(snapshot.Paths[0], "/")

	joined := filepath.Join(splitted[:3]...)

	restoreRoot := filepath.Join(os.TempDir(), "wresticRestore")

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

func (r *Restic) parsePath(paths []string) string {
	return path.Base(paths[len(paths)-1])
}

func (r *Restic) s3Restore(log logr.Logger, snapshot Snapshot) error {

	log.Info("S3 chosen as restore destination")

	snapDate := snapshot.Time.Format(time.RFC3339)
	PVCName := r.parsePath(snapshot.Paths)
	fileName := fmt.Sprintf("backup-%v-%v-%v.tar.gz", snapshot.Hostname, PVCName, snapDate)

	s3Client := s3.New(os.Getenv(RestoreS3EndpointEnv), os.Getenv(RestoreS3AccessKeyIDEnv), os.Getenv(RestoreS3SecretAccessKey))
	err := s3Client.Connect()
	if err != nil {
		return err
	}

	readPipe, writePipe := io.Pipe()

	latestSnap, err := r.getLatestSnapshot(snapshot.ID, log)
	if err != nil {
		return err
	}

	snapRoot, th := r.getSnapshotRoot(latestSnap, log)

	zw := gzip.NewWriter(writePipe)

	errorChanel := make(chan error)

	go func() {
		errorChanel <- s3Client.Upload(s3.UploadObject{
			Name:         fileName,
			ObjectStream: readPipe,
		})
	}()

	finalWriter := io.WriteCloser(zw)

	// If it's an stdin backup we restore we'll only have to write one header
	// as stdin backups only contain one vritual file.
	if th != nil {
		tw := tar.NewWriter(zw)
		err := tw.WriteHeader(th)
		if err != nil {
			return err
		}
		finalWriter = tw
		defer tw.Close()
	}

	opts := CommandOptions{
		Path: r.resticPath,
		Args: []string{
			"dump",
			latestSnap.ID,
			snapRoot,
		},
		StdOut: finalWriter,
		StdErr: &outputWrapper{
			parser: &logErrParser{
				log: log.WithName("restic"),
			},
		},
	}

	log.Info("starting restore", "s3 filename", fileName)

	cmd := NewCommand(r.ctx, log, opts)
	cmd.Run()

	// We need to close the writers in a specific order
	finalWriter.Close()
	zw.Close()

	// Send EOF so minio client knows it's finished
	// or else the chanel will block forever
	writePipe.Close()

	log.Info("restore finished")

	return <-errorChanel

}

// getSnapshotRoot will return the correct root to trigger the restore. If the
// snapshot was done as a stdin backup it will also return a tar header.
func (r *Restic) getSnapshotRoot(snapshot Snapshot, log logr.Logger) (string, *tar.Header) {

	buf := bytes.Buffer{}

	opts := CommandOptions{
		Path: r.resticPath,
		Args: []string{
			"ls",
			snapshot.ID,
			"--json",
		},
		StdOut: &buf,
	}

	cmd := NewCommand(r.ctx, log, opts)
	cmd.Run()

	// A backup from stdin will always contain exactly one file when executing ls.
	// This logic will also work if it's not a stdin backup. For the sake of the
	// dump this is the same case.
	split := strings.Split(buf.String(), "\n")

	amountOfFiles, lastNode := r.countFileNodes(split)

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

func (r *Restic) countFileNodes(rawJSON []string) (int, *fileNode) {
	count := 0
	lastNode := &fileNode{}
	for _, fileJSON := range rawJSON {
		node := &fileNode{}
		err := json.Unmarshal([]byte(fileJSON), node)
		if err != nil {
			continue
		}
		if node.Type == "file" {
			count++
			lastNode = node
		}
	}
	return count, lastNode
}
