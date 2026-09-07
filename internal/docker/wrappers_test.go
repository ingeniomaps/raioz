package docker

// These tests exercise the thin wrapper functions that dispatch to
// *WithContext variants and the early "file does not exist" / validation
// branches, so code can be covered without a Docker daemon. Functions that
// actually invoke the Docker CLI are not asserted for success; we only check
// that they return (error or not) without panicking.

import (
	"path/filepath"
	"testing"
)

func TestDownWrappers_MissingPath(t *testing.T) {
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "nothing.yml")

	// Down returns nil for missing path.
	if err := Down(missing); err != nil {
		t.Errorf("Down err = %v", err)
	}
	// StopServiceWithContext returns nil for missing path
	// and also nil for empty service name.
}

func TestRunnerWrappers_BadComposePath(t *testing.T) {
	// Path with dangerous chars fails validation.
	bad := "/tmp/compose;rm.yml"

	if err := Up(bad); err == nil {
		t.Error("Up should fail for invalid path")
	}
	if err := UpServicesWithContext(nil, bad, []string{"api"}); err == nil {
		// nil ctx is OK because validation runs before context use.
		t.Error("UpServicesWithContext should fail for invalid path")
	}
}

func TestGetServicesStatus_MissingPath(t *testing.T) {
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "nothing.yml")
	status, err := GetServicesStatus(missing)
	if err != nil {
		t.Errorf("GetServicesStatus err = %v", err)
	}
	if len(status) != 0 {
		t.Errorf("status = %v, want empty", status)
	}
}
