package docker

import (
	"context"
	"testing"
)

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
