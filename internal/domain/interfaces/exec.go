package interfaces

import (
	"context"
	"io"
)

// CommandExecutor defines operations for executing external commands
type CommandExecutor interface {
	// Command creates a new command
	Command(name string, args ...string) Command
	// CommandContext creates a new command with context
	CommandContext(ctx context.Context, name string, args ...string) Command
}

// Command represents an external command to be executed
type Command interface {
	// Run runs the command
	Run() error
	// Output runs the command and returns its output
	Output() ([]byte, error)
	// CombinedOutput runs the command and returns combined stdout and stderr
	CombinedOutput() ([]byte, error)
	// Start starts the command
	Start() error
	// Wait waits for the command to finish
	Wait() error
	// SetDir sets the working directory
	SetDir(dir string)
	// SetStdout sets stdout
	SetStdout(w io.Writer)
	// SetStderr sets stderr
	SetStderr(w io.Writer)
	// SetStdin sets stdin
	SetStdin(r io.Reader)
}
