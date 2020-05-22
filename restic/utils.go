package restic

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
)

type backupOutputParser struct {
	log         logr.Logger
	errorCount  int
	lineCounter int
	summaryfunc func(summary backupSummary, errorCount int, folder string, startTimestamp, endTimestamp int64)
	folder      string
}

// LineParser takes a single string from an output and passes it the the
// concrete implementation
type LineParser interface {
	Parse(s string) error
}

// outputWrapper will split the output into lines.
type outputWrapper struct {
	parser LineParser
}

func (s *outputWrapper) Write(p []byte) (n int, err error) {

	scanner := bufio.NewScanner(bytes.NewReader(p))

	for scanner.Scan() {
		err := s.parser.Parse(scanner.Text())
		if err != nil {
			return len(p), err
		}
	}

	return len(p), nil
}

type logOutParser struct {
	log logr.Logger
}

func (l *logOutParser) Parse(s string) error {
	l.log.Info(s)
	return nil
}

type logErrParser struct {
	log logr.Logger
}

func (l *logErrParser) Parse(s string) error {
	l.log.Error(fmt.Errorf("error during command"), s)
	return nil
}

func (b *backupOutputParser) Parse(s string) error {
	envelope := &backupEnvelope{}

	err := json.Unmarshal([]byte(s), envelope)
	if err != nil {
		b.log.Error(err, "can't decode restic json output", "string", s)
		return err
	}

	switch envelope.MessageType {
	case "error":
		b.errorCount++
		b.log.Error(fmt.Errorf("error occurred during backup"), envelope.Item+" during "+envelope.During+" "+envelope.Error.Op)
	case "status":
		// Restic does the json output with 60hz, which is a bit much...
		if b.lineCounter%60 == 0 {
			percent := envelope.PercentDone * 100
			b.log.Info("progress of backup", "percentage", fmt.Sprintf("%.2f%%", percent))
		}
		b.lineCounter++
	case "summary":
		b.log.Info("backup finished", "new files", envelope.FilesNew, "changed files", envelope.FilesChanged, "errors", b.errorCount)
		b.log.Info("stats", "time", envelope.TotalDuration, "bytes added", envelope.DataAdded, "bytes processed", envelope.TotalBytesProcessed)
		b.summaryfunc(envelope.backupSummary, b.errorCount, b.folder, 1, time.Now().Unix())
	}
	return nil
}
