package restic

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"git.vshn.net/vshn/wrestic/output"
)

const notInitialisedError = "Not initialised yet"

// ListSnapshotsStruct holds the state of the listsnapshots command.
type ListSnapshotsStruct struct {
	genericCommand
	snaps snapList
}

func newListSnapshots() *ListSnapshotsStruct {
	return &ListSnapshotsStruct{}
}

// ListSnapshots executes the list snapshots command of restic.
func (l *ListSnapshotsStruct) ListSnapshots() []Snapshot {
	args := []string{"snapshots", "--json", "-q", "--no-lock"}
	var timeout int
	var converr error

	if timeout, converr = strconv.Atoi(os.Getenv(listTimeoutEnv)); converr != nil {
		timeout = 300
	}

	done := make(chan bool)
	go func() {
		l.genericCommand.exec(args, commandOptions{print: false})
		done <- true
	}()
	fmt.Printf("Listing snapshots, timeout: %v\n", timeout)
	select {
	case <-done:
		if len(l.StdErrOut) > 1 && strings.Contains(l.StdErrOut[1], "following location?") {
			l.Error = errors.New(notInitialisedError)
			return nil
		}
		snaps := make([]Snapshot, 0)
		output := strings.Join(l.StdOut, "\n")
		err := json.Unmarshal([]byte(output), &snaps)
		if err != nil {
			fmt.Printf("Error listing snapshots:\n%v\n%v\n%v\n", err, output, strings.Join(l.StdErrOut, "\n"))
			l.Error = err
			return nil
		}
		availableSnapshots := len(snaps)
		fmt.Printf("%v command:\n%v Snapshots\n", args[0], availableSnapshots)
		l.snaps = snapList(snaps)
		return snaps
	case <-time.After(time.Duration(timeout) * time.Second):
		l.Error = errors.New("connection timed out")
		return nil
	}
}

// GetWebhookData returns a list of snapshots for the webhook.
func (l *ListSnapshotsStruct) GetWebhookData() []output.JsonMarshaller {
	list := []output.JsonMarshaller{
		l.snaps,
	}
	return list
}
