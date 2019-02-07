package restic

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"git.vshn.net/vshn/wrestic/output"
	"git.vshn.net/vshn/wrestic/s3"
)

// RestoreStruct holds the state of the restore command.
type RestoreStruct struct {
	genericCommand
	restoreType   string
	restoreDir    string
	restoreFilter string
	verifyRestore bool
	stats         *restoreStats
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

func newRestore() *RestoreStruct {
	return &RestoreStruct{}
}

func (r *RestoreStruct) setState(restoreType, restoreDir, restoreFilter string, verifyRestore bool) {
	r.restoreType = restoreType
	r.restoreDir = restoreDir
	r.restoreFilter = restoreFilter
	r.verifyRestore = verifyRestore
}

// Archive uploads the last version of each snapshot to S3.
func (r *RestoreStruct) Archive(snaps []Snapshot, restoreType, restoreDir, restoreFilter string, verifyRestore bool) {

	r.setState(restoreType, restoreDir, restoreFilter, verifyRestore)

	fmt.Println("Archiving latest snapshots for every host")

	for _, v := range snaps {
		PVCname := r.parsePath(v.Paths)
		fmt.Printf("Archive running for %v\n", fmt.Sprintf("%v-%v", v.Hostname, PVCname))
		if err := r.restoreCommand(v.ID, r.restoreType, snaps); err != nil {
			r.errorMessage = err
			return
		}
	}

}

type restoreStats struct {
	RestoreLocation string   `json:"restore_location,omitempty"`
	SnapshotID      string   `json:"snapshot_ID,omitempty"`
	RestoredFiles   []string `json:"restored_files,omitempty"`
}

type tarStream struct {
	path       string
	tarHeader  *tar.Header
	readerChan chan io.ReadCloser
	runFunc    func()
}

// ToJson implements output.JsonMarshaller
func (r *restoreStats) ToJson() []byte {
	tmp, _ := json.Marshal(r)
	return tmp
}

// Restore takes a snapshotID and a method to create a restore job.
func (r *RestoreStruct) Restore(snapshotID, method string, snaps []Snapshot, restoreDir, restoreFilter string, verifyRestore bool) {

	r.setState(method, restoreDir, restoreFilter, verifyRestore)

	r.errorMessage = r.restoreCommand(snapshotID, method, snaps)
}

func (r *RestoreStruct) restoreCommand(snapshotID, method string, snaps []Snapshot) error {
	fmt.Println("Starting restore...")

	snapshot := Snapshot{}

	if snapshotID == "" {
		fmt.Println("No snapshot defined, using latest one.")
		snapshot = snaps[len(snaps)-1]
		fmt.Printf("Snapshot %v is being restored.\n", snapshot.Time)
	} else {
		for i := range snaps {
			if snaps[i].ID == snapshotID {
				snapshot = snaps[i]
			}
		}
		if snapshot.ID == "" {
			message := fmt.Sprintf("No Snapshot found with ID %v", snapshotID)
			fmt.Println(message)
			return fmt.Errorf(message)
		}
	}

	method = strings.ToLower(method)

	// TODO: implement some enum here: https://blog.learngoprogramming.com/golang-const-type-enums-iota-bc4befd096d3
	if method == "folder" {
		return r.folderRestore(snapshot)
	}

	if method == "s3" {
		return r.s3Restore(snapshot)
	}

	return fmt.Errorf("%v is not a valid restore type", r.restoreType)
}

func (r *RestoreStruct) folderRestore(snapshot Snapshot) error {

	args := []string{"restore", snapshot.ID, "--target", r.restoreDir}

	if r.restoreFilter != "" {
		args = append(args, "--include", r.restoreFilter)
	}

	if r.verifyRestore {
		args = append(args, "--verify")
	}

	r.genericCommand.exec(args, commandOptions{print: true})
	notIgnoredErrors := 0
	for _, errLine := range r.stdErrOut {
		if !strings.Contains(errLine, "Lchown") {
			notIgnoredErrors++
		}
	}
	if notIgnoredErrors > 0 {
		return fmt.Errorf("There were %v unignored errors, please have a look", notIgnoredErrors)
	}
	fmt.Println("Restore successful.")

	return nil
}

func (r *RestoreStruct) s3Restore(snapshot Snapshot) error {
	fmt.Println("S3 chosen as restore destination")

	endpoint := os.Getenv(RestoreS3EndpointEnv)
	snapDate := snapshot.Time.Format(time.RFC3339)
	PVCName := r.parsePath(snapshot.Paths)
	fileName := fmt.Sprintf("backup-%v-%v-%v.tar.gz", snapshot.Hostname, PVCName, snapDate)
	stats := &restoreStats{
		RestoreLocation: fmt.Sprintf("%v/%v", endpoint, fileName),
		SnapshotID:      snapshot.ID,
	}

	s3Client := s3.New(endpoint, os.Getenv(RestoreS3AccessKeyIDEnv), os.Getenv(RestoreS3SecretAccessKey))
	err := s3Client.Connect()
	if err != nil {
		return err
	}

	stream := r.compress(r.getTarReader(snapshot), stats)
	upload := s3.UploadObject{
		ObjectStream: stream,
		Name:         fileName,
	}
	err = s3Client.Upload(upload)
	if err != nil {
		return err
	}
	fmt.Println("Restore successful.")

	r.stats = stats

	return nil
}

func (r *RestoreStruct) compress(file tarStream, stats *restoreStats) io.Reader {
	readPipe, writePipe := io.Pipe()
	gzpWriter := gzip.NewWriter(writePipe)

	go func() {
		defer func() {
			gzpWriter.Close()
			writePipe.Close()
		}()
		stats.RestoredFiles = append(stats.RestoredFiles, file.path)

		go file.runFunc()
		reader := <-file.readerChan
		var writer io.Writer
		if file.tarHeader != nil {
			tw := tar.NewWriter(gzpWriter)
			tw.WriteHeader(file.tarHeader)
			writer = tw
			defer tw.Close()
		} else {
			writer = gzpWriter
		}
		_, err := io.Copy(writer, reader)
		if err != nil {
			fmt.Printf("\n%v\n", err)
			r.errorMessage = err
			return
		}
		file.readerChan <- reader
		fmt.Println("done!")
	}()

	return readPipe
}

// GetWebhookData returns a list of restore stats to send to the webhook.
func (r *RestoreStruct) GetWebhookData() []output.JsonMarshaller {
	return []output.JsonMarshaller{
		r.stats,
	}
}

func (r *RestoreStruct) parsePath(paths []string) string {
	return path.Base(paths[len(paths)-1])
}

func (r *RestoreStruct) getTarReader(snapshot Snapshot) tarStream {
	args := []string{"dump", snapshot.ID}

	snapshotRoot, header := r.getSnapshotRoot(snapshot)

	// Currently baas and wrestic have one path per snapshot
	tmpArgs := append(args, snapshotRoot)
	cmd := exec.Command(restic, tmpArgs...)
	cmd.Env = os.Environ()

	readerChan := make(chan io.ReadCloser, 0)

	return tarStream{
		path:       snapshotRoot,
		readerChan: readerChan,
		tarHeader:  header,
		runFunc: func() {

			stdOut, err := cmd.StdoutPipe()
			if err != nil {
				return
			}
			var stdErr bytes.Buffer
			cmd.Stderr = &stdErr

			err = cmd.Start()
			if err != nil {
				fmt.Println(err)
				return
			}

			readerChan <- stdOut
			<-readerChan

			err = cmd.Wait()
			if err != nil {
				fmt.Printf("Command failed with: '%v'\n", err)
				fmt.Printf("Output: %v\n", stdErr.String())
				return
			}
		},
	}
}

func (r *RestoreStruct) getSnapshotRoot(snapshot Snapshot) (string, *tar.Header) {
	cmd := newGenericCommand()
	args := []string{"ls", snapshot.ID, "--json"}
	cmd.exec(args, commandOptions{print: false})
	pathJSON := cmd.GetStdOut()[1]

	file := fileNode{}
	err := json.Unmarshal([]byte(pathJSON), &file)
	if err != nil {
		return snapshot.Paths[0], nil
	}

	var header *tar.Header
	if len(cmd.GetStdOut()) == 2 {
		header = &tar.Header{
			Name:       file.Path,
			Size:       file.Size,
			Mode:       int64(file.Mode),
			Uid:        file.UID,
			Gid:        file.GID,
			ModTime:    file.Mtime,
			AccessTime: file.Atime,
			ChangeTime: file.Ctime,
		}
	}

	return file.Path, header
}
