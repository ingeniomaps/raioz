// Package runtime provides the container runtime binary name.
// Defaults to "docker" but can be set to "podman", "nerdctl", etc.
// via the RAIOZ_RUNTIME environment variable or SetBinary().
package runtime

import "os"

// defaultBinary is used when no override is set.
const defaultBinary = "docker"

var binary string

func init() {
	binary = defaultBinary
	if env := os.Getenv("RAIOZ_RUNTIME"); env != "" {
		binary = env
	}
}

// Binary returns the container runtime binary name ("docker", "podman", etc.).
func Binary() string {
	return binary
}

// ComposeBinary returns the compose subcommand.
// For docker: "docker compose". For podman: "podman compose".
func ComposeBinary() (string, string) {
	return binary, "compose"
}

// SetBinary overrides the container runtime binary.
func SetBinary(b string) {
	if b != "" {
		binary = b
	}
}

// IsDocker returns true if the current runtime is Docker.
func IsDocker() bool {
	return binary == "docker"
}
