package docker

import (
	"context"
	"testing"
)

func TestWithComposeProjectName(t *testing.T) {
	tests := []struct {
		name     string
		projName string
		wantSet  bool
	}{
		{"empty name returns same ctx", "", false},
		{"non-empty name sets value", "my-project", true},
		{"spaces are valid", "my project", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parent := context.Background()
			got := WithComposeProjectName(parent, tt.projName)

			if !tt.wantSet {
				// Should return the same context
				if got != parent {
					t.Error("expected same context for empty name")
				}
				return
			}

			// Verify the value is set by extracting it
			env := composeProjectEnvFromContext(got)
			want := "COMPOSE_PROJECT_NAME=" + tt.projName
			if env != want {
				t.Errorf("env = %q, want %q", env, want)
			}
		})
	}
}

func TestComposeProjectEnvFromContext(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want string
	}{
		{
			"no value in context",
			context.Background(),
			"",
		},
		{
			"with project name",
			WithComposeProjectName(context.Background(), "test-proj"),
			"COMPOSE_PROJECT_NAME=test-proj",
		},
		{
			"with empty string set via raw context value",
			context.WithValue(context.Background(), composeProjectNameKey{}, ""),
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := composeProjectEnvFromContext(tt.ctx)
			if got != tt.want {
				t.Errorf("composeProjectEnvFromContext() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJoinComposePaths(t *testing.T) {
	tests := []struct {
		name  string
		paths []string
		want  string
	}{
		{"single path", []string{"/a/b.yml"}, "/a/b.yml"},
		{"two paths", []string{"/a/b.yml", "/c/d.yml"}, "/a/b.yml:/c/d.yml"},
		{"three paths", []string{"a", "b", "c"}, "a:b:c"},
		{"empty slice", []string{}, ""},
		{"empty string element", []string{""}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := JoinComposePaths(tt.paths)
			if got != tt.want {
				t.Errorf("JoinComposePaths(%v) = %q, want %q", tt.paths, got, tt.want)
			}
		})
	}
}

func TestComposeFileArgs(t *testing.T) {
	tests := []struct {
		name        string
		composePath string
		want        []string
	}{
		{
			"single file",
			"/app/compose.yml",
			[]string{"-f", "/app/compose.yml"},
		},
		{
			"two files",
			"/app/compose.yml:/app/overlay.yml",
			[]string{"-f", "/app/compose.yml", "-f", "/app/overlay.yml"},
		},
		{
			"three files",
			"a.yml:b.yml:c.yml",
			[]string{"-f", "a.yml", "-f", "b.yml", "-f", "c.yml"},
		},
		{
			"empty segments dropped",
			"a.yml::b.yml",
			[]string{"-f", "a.yml", "-f", "b.yml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComposeFileArgs(tt.composePath)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d: %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("arg[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestJoinAndSplitRoundtrip(t *testing.T) {
	paths := []string{"/a/compose.yml", "/b/overlay.yml", "/c/extra.yml"}
	joined := JoinComposePaths(paths)
	split := SplitComposePaths(joined)

	if len(split) != len(paths) {
		t.Fatalf("roundtrip: len = %d, want %d", len(split), len(paths))
	}
	for i := range paths {
		if split[i] != paths[i] {
			t.Errorf("roundtrip[%d] = %q, want %q", i, split[i], paths[i])
		}
	}
}

func TestPrimaryComposeFile_MultiFile(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"single", "/a/b.yml", "/a/b.yml"},
		{"multi", "/a/b.yml:/c/d.yml", "/a/b.yml"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PrimaryComposeFile(tt.path)
			if got != tt.want {
				t.Errorf("PrimaryComposeFile(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
