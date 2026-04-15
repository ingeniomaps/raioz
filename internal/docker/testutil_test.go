package docker

import (
	"os"
	"testing"
)

// requireDocker skips the test unless RAIOZ_DOCKER_TESTS=1 is set.
// This prevents Docker integration tests from running during normal
// `go test ./...` invocations, which can spawn dozens of containers
// in parallel and destabilize the system.
func requireDocker(t *testing.T) {
	t.Helper()
	if os.Getenv("RAIOZ_DOCKER_TESTS") != "1" {
		t.Skip("skipping Docker integration test (set RAIOZ_DOCKER_TESTS=1 to run)")
	}
}
