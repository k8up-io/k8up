package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/go-logr/logr"
)

// CommandOptions contains options for the command struct.
type CommandOptions struct {
	Path   string    // path where the command is to be executed
	StdIn  io.Reader // set the StdIn for the command
	StdOut io.Writer // set the StdOut for the command
	StdErr io.Writer // set StdErr for the command
	Args   []string
}

// Command can handle a given command.
type Command struct {
	options    CommandOptions
	FatalError error
	Errors     []error
	cmdLogger  logr.Logger
	ctx        context.Context
	cmd        *exec.Cmd
}

// NewCommand returns a new command
func NewCommand(ctx context.Context, log logr.Logger, commandOptions CommandOptions) *Command {
	return &Command{
		options:   commandOptions,
		Errors:    []error{},
		cmdLogger: log.WithName("command"),
		ctx:       ctx,
	}
}

// Run will run the currently configured command and wait for its completion.
func (c *Command) Run() {

	c.Configure()

	c.Start()

	c.Wait()

}

// Configure will setup the command object.
// Mainly set the env vars and wire the right stdins/outs.
func (c *Command) Configure() {
	c.cmdLogger.Info("restic command", "path", c.options.Path, "args", c.options.Args)
	c.cmd = exec.CommandContext(c.ctx, c.options.Path, c.options.Args...)
	osEnv := os.Environ()
	c.cmd.Env = c.setResticProgressFPSIfNotDefined(osEnv)

	if c.options.StdIn != nil {
		c.cmd.Stdin = c.options.StdIn
	}

	if c.options.StdOut != nil {
		c.cmd.Stdout = c.options.StdOut
	}

	if c.options.StdErr != nil {
		c.cmd.Stderr = c.options.StdErr
	}
}

func (c *Command) setResticProgressFPSIfNotDefined(givenEnv []string) []string {
	for _, envVar := range givenEnv {
		if strings.HasPrefix("RESTIC_PROGRESS_FPS=", envVar) {
			return givenEnv
		}
	}

	const frequency = 1.0 / 60.0
	c.cmdLogger.Info("Defining RESTIC_PROGRESS_FPS", "frequency", frequency)
	return append(givenEnv, fmt.Sprintf("RESTIC_PROGRESS_FPS=%f", frequency))
}

// Start starts the specified command but does not wait for it to complete.
func (c *Command) Start() {
	if c.cmd == nil {
		c.FatalError = fmt.Errorf("command not configured")
		return
	}
	err := c.cmd.Start()
	if err != nil {
		c.FatalError = fmt.Errorf("cmd.Start() err: %w", err)
		return
	}
}

// Wait waits for the command to exit and waits for any copying to stdin or copying from stdout or stderr to complete.
func (c *Command) Wait() {
	if c.cmd == nil {
		c.FatalError = fmt.Errorf("command not configured")
		return
	}

	if c.cmd.Process == nil {
		c.FatalError = fmt.Errorf("the process did not start, please check if execution bit is set")
		return
	}

	err := c.cmd.Wait()
	if err != nil {
		// The error could contain an IO error...
		// io.EOF is ignored as this will be sent if we close the pipes somewhere, which is expected.
		if err == io.EOF {
			return
		}
		// ...as well as an exiterror
		if exiterr, ok := err.(*exec.ExitError); ok {
			// We ignore exit code 3 as this will be set if the snapshot was created but some files failed to read.
			// This is handled by the backup summary parsing.
			// See https://restic.readthedocs.io/en/stable/040_backup.html?highlight=exit%20code#exit-status-codes
			if exiterr.ExitCode() != 3 {
				c.FatalError = fmt.Errorf("cmd.Wait() err: %d", exiterr.ExitCode())
			}
		} else { // if it's some other error we'd need to catch it, too
			c.FatalError = fmt.Errorf("cmd.Wait() err: %w", err)
		}
		return
	}
}
