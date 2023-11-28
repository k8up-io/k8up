package logging

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/go-logr/logr"
)

type BackupSummary struct {
	MessageType         string  `json:"message_type"`
	FilesNew            int     `json:"files_new"`
	FilesChanged        int     `json:"files_changed"`
	FilesUnmodified     int     `json:"files_unmodified"`
	DirsNew             int     `json:"dirs_new"`
	DirsChanged         int     `json:"dirs_changed"`
	DirsUnmodified      int     `json:"dirs_unmodified"`
	DataBlobs           int     `json:"data_blobs"`
	TreeBlobs           int     `json:"tree_blobs"`
	DataAdded           int64   `json:"data_added"`
	TotalFilesProcessed int     `json:"total_files_processed"`
	TotalBytesProcessed int     `json:"total_bytes_processed"`
	TotalDuration       float64 `json:"total_duration"`
	SnapshotID          string  `json:"snapshot_id"`
}

type BackupEnvelope struct {
	MessageType string `json:"message_type,omitempty"`
	BackupStatus
	BackupSummary
	BackupError
}

type BackupStatus struct {
	PercentDone      float64  `json:"percent_done"`
	TotalFiles       int      `json:"total_files"`
	FilesDone        int      `json:"files_done"`
	TotalBytes       int      `json:"total_bytes"`
	BytesDone        int      `json:"bytes_done"`
	CurrentFiles     []string `json:"current_files"`
	ErrorCount       int      `json:"error_count"`
	SecondsRemaining int      `json:"seconds_remaining"`
}

// SummaryFunc takes the summed up status of the backup and will process this further like
// logging, metrics and webhooks.
type SummaryFunc func(summary BackupSummary, errorCount int, folder string, startTimestamp, endTimestamp int64)

// PercentageFunc should format and print the given float.
type PercentageFunc func(logr.Logger, *BackupEnvelope)

type BackupOutputParser struct {
	log         logr.Logger
	errorCount  int
	summaryFunc SummaryFunc
	statusFunc  PercentageFunc
	folder      string
}

type BackupError struct {
	Error struct {
		Op   string `json:"Op"`
		Path string `json:"Path"`
		Err  int    `json:"Err"`
	} `json:"error"`
	During string `json:"during"`
	Item   string `json:"item"`
}

type outFunc func(string)

// New creates a writer which directly writes to the given logger function.
func New(out outFunc) io.Writer {
	return &writer{out}
}

// NewInfoWriter creates a writer with the name "stdout" which directly writes to the given logger using info level.
// It ensures that each line is handled separately. This avoids mangled lines when parsing
// JSON outputs.
func NewInfoWriter(l logr.Logger) io.Writer {
	return New((&LogInfoPrinter{l}).out)
}

// NewErrorWriter creates a writer with the name "stderr" which directly writes to the given logger using info level.
// It ensures that each line is handled seperately. This avoids mangled lines when parsing
// JSON outputs.
func NewErrorWriter(l logr.Logger) io.Writer {
	return New((&LogErrPrinter{l}).out)
}

type writer struct {
	out outFunc
}

func (w writer) Write(p []byte) (int, error) {

	scanner := bufio.NewScanner(bytes.NewReader(p))

	for scanner.Scan() {
		w.out(scanner.Text())
	}

	return len(p), nil
}

type LogInfoPrinter struct {
	log logr.Logger
}

func (l *LogInfoPrinter) out(s string) {
	l.log.WithName("stdout").Info(s)
}

type LogErrPrinter struct {
	Log logr.Logger
}

func (l *LogErrPrinter) out(s string) {
	l.Log.WithName("stderr").Info(s)
}

func NewBackupOutputParser(logger logr.Logger, folderName string, summaryFunc SummaryFunc) io.Writer {
	bop := &BackupOutputParser{
		log:         logger,
		folder:      folderName,
		summaryFunc: summaryFunc,
		statusFunc:  PrintStatus,
	}
	return New(bop.out)
}

func NewStdinBackupOutputParser(logger logr.Logger, folderName string, summaryFunc SummaryFunc) io.Writer {
	bop := &BackupOutputParser{
		log:         logger,
		folder:      folderName,
		summaryFunc: summaryFunc,
		statusFunc:  IgnoreStatus,
	}
	return New(bop.out)
}

func (b *BackupOutputParser) out(s string) {
	envelope := &BackupEnvelope{}

	err := json.Unmarshal([]byte(s), envelope)
	if err != nil {
		b.log.Info("restic output", "msg", s)
	}

	switch envelope.MessageType {
	case "error":
		b.errorCount++
		b.log.Error(fmt.Errorf("error occurred during backup"), envelope.Item+" during "+envelope.During+" "+envelope.Error.Op)
	case "status":
		b.statusFunc(b.log, envelope)
	case "summary":
		b.log.Info("backup finished", "new files", envelope.FilesNew, "changed files", envelope.FilesChanged, "errors", b.errorCount)
		b.log.Info("stats", "time", envelope.TotalDuration, "bytes added", envelope.DataAdded, "bytes processed", envelope.TotalBytesProcessed)
		b.summaryFunc(envelope.BackupSummary, b.errorCount, b.folder, 1, time.Now().Unix())
	}
}

func PrintStatus(logger logr.Logger, p *BackupEnvelope) {
	logger.Info("progress", "ETL", humanReadableTime(p.SecondsRemaining), "percentage", fmt.Sprintf("%.2f%%", p.PercentDone*100), "files", fmt.Sprintf("%d/%d", p.FilesDone, p.TotalFiles), "size", fmt.Sprintf("%s/%s", humanReadable(float64(p.BytesDone)), humanReadable(float64(p.TotalBytes))), "currentFile", p.CurrentFiles)
}

// This function generates a human-readable string for the given bytes. It will
// use the largest unit possible.
func humanReadable(bytes float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	unitIndex := 0
	for bytes > 1024 {
		bytes /= 1024
		unitIndex++
	}
	return fmt.Sprintf("%.2f%s", bytes, units[unitIndex])
}

// This function generates a human-readable string for time duration.
// It will use the largest unit possible. i.e. 1h30m instead of 90m
// The input is in seconds.
func humanReadableTime(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	seconds = seconds % 60

	result := ""

	if hours > 0 {
		result += fmt.Sprintf("%dh", hours)
	}
	if minutes > 0 {
		// Add a space for readability if hours were added
		if hours > 0 {
			result += " "
		}
		result += fmt.Sprintf("%dm", minutes)
	}
	if seconds > 0 {
		// Add a space for readability if hours or minutes were added
		if hours > 0 || minutes > 0 {
			result += " "
		}
		result += fmt.Sprintf("%ds", seconds)
	}

	// Handle the case where the duration is 0 seconds
	if result == "" {
		result = "0s"
	}

	return result
}

func IgnoreStatus(_ logr.Logger, _ *BackupEnvelope) {
	// NOOP
}
