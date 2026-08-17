package docker

import (
	"context"
	"strings"
	"testing"
)

// `raioz clean --dry-run` is what a user runs before trusting the real
// thing, so the dry path must report without removing anything.
func TestCleanUnusedImages(t *testing.T) {
	t.Run("dry run lists without pruning", func(t *testing.T) {
		argsFile := fakeDocker(t, map[string]fakeReply{"images": {stdout: "sha256:aaa\nsha256:bbb\n"}})
		actions, err := CleanUnusedImages(true)
		if err != nil {
			t.Fatalf("CleanUnusedImages: %v", err)
		}
		if len(actions) != 2 {
			t.Errorf("actions = %v, want one per dangling image", actions)
		}
		for _, a := range actions {
			if !strings.HasPrefix(a, "Would remove") {
				t.Errorf("dry run must only describe, got %q", a)
			}
		}
		if strings.Contains(fakeArgs(t, argsFile), "prune") {
			t.Error("a dry run must never prune")
		}
	})

	t.Run("dry run with nothing to clean", func(t *testing.T) {
		fakeDocker(t, map[string]fakeReply{"images": {stdout: "\n"}})
		actions, err := CleanUnusedImages(true)
		if err != nil {
			t.Fatalf("CleanUnusedImages: %v", err)
		}
		if len(actions) != 1 || !strings.Contains(actions[0], "No unused images") {
			t.Errorf("actions = %v, want a single 'nothing found' line", actions)
		}
	})

	t.Run("real run prunes", func(t *testing.T) {
		argsFile := fakeDocker(t, map[string]fakeReply{"image": {stdout: "Total reclaimed space: 1.2GB\n"}})
		if _, err := CleanUnusedImages(false); err != nil {
			t.Fatalf("CleanUnusedImages: %v", err)
		}
		if args := fakeArgs(t, argsFile); !strings.Contains(args, "prune") {
			t.Errorf("expected an image prune, got:\n%s", args)
		}
	})

	t.Run("docker failure surfaces", func(t *testing.T) {
		fakeDocker(t, map[string]fakeReply{"images": {exitCode: 1}})
		if _, err := CleanUnusedImages(true); err == nil {
			t.Error("expected the docker failure to surface")
		}
	})
}

func TestCleanUnusedNetworks(t *testing.T) {
	t.Run("dry run lists without pruning", func(t *testing.T) {
		argsFile := fakeDocker(t, map[string]fakeReply{"network": {stdout: "old-net\n"}})
		actions, err := CleanUnusedNetworks(true)
		if err != nil {
			t.Fatalf("CleanUnusedNetworks: %v", err)
		}
		if len(actions) == 0 {
			t.Error("expected the unused network to be reported")
		}
		if strings.Contains(fakeArgs(t, argsFile), "prune") {
			t.Error("a dry run must never prune")
		}
	})

	t.Run("real run prunes", func(t *testing.T) {
		argsFile := fakeDocker(t, map[string]fakeReply{"network": {stdout: "old-net\n"}})
		if _, err := CleanUnusedNetworks(false); err != nil {
			t.Fatalf("CleanUnusedNetworks: %v", err)
		}
		if args := fakeArgs(t, argsFile); !strings.Contains(args, "prune") {
			t.Errorf("expected a network prune, got:\n%s", args)
		}
	})
}

// `raioz status` shows this; a malformed inspect line must not produce a
// half-filled map the caller then prints as truth.
func TestGetImageInfo(t *testing.T) {
	t.Run("parsed", func(t *testing.T) {
		fakeDocker(t, map[string]fakeReply{
			"image": {stdout: "sha256:abc|[nginx:alpine]|2026-01-01T00:00:00Z\n"},
		})
		info, err := GetImageInfo("nginx:alpine")
		if err != nil {
			t.Fatalf("GetImageInfo: %v", err)
		}
		if info["id"] != "sha256:abc" {
			t.Errorf("id = %q, want sha256:abc", info["id"])
		}
	})

	t.Run("unknown image", func(t *testing.T) {
		fakeDocker(t, map[string]fakeReply{"image": {exitCode: 1}})
		if _, err := GetImageInfo("nope"); err == nil {
			t.Error("expected an error for an image docker cannot inspect")
		}
	})
}

// The proxy backfills a dep's port from the image when the yaml declares
// none. A wrong pick sends every request to a port nothing listens on.
func TestGetImageExposedPort(t *testing.T) {
	t.Run("lowest tcp port wins", func(t *testing.T) {
		SetExposedPortCacheForTest("pgadmin", 0, nil)
		fakeDocker(t, map[string]fakeReply{
			"image": {stdout: `{"443/tcp":{},"80/tcp":{}}`},
		})
		got, err := GetImageExposedPort(context.Background(), "multi-port-image")
		if err != nil {
			t.Fatalf("GetImageExposedPort: %v", err)
		}
		if got != 80 {
			t.Errorf("port = %d, want the lowest TCP port 80", got)
		}
	})

	t.Run("no ports declared", func(t *testing.T) {
		fakeDocker(t, map[string]fakeReply{"image": {stdout: "null"}})
		if _, err := GetImageExposedPort(context.Background(), "portless-image"); err == nil {
			t.Error("an image without ExposedPorts must report that, not guess")
		}
	})

	t.Run("answers come from the cache the second time", func(t *testing.T) {
		SetExposedPortCacheForTest("cached-image", 6379, nil)
		argsFile := fakeDocker(t, map[string]fakeReply{"image": {stdout: `{"1/tcp":{}}`}})
		got, err := GetImageExposedPort(context.Background(), "cached-image")
		if err != nil || got != 6379 {
			t.Fatalf("got %d, %v; want the cached 6379", got, err)
		}
		if fakeArgs(t, argsFile) != "" {
			t.Error("a cached image must not be inspected again")
		}
	})
}
