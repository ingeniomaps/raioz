package docker

import (
	"context"
	"strings"
	"testing"

	"raioz/internal/naming"
)

// EnsureImage decides whether `raioz up` pulls. A wrong answer either
// stalls every start on a redundant pull or runs a container against an
// image that is not there.
func TestImageExistsAndEnsure(t *testing.T) {
	t.Run("present locally", func(t *testing.T) {
		fakeDocker(t, map[string]fakeReply{"image": {}})
		ok, err := ImageExists("nginx:alpine")
		if err != nil || !ok {
			t.Errorf("ImageExists() = %v, %v; want true, nil", ok, err)
		}
	})

	t.Run("absent is not an error", func(t *testing.T) {
		// `docker image inspect` exits 1 for an unknown image; treating
		// that as a failure would abort the up instead of pulling.
		fakeDocker(t, map[string]fakeReply{"image": {exitCode: 1}})
		ok, err := ImageExists("nginx:alpine")
		if err != nil || ok {
			t.Errorf("ImageExists() = %v, %v; want false, nil", ok, err)
		}
	})

	t.Run("present image is not pulled", func(t *testing.T) {
		argsFile := fakeDocker(t, map[string]fakeReply{"image": {}, "pull": {}})
		if err := EnsureImage("nginx:alpine"); err != nil {
			t.Fatalf("EnsureImage: %v", err)
		}
		if strings.Contains(fakeArgs(t, argsFile), "pull") {
			t.Error("an image already present must not be pulled again")
		}
	})

	t.Run("absent image gets pulled", func(t *testing.T) {
		argsFile := fakeDocker(t, map[string]fakeReply{"image": {exitCode: 1}, "pull": {}})
		if err := EnsureImage("nginx:alpine"); err != nil {
			t.Fatalf("EnsureImage: %v", err)
		}
		if !strings.Contains(fakeArgs(t, argsFile), "pull") {
			t.Error("a missing image must trigger a pull")
		}
	})
}

// Volumes hold the user's data. Creating one that already exists is
// harmless; removing one that does not must stay a no-op rather than an
// error that aborts a teardown midway.
func TestEnsureAndRemoveVolume(t *testing.T) {
	t.Run("existing volume is left alone", func(t *testing.T) {
		argsFile := fakeDocker(t, map[string]fakeReply{"volume inspect": {}})
		if err := EnsureVolume("pg-data"); err != nil {
			t.Fatalf("EnsureVolume: %v", err)
		}
		if strings.Contains(fakeArgs(t, argsFile), "create") {
			t.Error("an existing volume must not be re-created")
		}
	})

	t.Run("missing volume is created", func(t *testing.T) {
		argsFile := fakeDocker(t, map[string]fakeReply{
			"volume inspect": {exitCode: 1},
			"volume create":  {},
		})
		if err := EnsureVolume("pg-data"); err != nil {
			t.Fatalf("EnsureVolume: %v", err)
		}
		if !strings.Contains(fakeArgs(t, argsFile), "create") {
			t.Errorf("expected a volume create, got:\n%s", fakeArgs(t, argsFile))
		}
	})

	t.Run("removing an absent volume is a no-op", func(t *testing.T) {
		argsFile := fakeDocker(t, map[string]fakeReply{"volume inspect": {exitCode: 1}})
		if err := RemoveVolume("pg-data"); err != nil {
			t.Fatalf("RemoveVolume: %v", err)
		}
		if strings.Contains(fakeArgs(t, argsFile), "rm") {
			t.Error("nothing to remove; docker rm must not run")
		}
	})
}

// A network still holding containers belongs to a live project, so down
// must leave it. An unknown network is simply free.
func TestIsNetworkInUse(t *testing.T) {
	cases := []struct {
		name  string
		reply fakeReply
		want  bool
	}{
		{name: "two containers attached", reply: fakeReply{stdout: "2\n"}, want: true},
		{name: "empty network", reply: fakeReply{stdout: "0\n"}},
		{name: "unknown network", reply: fakeReply{exitCode: 1}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeDocker(t, map[string]fakeReply{"network": tc.reply})
			got, err := IsNetworkInUse("acme-net")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("IsNetworkInUse() = %v, want %v", got, tc.want)
			}
		})
	}
}

// Without labels the ls filter would match every network on the machine
// and the sweep would try to remove all of them.
func TestRemoveLabeledNetworksRequiresLabels(t *testing.T) {
	argsFile := fakeDocker(t, map[string]fakeReply{"network": {stdout: "some-net\n"}})
	got, err := RemoveLabeledNetworks(context.Background(), nil)
	if got != nil || err != nil {
		t.Errorf("got %v, %v; want nil, nil", got, err)
	}
	if fakeArgs(t, argsFile) != "" {
		t.Error("an unlabeled sweep must not reach docker at all")
	}
}

func TestRemoveLabeledNetworksSkipsBusyOnes(t *testing.T) {
	// The same `network` subcommand serves ls, inspect and rm here, so the
	// fake reports every network as holding a container — the sweep must
	// then remove nothing.
	fakeDocker(t, map[string]fakeReply{"network": {stdout: "acme-net\n"}})
	removed, err := RemoveLabeledNetworks(context.Background(),
		map[string]string{naming.LabelManaged: "true"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("a network still in use must survive, got %v", removed)
	}
}

// Re-attaching a container is how `up` reconciles a partially-connected
// stack, so docker's "already exists" has to read as success.
func TestConnectContainerToNetwork(t *testing.T) {
	t.Run("connects with aliases", func(t *testing.T) {
		argsFile := fakeDocker(t, map[string]fakeReply{"network": {}})
		err := ConnectContainerToNetwork(context.Background(), "acme-api", "acme-net",
			[]string{"api", "api.internal"})
		if err != nil {
			t.Fatalf("ConnectContainerToNetwork: %v", err)
		}
		args := fakeArgs(t, argsFile)
		for _, want := range []string{"connect", "--alias", "api.internal", "acme-net", "acme-api"} {
			if !strings.Contains(args, want) {
				t.Errorf("missing %q in:\n%s", want, args)
			}
		}
	})

	t.Run("already connected is not an error", func(t *testing.T) {
		fakeDocker(t, map[string]fakeReply{
			"network": {stdout: "endpoint already exists in network", exitCode: 1},
		})
		if err := ConnectContainerToNetwork(context.Background(), "c", "n", nil); err != nil {
			t.Errorf("already-connected must be a no-op, got %v", err)
		}
	})

	t.Run("real failures surface", func(t *testing.T) {
		fakeDocker(t, map[string]fakeReply{
			"network": {stdout: "no such container", exitCode: 1},
		})
		if err := ConnectContainerToNetwork(context.Background(), "c", "n", nil); err == nil {
			t.Error("expected a genuine connect failure to surface")
		}
	})
}
