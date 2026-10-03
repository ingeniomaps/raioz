package app

import (
	"context"
	"errors"
	"os"
	"testing"
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

	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}
