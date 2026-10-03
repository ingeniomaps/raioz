package app

import (
	"context"
	"errors"
	"os"
	"testing"

	"raioz/internal/domain/interfaces"
)

// TestMain keeps the package's tests off the machine they run on: the
// global state is read from an empty directory instead of the developer's
// own, and stopping "another project" never re-executes the test binary
// in a real project directory.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "raioz-app-test-")
	if err != nil {
		panic(err)
	}
	os.Setenv("RAIOZ_HOME", home)
	// A test that clears RAIOZ_HOME falls back to the XDG base; keep that
	// off the real state dir too.
	os.Setenv("XDG_STATE_HOME", home)
	downProjectFn = func(context.Context, string) error {
		return errors.New("downProjectFn is disabled in tests; stub it")
	}
	// These reach the Docker daemon of the machine running the tests, and
	// the last one removes containers. A test that needs them stubs them
	// (withDownOthersHooks); none may ever fall through to the real thing.
	listActiveProjectsFn = func(context.Context) ([]string, error) { return nil, nil }
	listPublishedPortsFn = func(context.Context) ([]publishedPort, error) { return nil, nil }
	stopProjectContainersFn = func(context.Context, string) ([]string, error) {
		return nil, errors.New("stopProjectContainersFn is disabled in tests; stub it")
	}
	// Nor launch a host service.
	startHostServiceFn = func(context.Context, interfaces.DockerRunner, interfaces.ServiceContext) (int, error) {
		return 0, errors.New("starting host services is disabled in tests")
	}
	// Nor tear a container down and start it again.
	recreateTargetFn = func(
		context.Context, interfaces.DockerRunner, func() (interfaces.ServiceContext, bool),
	) error {
		return errors.New("recreate is disabled in tests")
	}

	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}
