package logging

import (
	"testing"

	"github.com/go-logr/logr"
)

func FuzzBackupOutputParser(f *testing.F) {
	// Seed with realistic restic JSON output
	f.Add(`{"message_type":"status","percent_done":0.5,"total_files":100,"files_done":50}`)
	f.Add(`{"message_type":"summary","files_new":10,"files_changed":2,"total_duration":1.5,"snapshot_id":"abc12345"}`)
	f.Add(`{"message_type":"error","error":{"Op":"read","Path":"/data/file","Err":13},"during":"scan","item":"/data/file"}`)
	f.Add(`not json at all`)
	f.Add(`{}`)
	f.Add(`{"message_type":"unknown"}`)
	f.Add(``)
	f.Add(`{"message_type":"status","percent_done":-1}`)
	f.Add(`{"message_type":"summary","total_duration":999999999999}`)

	f.Fuzz(func(t *testing.T, input string) {
		summaryCalled := false
		summaryFunc := func(summary BackupSummary, errorCount int, folder string, startTimestamp, endTimestamp int64) {
			summaryCalled = true
			_ = summaryCalled
		}

		parser := &BackupOutputParser{
			log:            logr.Discard(),
			folder:         "/data",
			summaryFunc:    summaryFunc,
			percentageFunc: IgnorePercentage,
		}

		// Must not panic
		parser.out(input)
	})
}
