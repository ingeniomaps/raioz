package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"raioz/internal/runtime"
	"strings"
)

// ExecInService runs a command inside a running container
func ExecInService(
	ctx context.Context, composePath string, serviceName string,
	command []string, interactive bool,
) error {
	if err := ValidateComposePath(composePath); err != nil {
		return fmt.Errorf("invalid compose path: %w", err)
	}

	args := []string{"compose", "-f", composePath, "exec"}
	if !interactive {
		args = append(args, "-T")
	}
	args = append(args, serviceName)
	args = append(args, command...)

	cmd := exec.CommandContext(ctx, runtime.Binary(), args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	var stderrBuf bytes.Buffer
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	if err := cmd.Run(); err != nil {
		captured := strings.TrimSpace(stderrBuf.String())
		if captured != "" {
			return fmt.Errorf("%s", captured)
		}
		return err
	}

	return nil
}
