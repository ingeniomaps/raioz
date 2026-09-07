package docker

import (
	"context"
	"testing"

	"raioz/internal/naming"
)

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
