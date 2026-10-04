package docker

import (
	"os"

	"raioz/internal/fsutil"
	"raioz/internal/runtime"
)

// quietPullArgs silences the image pull of `compose up` when its progress
// would land in a file or a pipe. On a terminal compose redraws one line
// per layer; anywhere else it prints every update as a new line, hundreds
// of them per image, and buries what raioz itself reports. The flag is
// only passed to docker, the one runtime known to take it.
func quietPullArgs(out *os.File) []string {
	if fsutil.IsTerminal(out) || runtime.Binary() != "docker" {
		return nil
	}
	return []string{"--quiet-pull"}
}
