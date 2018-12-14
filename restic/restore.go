package restic

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"git.vshn.net/vshn/wrestic/s3"
)

// RestoreStruct holds the state of the restore command.
type RestoreStruct struct {
	genericCommand
	restoreType   string
	restoreDir    string
	restoreFilter string
	verifyRestore bool
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
	sortedSnaps := snapList(snaps)

	sort.Sort(sort.Reverse(sortedSnaps))

	snapMap := make(map[string]Snapshot)
	for _, snap := range sortedSnaps {
		if _, ok := snapMap[snap.Hostname]; !ok {
			snapMap[snap.Hostname] = snap
		}
	}

	for _, v := range snapMap {
		fmt.Printf("Archive running for %v\n", v.Hostname)
		if err := r.restoreCommand(v.ID, r.restoreType, sortedSnaps); err != nil {
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
		snapshot := snaps[len(snaps)-1]
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
	return nil
}

func (r *RestoreStruct) s3Restore(snapshot Snapshot) error {
	fmt.Println("S3 chosen as restore destination")
	r.listFilesInSnapshot(snapshot)
	fileList := r.stdOut
	readers, err := r.createFileReaders(snapshot, fileList)
	if err != nil {
		return err
	}

	endpoint := os.Getenv(RestoreS3EndpointEnv)
	snapDate := snapshot.Time.Format(time.RFC3339)
	fileName := fmt.Sprintf("backup-%v-%v.tar.gz", snapshot.Hostname, snapDate)
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
	return nil
}

func (r *RestoreStruct) listFilesInSnapshot(snapshot Snapshot) {
	args := []string{"ls", "-l", "--no-lock", snapshot.ID}
	fmt.Printf("Listing files in snapshot %v\n", snapshot.Time)
	r.genericCommand.exec(args, commandOptions{print: false})
}

type restoreFile struct {
	name    string
	mode    int64
	size    int64
	runFunc func()
	uid     int
	gid     int
	closer  chan io.ReadCloser
}

func (r *RestoreStruct) createFileReaders(snapshot Snapshot, fileList []string) ([]restoreFile, error) {
	// This case is so special we can't use genericCommand for this
	args := []string{"dump", snapshot.ID}

	fmt.Println("Adding files to the restore")

	filesToProcess := []restoreFile{}

	for i, file := range fileList {
		//Skip first entry
		if i == 0 {
			continue
		}
		scanner := bufio.NewScanner(strings.NewReader(file))
		scanner.Split(bufio.ScanWords)
		j := 0
		add := false
		// TODO: Next version of restic will most likely have
		// a json flag for the ls command.
		tmpFile := restoreFile{}
		for scanner.Scan() {
			m := scanner.Text()

			if j == 0 {
				if !strings.HasPrefix(m, "d") {
					add = true
					perms := m[1:len(m)]
					usrPerm := perms[0:3]
					groupPerm := perms[3:6]
					everyOne := perms[6:9]
					tmpPermString := "0" + r.parsePerm(usrPerm)
					tmpPermString = tmpPermString + r.parsePerm(groupPerm)
					tmpPermString = tmpPermString + r.parsePerm(everyOne)
					tmpFile.mode, _ = strconv.ParseInt(tmpPermString, 0, 64)
				} else {
					j++
					continue
				}
			}

			if j == 3 && add {
				size, err := strconv.ParseInt(m, 10, 64)
				tmpFile.size = size
				if err != nil {
					return nil, err
				}
			}

			if j == 1 && add {
				uid, err := strconv.Atoi(m)
				tmpFile.uid = uid
				if err != nil {
					return nil, err
				}
			}

			if j == 2 && add {
				gid, err := strconv.Atoi(m)
				tmpFile.gid = gid
				if err != nil {
					return nil, err
				}
			}

			if j == 6 && add {
				if r.restoreFilter != "" {
					if strings.Contains(m, r.restoreFilter) {
						tmpFile.name = m
					} else {
						add = false
					}
				} else {
					tmpFile.name = m
				}
			}
			j++
		}
		if add {
			filesToProcess = append(filesToProcess, tmpFile)
		}
	}

	for i := range filesToProcess {
		tmpArgs := append(args, filesToProcess[i].name)
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

func (r *RestoreStruct) tarGz(files []restoreFile, stats *restoreStats) io.Reader {
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
			stats.RestoredFiles = append(stats.RestoredFiles, file.name)
			fmt.Printf("Compressing %v...", file.name)
			header := &tar.Header{
				Name: file.name,
				Mode: file.mode,
				Size: file.size,
				Uid:  file.uid,
				Gid:  file.gid,
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

func (r *RestoreStruct) parsePerm(perm string) string {
	permInt := 0
	for _, char := range perm {
		if char == 'r' {
			permInt = permInt + 4
		}
		if char == 'w' {
			permInt = permInt + 2
		}
		if char == 'x' {
			permInt = permInt + 1
		}
	}
	return strconv.Itoa(permInt)
}
