package docker

import (
	"context"
	"strings"
	"testing"
	"time"

	"raioz/internal/naming"
)

// The subnet is what makes container IPs deterministic (proxy.publish
// false depends on it) and the labels are what lets `down` sweep the
// network later — both have to reach the command line.
func TestCreateNetworkPassesSubnetAndLabels(t *testing.T) {
	argsFile := fakeDocker(t, map[string]fakeReply{"network": {}})

	err := CreateNetworkWithConfigAndContext(context.Background(), NetworkConfig{
		Name:   "acme-net",
		Subnet: "172.28.0.0/16",
		Labels: map[string]string{
			naming.LabelManaged:   "true",
			naming.LabelWorkspace: "acme",
		},
	}, false)
	if err != nil {
		t.Fatalf("CreateNetworkWithConfigAndContext: %v", err)
	}

	args := fakeArgs(t, argsFile)
	for _, want := range []string{
		"--driver", "bridge", "--subnet", "172.28.0.0/16",
		naming.LabelManaged + "=true", naming.LabelWorkspace + "=acme", "acme-net",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("missing %q in:\n%s", want, args)
		}
	}
}

func TestCreateNetworkSurfacesFailure(t *testing.T) {
	fakeDocker(t, map[string]fakeReply{
		"network": {stdout: "pool overlaps with other one", exitCode: 1},
	})
	err := CreateNetworkWithConfigAndContext(
		context.Background(), NetworkConfig{Name: "acme-net"}, false)
	if err == nil {
		t.Fatal("expected the create failure to surface")
	}
	if !strings.Contains(err.Error(), "overlaps") {
		t.Errorf("docker's own output should reach the user, got %q", err)
	}
}

// ADR-025's launcher pattern waits here: a `command:` that shells out to
// compose returns before its container exists, and raioz must not report
// ready until it does.
func TestWaitForContainer(t *testing.T) {
	t.Run("already there returns immediately", func(t *testing.T) {
		fakeDocker(t, map[string]fakeReply{"inspect": {stdout: "running\n"}})
		start := time.Now()
		if err := WaitForContainer(context.Background(), "c1", 5*time.Second); err != nil {
			t.Fatalf("WaitForContainer: %v", err)
		}
		if elapsed := time.Since(start); elapsed > time.Second {
			t.Errorf("the first probe must not wait for the tick, took %v", elapsed)
		}
	})

	t.Run("never appears", func(t *testing.T) {
		fakeDocker(t, map[string]fakeReply{"inspect": {exitCode: 1}})
		err := WaitForContainer(context.Background(), "c1", 1200*time.Millisecond)
		if err == nil {
			t.Fatal("expected a timeout error")
		}
		if !strings.Contains(err.Error(), "did not appear") {
			t.Errorf("error should say the container never showed up, got %q", err)
		}
	})

	t.Run("cancellation is honored", func(t *testing.T) {
		fakeDocker(t, map[string]fakeReply{"inspect": {exitCode: 1}})
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()
		if err := WaitForContainer(ctx, "c1", time.Minute); err == nil {
			t.Error("a cancelled context must abort the wait")
		}
	})

	t.Run("guards its inputs", func(t *testing.T) {
		if err := WaitForContainer(context.Background(), "", time.Second); err == nil {
			t.Error("empty name must be rejected")
		}
		if err := WaitForContainer(context.Background(), "c1", 0); err == nil {
			t.Error("a non-positive timeout must be rejected")
		}
	})
}

// The port allocator asks "who holds this port?" to decide between
// reusing a shared dep, prompting, or failing. Mislabeling a foreign
// container as raioz-owned would let it stop someone else's service.
func TestIdentifyPortOccupant(t *testing.T) {
	t.Run("not a container", func(t *testing.T) {
		fakeDocker(t, map[string]fakeReply{"ps": {stdout: "\n"}})
		occ := IdentifyPortOccupant(context.Background(), 5432)
		if occ.IsDocker || occ.IsRaioz {
			t.Errorf("nothing published the port; got %+v", occ)
		}
		if occ.Port != 5432 {
			t.Errorf("Port = %d, want 5432", occ.Port)
		}
	})

	t.Run("raioz container reports its labels", func(t *testing.T) {
		fakeDocker(t, map[string]fakeReply{
			"ps":      {stdout: "acme-postgres\tdeadbeef\n"},
			"inspect": {stdout: "true\n"},
		})
		occ := IdentifyPortOccupant(context.Background(), 5432)
		if !occ.IsDocker || occ.ContainerName != "acme-postgres" || occ.ContainerID != "deadbeef" {
			t.Fatalf("container identity not parsed: %+v", occ)
		}
		if !occ.IsRaioz {
			t.Error("a container stamped com.raioz.managed=true is ours")
		}
	})

	t.Run("foreign container is not claimed", func(t *testing.T) {
		fakeDocker(t, map[string]fakeReply{
			"ps":      {stdout: "someone-elses-db\tcafe\n"},
			"inspect": {stdout: "<no value>\n"},
		})
		occ := IdentifyPortOccupant(context.Background(), 5432)
		if !occ.IsDocker {
			t.Fatal("it is still a container")
		}
		if occ.IsRaioz {
			t.Error("an unlabeled container must not be claimed as raioz-managed")
		}
	})
}
