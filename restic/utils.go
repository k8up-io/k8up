package restic

import (
	"bufio"
	"bytes"
	"fmt"

	"github.com/go-logr/logr"
)

// LineParser takes a single string from an output and passes it the the
// concrete implementation
type LineParser interface {
	Parse(s string)
}

// outputWrapper will split the output into lines.
type outputWrapper struct {
	parser LineParser
}

func (s *outputWrapper) Write(p []byte) (n int, err error) {

	scanner := bufio.NewScanner(bytes.NewReader(p))

	for scanner.Scan() {
		s.parser.Parse(scanner.Text())
	}

	return len(p), nil
}

type logOutParser struct {
	log logr.Logger
}

func (l *logOutParser) Parse(s string) {
	l.log.Info(s)
}

type logErrParser struct {
	log logr.Logger
}

func (l *logErrParser) Parse(s string) {
	l.log.Error(fmt.Errorf("error during command"), s)
}
