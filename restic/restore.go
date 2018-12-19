package restic

import (
	"archive/tar"
	"bufio"
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

func newRestore() *RestoreStruct {
	return &RestoreStruct{}
}

type fileJSON []struct {
	Time       time.Time  `json:"time"`
	Tree       string     `json:"tree"`
	Paths      []string   `json:"paths"`
	Hostname   string     `json:"hostname"`
	Username   string     `json:"username"`
	UID        int        `json:"uid"`
	Gid        int        `json:"gid"`
	ID         string     `json:"id"`
	ShortID    string     `json:"short_id"`
	Nodes      []fileNode `json:"nodes"`
	StructType string     `json:"struct_type"`
}

type fileNode struct {
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Path       string    `json:"path"`
	UID        int       `json:"uid"`
	Gid        int       `json:"gid"`
	Mode       int64     `json:"mode"`
	Mtime      time.Time `json:"mtime"`
	Atime      time.Time `json:"atime"`
	Ctime      time.Time `json:"ctime"`
	StructType string    `json:"struct_type"`
	Size       int64     `json:"size,omitempty"`
	closer     chan io.ReadCloser
	runFunc    func()
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
	r.listFilesInSnapshot(snapshot)
	fileList := fileJSON{}
	out := []byte(strings.Join(r.stdOut, "\n"))
	err := json.Unmarshal(out, &fileList)
	if err != nil {
		return err
	}
	readers, err := r.createFileReaders(snapshot, fileList)
	if err != nil {
		return err
	}

	endpoint := os.Getenv(RestoreS3EndpointEnv)
	snapDate := snapshot.Time.Format(time.RFC3339)
	PVCName := r.parsePath(snapshot.Paths)
	fileName := fmt.Sprintf("backup-%v-%v-%v.tar.gz", snapshot.Hostname, PVCName, snapDate)
	stats := &restoreStats{
		RestoreLocation: fmt.Sprintf("%v/%v", endpoint, fileName),
		SnapshotID:      snapshot.ID,
	}

	s3Client := s3.New(endpoint, os.Getenv(RestoreS3AccessKeyIDEnv), os.Getenv(RestoreS3SecretAccessKey))
	err = s3Client.Connect()
	if err != nil {
		return err
	}
	stream := r.tarGz(readers, stats)
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

func (r *RestoreStruct) listFilesInSnapshot(snapshot Snapshot) {
	args := []string{"ls", "-l", "--no-lock", "--json", snapshot.ID}
	fmt.Printf("Listing files in snapshot %v\n", snapshot.Time)
	r.genericCommand.exec(args, commandOptions{print: false})
}

func (r *RestoreStruct) createFileReaders(snapshot Snapshot, fileList fileJSON) ([]fileNode, error) {
	// This case is so special we can't use genericCommand for this
	args := []string{"dump", snapshot.ID}

	fmt.Println("Adding files to the restore")

	filesToProcess := []fileNode{}

	for _, files := range fileList {

		for _, node := range files.Nodes {
			add := false
			var tmpFile fileNode
			if node.Type != "dir" {
				tmpFile = node
				add = true
			}
			if add {
				filesToProcess = append(filesToProcess, tmpFile)
			}
		}

	}

	for i := range filesToProcess {
		tmpArgs := append(args, filesToProcess[i].Path)
		cmd := exec.Command(restic, tmpArgs...)
		cmd.Env = os.Environ()
		closerChan := make(chan io.ReadCloser, 0)
		filesToProcess[i].closer = closerChan
		filesToProcess[i].runFunc = func() {

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

			closerChan <- stdOut
			<-closerChan

			err = cmd.Wait()
			if err != nil {
				fmt.Printf("Command failed with: '%v'\n", err)
				fmt.Printf("Output: %v\n", stdErr.String())
				return
			}
		}
	}

	return filesToProcess, nil
}

func (r *RestoreStruct) tarGz(files []fileNode, stats *restoreStats) io.Reader {
	readPipe, writePipe := io.Pipe()
	gzpWriter := gzip.NewWriter(writePipe)
	trWriter := tar.NewWriter(gzpWriter)

	go func() {
		defer func() {
			trWriter.Close()
			gzpWriter.Close()
			writePipe.Close()
		}()
		for _, file := range files {
			stats.RestoredFiles = append(stats.RestoredFiles, file.Path)
			fmt.Printf("Compressing %v...", file.Path)
			header := &tar.Header{
				Name:       file.Path,
				Mode:       file.Mode,
				Size:       file.Size,
				Uid:        file.UID,
				Gid:        file.Gid,
				AccessTime: file.Atime,
				ChangeTime: file.Ctime,
				ModTime:    file.Mtime,
			}

			err := trWriter.WriteHeader(header)
			if err != nil {
				fmt.Printf("\n%v\n", err)
				r.errorMessage = err
				return
			}
			go file.runFunc()
			reader := <-file.closer
			buffer := bufio.NewReader(reader)
			_, err = io.Copy(trWriter, buffer)
			if err != nil {
				fmt.Printf("\n%v\n", err)
				r.errorMessage = err
				return
			}
			file.closer <- reader
			fmt.Println(" done!")
		}
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
