package docker

import (
	"context"
	"testing"
)

func TestDepVolume_BindMounts(t *testing.T) {
	for _, spec := range []string{"./data:/data", "/var/lib/x:/data", "~/x:/data", ":/data", ""} {
		if name, named, _ := DepVolume(context.Background(), "bencha", "cache", spec); named {
			t.Errorf("DepVolume(%q) = %q named, want a bind mount", spec, name)
		}
	}
}
