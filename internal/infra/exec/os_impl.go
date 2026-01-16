package exec

import (
	"context"
	"io"
	"os/exec"

	execinterfaces "raioz/internal/domain/interfaces"
)

// Ensure OSCommandExecutor implements execinterfaces.CommandExecutor
var _ execinterfaces.CommandExecutor = (*OSCommandExecutor)(nil)

// OSCommandExecutor is the concrete implementation of CommandExecutor using os/exec
type OSCommandExecutor struct{}

// NewOSCommandExecutor creates a new OSCommandExecutor implementation
func NewOSCommandExecutor() execinterfaces.CommandExecutor {
	return &OSCommandExecutor{}
}

// Command creates a new command
func (e *OSCommandExecutor) Command(name string, args ...string) execinterfaces.Command {
	return &OSCommand{
		cmd: exec.Command(name, args...),
	}
}

// CommandContext creates a new command with context
func (e *OSCommandExecutor) CommandContext(ctx context.Context, name string, args ...string) execinterfaces.Command {
	return &OSCommand{
		cmd: exec.CommandContext(ctx, name, args...),
	}
}

// OSCommand is the concrete implementation of Command
type OSCommand struct {
	cmd *exec.Cmd
}

// Run runs the command
func (c *OSCommand) Run() error {
	return c.cmd.Run()
}

// Output runs the command and returns its output
func (c *OSCommand) Output() ([]byte, error) {
	return c.cmd.Output()
}

// CombinedOutput runs the command and returns combined stdout and stderr
func (c *OSCommand) CombinedOutput() ([]byte, error) {
	return c.cmd.CombinedOutput()
}

// Start starts the command
func (c *OSCommand) Start() error {
	return c.cmd.Start()
}

// Wait waits for the command to finish
func (c *OSCommand) Wait() error {
	return c.cmd.Wait()
}

// SetDir sets the working directory
func (c *OSCommand) SetDir(dir string) {
	c.cmd.Dir = dir
}

// SetStdout sets stdout
func (c *OSCommand) SetStdout(w io.Writer) {
	c.cmd.Stdout = w
}

// SetStderr sets stderr
func (c *OSCommand) SetStderr(w io.Writer) {
	c.cmd.Stderr = w
}

// SetStdin sets stdin
func (c *OSCommand) SetStdin(r io.Reader) {
	c.cmd.Stdin = r
}
