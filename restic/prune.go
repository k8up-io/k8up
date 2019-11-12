package restic

import (
	"fmt"
	"os"
	"strings"

	"git.vshn.net/vshn/wrestic/output"
)

// PruneStruct holds the state of the prune command.
type PruneStruct struct {
	genericCommand
	webhookSender  WebhookSender
	snapshotLister *ListSnapshotsStruct
}

func newPrune(snapshotLister *ListSnapshotsStruct, webhookSender WebhookSender, commandState *commandState) *PruneStruct {
	genericCommand := newGenericCommand(commandState)
	return &PruneStruct{
		webhookSender:  webhookSender,
		snapshotLister: snapshotLister,
		genericCommand: *genericCommand,
	}
}

func (p *PruneStruct) Prune() {

	// TODO: check for integers
	args := []string{"forget", "--prune"}

	if last := os.Getenv(keepLastEnv); last != "" {
		args = append(args, keepLastArg, last)
	}

	if hourly := os.Getenv(keepHourlyEnv); hourly != "" {
		args = append(args, keepHourlyArg, hourly)
	}

	if daily := os.Getenv(keepDailyEnv); daily != "" {
		args = append(args, keepDailyArg, daily)
	}

	if weekly := os.Getenv(keepWeeklyEnv); weekly != "" {
		args = append(args, keepWeeklyArg, weekly)
	}

	if monthly := os.Getenv(keepMonthlyEnv); monthly != "" {
		args = append(args, keepMonthlyArg, monthly)
	}

	if yearly := os.Getenv(keepYearlyEnv); yearly != "" {
		args = append(args, keepYearlyArg, yearly)
	}

	fmt.Println("Run forget and update the webhook")
	fmt.Println("forget params: ", strings.Join(args, " "))
	p.genericCommand.exec(args, commandOptions{print: true})
}

// GetWebhookData prepares and returns the data that gets sent via webhook at the end
func (p *PruneStruct) GetWebhookData() []output.JsonMarshaller {
	stats := make([]output.JsonMarshaller, 0)

	stats = append(stats, &WebhookStats{
		Name:       os.Getenv(Hostname),
		BucketName: getBucket(),
		Snapshots:  p.snapshotLister.ListSnapshots(false),
	})

	return stats
}
