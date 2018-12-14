package restic

import (
	"errors"
)

// CheckStruct holds the state of the check command.
type CheckStruct struct {
	genericCommand
}

func newCheck() *CheckStruct {
	return &CheckStruct{}
}

// Check runs the check command.
func (c *CheckStruct) Check() {
	args := []string{"check"}
	c.genericCommand.exec(args, commandOptions{print: true})
	if len(c.stdErrOut) > 0 {
		c.errorMessage = errors.New("There was at least one backup error")
	}
}
