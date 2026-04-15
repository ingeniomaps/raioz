package git

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// TestGetCommitSHA_RealRepo exercises the happy path of GetCommitSHA against
// a freshly initialised local repository. It verifies the returned SHA is
// non-empty and shortened to 12 characters.
func TestGetCommitSHA_RealRepo(t *testing.T) {
	skipIfNoGit(t)
	dir := initLocalRepo(t, filepath.Join(t.TempDir(), "repo"), "main")

	sha, err := GetCommitSHA(context.Background(), dir)
	if err != nil {
		t.Fatalf("GetCommitSHA error = %v", err)
	}
	if sha == "" {
		t.Error("GetCommitSHA returned empty string")
	}
	if len(sha) != 12 {
		t.Errorf("GetCommitSHA length = %d, want 12", len(sha))
	}
	// Short SHA must be hex.
	for _, r := range sha {
		isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
		if !isHex {
			t.Errorf("GetCommitSHA returned non-hex char %q", r)
			break
		}
	}
}

// TestGetCommitDate_RealRepo exercises the happy path of GetCommitDate.
func TestGetCommitDate_RealRepo(t *testing.T) {
	skipIfNoGit(t)
	dir := initLocalRepo(t, filepath.Join(t.TempDir(), "repo"), "main")

	date, err := GetCommitDate(context.Background(), dir)
	if err != nil {
		t.Fatalf("GetCommitDate error = %v", err)
	}
	if date == "" {
		t.Error("GetCommitDate returned empty string")
	}
	// Format from git log --format=%ci looks like "2025-01-02 03:04:05 +0000".
	if !strings.Contains(date, "-") || !strings.Contains(date, ":") {
		t.Errorf("GetCommitDate unexpected format: %q", date)
	}
}
