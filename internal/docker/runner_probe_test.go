package docker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// composeFixture writes a compose file the path validator accepts, so a
// test can reach the command-building code instead of stopping at the
// injection guard.
func composeFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "docker-compose.yml")
	if err := os.WriteFile(path, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose fixture: %v", err)
	}
	return path
}

// Every raioz command that talks to compose funnels through these, and
// the flags are the contract: --remove-orphans on the wrong scope, or a
// missing -f, tears down the wrong containers.
func TestComposeCommandsBuildTheRightArgv(t *testing.T) {
	compose := composeFixture(t)

	cases := []struct {
		name string
		run  func(ctx context.Context) error
		want []string
		deny []string
	}{
		{
			name: "up with a service subset",
			run: func(ctx context.Context) error {
				return UpServicesWithContext(ctx, compose, []string{"api"})
			},
			want: []string{"compose", "-f", compose, "up", "-d", "--remove-orphans", "api"},
		},
		{
			name: "restart names the services",
			run: func(ctx context.Context) error {
				return RestartServicesWithContext(ctx, compose, []string{"api", "web"})
			},
			want: []string{"compose", "restart", "api", "web"},
		},
		{
			name: "force recreate does not reuse containers",
			run: func(ctx context.Context) error {
				return ForceRecreateServicesWithContext(ctx, compose, []string{"api"})
			},
			want: []string{"--force-recreate", "api"},
		},
		{
			// Unlike up, down does not pass --remove-orphans: it tears
			// down what the file declares and leaves anything else to the
			// label sweep in the down flow.
			name: "down targets the compose file",
			run:  func(ctx context.Context) error { return DownWithContext(ctx, compose) },
			want: []string{"compose", "-f", compose, "down"},
			deny: []string{"--remove-orphans"},
		},
		{
			name: "stopping one service leaves the rest alone",
			run: func(ctx context.Context) error {
				return StopServiceWithContext(ctx, compose, "api")
			},
			want: []string{"compose", "stop", "api"},
			deny: []string{"down"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			argsFile := fakeDocker(t, map[string]fakeReply{"compose": {}})
			if err := tc.run(context.Background()); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			args := fakeArgs(t, argsFile)
			for _, want := range tc.want {
				if !strings.Contains(args, want) {
					t.Errorf("missing %q in:\n%s", want, args)
				}
			}
			for _, deny := range tc.deny {
				if strings.Contains(args, deny) {
					t.Errorf("unexpected %q in:\n%s", deny, args)
				}
			}
		})
	}
}

// The path is interpolated into a shell-less exec, but it still names a
// file the command will act on — the validator is what keeps a crafted
// value from pointing somewhere else. It has to run before docker does.
func TestComposeCommandsRejectBadPathsBeforeRunning(t *testing.T) {
	argsFile := fakeDocker(t, map[string]fakeReply{"compose": {}})

	if err := UpServicesWithContext(context.Background(), "", nil); err == nil {
		t.Error("an empty compose path must be rejected")
	}
	// down short-circuits earlier still: no compose file on disk means
	// the project is already down, so there is nothing to validate.
	if err := DownWithContext(context.Background(), ""); err != nil {
		t.Errorf("a missing compose file means already-down, got %v", err)
	}
	if fakeArgs(t, argsFile) != "" {
		t.Error("the guards must run before anything reaches docker")
	}
}

// `down` stops containers one by one during teardown. A container that
// is already gone, or already stopped, is the normal case on a re-run —
// neither may abort the sweep.
func TestStopContainerWithContext(t *testing.T) {
	cases := []struct {
		name    string
		reply   fakeReply
		wantErr bool
	}{
		{name: "stopped fine", reply: fakeReply{}},
		{name: "no such container", reply: fakeReply{stdout: "Error: No such container: x", exitCode: 1}},
		{name: "already stopped", reply: fakeReply{stdout: "Container is not running", exitCode: 1}},
		{name: "real failure", reply: fakeReply{stdout: "permission denied", exitCode: 1}, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeDocker(t, map[string]fakeReply{"stop": tc.reply})
			err := StopContainerWithContext(context.Background(), "c1")
			if (err != nil) != tc.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}

	t.Run("empty name is a no-op", func(t *testing.T) {
		argsFile := fakeDocker(t, map[string]fakeReply{"stop": {}})
		if err := StopContainerWithContext(context.Background(), ""); err != nil {
			t.Errorf("empty name should be ignored, got %v", err)
		}
		if fakeArgs(t, argsFile) != "" {
			t.Error("an empty name must not reach docker")
		}
	})
}

// raioz supports podman and nerdctl through RAIOZ_RUNTIME. These calls
// used to hardcode "docker", so on a podman-only machine `status` could
// not resolve container names and reported every service as stopped.
func TestComposePsHonorsTheConfiguredRuntime(t *testing.T) {
	compose := composeFixture(t)
	argsFile := fakeDocker(t, map[string]fakeReply{
		"compose": {stdout: "deadbeef\n"},
		"inspect": {stdout: "/acme-api\n"},
	})

	got, err := GetContainerNameWithContext(context.Background(), compose, "api")
	if err != nil {
		t.Fatalf("GetContainerNameWithContext: %v", err)
	}
	if got != "acme-api" {
		t.Errorf("name = %q, want acme-api (leading slash trimmed)", got)
	}
	if args := fakeArgs(t, argsFile); !strings.Contains(args, "compose") {
		t.Errorf("the configured runtime must receive the compose call, got:\n%s", args)
	}
}

func TestGetContainerNameForStoppedService(t *testing.T) {
	compose := composeFixture(t)
	// `compose ps -q` prints nothing when the service is not running.
	fakeDocker(t, map[string]fakeReply{"compose": {stdout: "\n"}})

	got, err := GetContainerNameWithContext(context.Background(), compose, "api")
	if err != nil {
		t.Fatalf("GetContainerNameWithContext: %v", err)
	}
	if got != "" {
		t.Errorf("a service that is not running has no container, got %q", got)
	}
}
