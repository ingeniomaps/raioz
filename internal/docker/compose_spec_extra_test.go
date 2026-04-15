package docker

import (
	"context"
	"reflect"
	"testing"
)

func TestWithComposeExtraEnv(t *testing.T) {
	t.Run("nil map returns same context", func(t *testing.T) {
		ctx := context.Background()
		if WithComposeExtraEnv(ctx, nil) != ctx {
			t.Error("nil map should return original context")
		}
	})

	t.Run("empty map returns same context", func(t *testing.T) {
		ctx := context.Background()
		if WithComposeExtraEnv(ctx, map[string]string{}) != ctx {
			t.Error("empty map should return original context")
		}
	})

	t.Run("env pairs sorted by key", func(t *testing.T) {
		ctx := WithComposeExtraEnv(context.Background(), map[string]string{
			"ZZZ": "last",
			"AAA": "first",
			"MMM": "middle",
		})
		got := composeExtraEnvFromContext(ctx)
		want := []string{"AAA=first", "MMM=middle", "ZZZ=last"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("missing key returns nil", func(t *testing.T) {
		if got := composeExtraEnvFromContext(context.Background()); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
}

func TestWithComposeEnvFiles(t *testing.T) {
	t.Run("nil slice returns same context", func(t *testing.T) {
		ctx := context.Background()
		if WithComposeEnvFiles(ctx, nil) != ctx {
			t.Error("nil slice should return original context")
		}
	})

	t.Run("files round-trip via context", func(t *testing.T) {
		files := []string{"/a.env", "/b.env"}
		ctx := WithComposeEnvFiles(context.Background(), files)
		got := ComposeEnvFilesFromContext(ctx)
		if !reflect.DeepEqual(got, files) {
			t.Errorf("got %v, want %v", got, files)
		}
	})

	t.Run("stored slice is decoupled from caller's", func(t *testing.T) {
		files := []string{"/a.env"}
		ctx := WithComposeEnvFiles(context.Background(), files)
		files[0] = "/mutated.env"
		got := ComposeEnvFilesFromContext(ctx)
		if got[0] != "/a.env" {
			t.Errorf("expected stored slice unchanged, got %v", got)
		}
	})

	t.Run("missing key returns nil", func(t *testing.T) {
		if got := ComposeEnvFilesFromContext(context.Background()); got != nil {
			t.Errorf("got %v, want nil", got)
		}
	})
}

func TestComposeEnvFileArgs(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		want  []string
	}{
		{name: "no files", files: nil, want: nil},
		{
			name:  "single file",
			files: []string{"/a.env"},
			want:  []string{"--env-file", "/a.env"},
		},
		{
			name:  "multiple files preserve order",
			files: []string{"/a.env", "/b.env"},
			want:  []string{"--env-file", "/a.env", "--env-file", "/b.env"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := WithComposeEnvFiles(context.Background(), tt.files)
			got := ComposeEnvFileArgs(ctx)
			if len(tt.want) == 0 && len(got) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComposeCommandEnv(t *testing.T) {
	original := osEnviron
	defer func() { osEnviron = original }()
	osEnviron = func() []string { return []string{"PATH=/usr/bin"} }

	ctx := WithComposeProjectName(context.Background(), "demo")
	ctx = WithComposeExtraEnv(ctx, map[string]string{"FOO": "bar"})

	got := composeCommandEnv(ctx)
	want := []string{"PATH=/usr/bin", "COMPOSE_PROJECT_NAME=demo", "FOO=bar"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDefaultOsEnviron(t *testing.T) {
	got := defaultOsEnviron()
	if len(got) == 0 {
		t.Error("defaultOsEnviron returned empty slice — process should always have env")
	}
}
