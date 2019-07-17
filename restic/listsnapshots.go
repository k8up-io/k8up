package restic

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"git.vshn.net/vshn/wrestic/output"
)

const notInitialisedError = "not initialised yet"

// ListSnapshotsStruct holds the state of the listsnapshots command.
type ListSnapshotsStruct struct {
	genericCommand
	snaps snapList
}

func newListSnapshots() *ListSnapshotsStruct {
	return &ListSnapshotsStruct{}
}

// ListSnapshots executes the list snapshots command of restic.
func (l *ListSnapshotsStruct) ListSnapshots(last bool) []Snapshot {
	args := []string{"snapshots", "--json", "-q", "--no-lock"}
	if last {
		args = append(args, "--last")
	}

	l.genericCommand.exec(args, commandOptions{print: false})
	fmt.Printf("Listing snapshots\n")
	if len(l.stdErrOut) > 1 && strings.Contains(l.stdErrOut[1], "following location?") {
		l.errorMessage = errors.New(notInitialisedError)
		return nil
	}
	snaps := make([]Snapshot, 0)
	output := strings.Join(l.stdOut, "\n")
	err := json.Unmarshal([]byte(output), &snaps)
	if err != nil {
		fmt.Printf("Error listing snapshots:\n%v\n%v\n%v\n", err, output, strings.Join(l.stdErrOut, "\n"))
		l.errorMessage = err
		return nil
	}
	availableSnapshots := len(snaps)
	fmt.Printf("%v command:\n%v Snapshots\n", args[0], availableSnapshots)
	l.snaps = snapList(snaps)
	return snaps

}

// GetWebhookData returns a list of snapshots for the webhook.
func (l *ListSnapshotsStruct) GetWebhookData() []output.JsonMarshaller {
	list := []output.JsonMarshaller{
		l.snaps,
	}
	return list
}
