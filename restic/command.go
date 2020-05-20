package restic

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

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

// Run will run the currently configured command
func (c *Command) Run() {

	c.Configure()

	c.Start()

	c.Wait()

}

func (c *Command) Configure() {
	c.cmd = exec.CommandContext(c.ctx, c.options.Path, c.options.Args...)
	c.cmd.Env = os.Environ()

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

func (c *Command) Start() {
	if c.cmd == nil {
		c.FatalError = fmt.Errorf("command not configured")
		return
	}
	err := c.cmd.Start()
	if err != nil {
		c.FatalError = fmt.Errorf("cmd.Start() err: %v", err)
		return
	}
}

func (c *Command) Wait() {
	if c.cmd == nil {
		c.FatalError = fmt.Errorf("command not configured")
		return
	}
	err := c.cmd.Wait()
	if err != nil {
		if err == io.EOF {
			return
		}
		c.FatalError = fmt.Errorf("cmd.Wait() err: %v", err)
		return
	}
}
