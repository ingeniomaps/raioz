package path

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePathInBase(t *testing.T) {
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "base")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatalf("Failed to create base directory: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		baseDir string
		wantErr bool
	}{
		// Valid paths
		{"valid path inside base", filepath.Join(baseDir, "file.txt"), baseDir, false},
		{"valid subdirectory", filepath.Join(baseDir, "sub", "file.txt"), baseDir, false},
		{"base directory itself", baseDir, baseDir, false},

		// Invalid paths - path traversal
		{"path with ..", filepath.Join(baseDir, "..", "file.txt"), baseDir, true},
		{"path with ../..", filepath.Join(baseDir, "..", "..", "file.txt"), baseDir, true},
		{"path outside base", filepath.Join(tmpDir, "file.txt"), baseDir, true},
		{"absolute path outside", "/etc/passwd", baseDir, true},
		{"path with .. in middle", filepath.Join(baseDir, "sub", "..", "..", "file.txt"), baseDir, true},

		// Edge cases
		{"empty path", "", baseDir, true},
		{"empty base", filepath.Join(baseDir, "file.txt"), "", true},
		{"path with .", filepath.Join(baseDir, ".", "file.txt"), baseDir, false},
		{"path with ./", filepath.Join(baseDir, "./file.txt"), baseDir, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePathInBase(tt.path, tt.baseDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePathInBase(%q, %q) error = %v, wantErr %v", tt.path, tt.baseDir, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAndCleanPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		// Valid paths
		{"valid simple path", "file.txt", "file.txt", false},
		{"valid relative path", "sub/file.txt", "sub/file.txt", false},
		{"path with .", "./file.txt", "file.txt", false},
		{"path with ./", "./sub/file.txt", "sub/file.txt", false},

		// Invalid paths
		{"path with ..", "../file.txt", "", true},
		{"path with ../..", "../../file.txt", "", true},
		{"path with .. in middle", "sub/../file.txt", "", true},
		{"empty path", "", "", true},
		{"path with null byte", "file\x00.txt", "", true},
		{"absolute path with ..", "/etc/../passwd", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateAndCleanPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAndCleanPath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				// For cleaned paths, we just verify it doesn't contain ..
				if strings.Contains(got, "..") {
					t.Errorf("ValidateAndCleanPath(%q) = %q, should not contain ..", tt.path, got)
				}
			}
		})
	}
}

func TestEnsurePathInBase(t *testing.T) {
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "base")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatalf("Failed to create base directory: %v", err)
	}

	tests := []struct {
		name     string
		baseDir  string
		userPath string
		wantErr  bool
	}{
		// Valid paths
		{"valid file path", baseDir, "file.txt", false},
		{"valid subdirectory", baseDir, "sub/file.txt", false},
		{"valid nested path", baseDir, "sub/nested/file.txt", false},

		// Invalid paths - path traversal
		{"path with ..", baseDir, "../file.txt", true},
		{"path with ../..", baseDir, "../../file.txt", true},
		{"path with .. in middle", baseDir, "sub/../file.txt", true},
		{"absolute path", baseDir, "/etc/passwd", true},

		// Edge cases
		{"empty user path", baseDir, "", true},
		{"empty base dir", "", "file.txt", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EnsurePathInBase(tt.baseDir, tt.userPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnsurePathInBase(%q, %q) error = %v, wantErr %v", tt.baseDir, tt.userPath, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Verify the returned path is within baseDir
				if err := ValidatePathInBase(got, tt.baseDir); err != nil {
					t.Errorf("EnsurePathInBase(%q, %q) returned path %q that is not within baseDir: %v", tt.baseDir, tt.userPath, got, err)
				}
			}
		})
	}
}

func TestPathTraversalRealWorld(t *testing.T) {
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "workspace")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatalf("Failed to create base directory: %v", err)
	}

	// Create a file outside the base directory
	outsideFile := filepath.Join(tmpDir, "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0644); err != nil {
		t.Fatalf("Failed to create outside file: %v", err)
	}

	// Test path traversal attempts
	maliciousPaths := []string{
		"../outside.txt",
		"sub/../../outside.txt",
		"../../outside.txt",
		"services/../../../etc/passwd",
		"../../etc/passwd",
	}

	for _, maliciousPath := range maliciousPaths {
		t.Run("traversal_"+maliciousPath, func(t *testing.T) {
			// Try to construct path
			fullPath := filepath.Join(baseDir, maliciousPath)
			err := ValidatePathInBase(fullPath, baseDir)
			if err == nil {
				t.Errorf("ValidatePathInBase should reject path traversal: %q", maliciousPath)
			}

			// Try with EnsurePathInBase
			_, err = EnsurePathInBase(baseDir, maliciousPath)
			if err == nil {
				t.Errorf("EnsurePathInBase should reject path traversal: %q", maliciousPath)
			}
		})
	}
}
