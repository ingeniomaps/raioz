package naming

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContainer(t *testing.T) {
	SetPrefix("raioz")
	defer SetPrefix("")

	got := Container("myapp", "api")
	if got != "raioz-myapp-api" {
		t.Errorf("expected raioz-myapp-api, got %s", got)
	}
}

func TestContainer_CustomPrefix(t *testing.T) {
	SetPrefix("acme")
	defer SetPrefix("")

	got := Container("ecommerce", "frontend")
	if got != "acme-ecommerce-frontend" {
		t.Errorf("expected acme-ecommerce-frontend, got %s", got)
	}
}

func TestProxyContainer(t *testing.T) {
	SetPrefix("raioz")
	defer SetPrefix("")

	got := ProxyContainer("myapp")
	if got != "raioz-proxy-myapp" {
		t.Errorf("expected raioz-proxy-myapp, got %s", got)
	}
}

func TestProxyContainer_CustomPrefix(t *testing.T) {
	SetPrefix("acme")
	defer SetPrefix("")

	got := ProxyContainer("shop")
	if got != "acme-proxy-shop" {
		t.Errorf("expected acme-proxy-shop, got %s", got)
	}
}

func TestCaddyVolume(t *testing.T) {
	SetPrefix("raioz")
	defer SetPrefix("")

	got := CaddyVolume("myapp")
	if got != "raioz-caddy-myapp" {
		t.Errorf("expected raioz-caddy-myapp, got %s", got)
	}
}

func TestSetPrefix_Empty(t *testing.T) {
	SetPrefix("custom")
	SetPrefix("")
	if GetPrefix() != DefaultPrefix {
		t.Errorf("expected default prefix after empty SetPrefix, got %s", GetPrefix())
	}
}

func TestLogFile(t *testing.T) {
	SetPrefix("raioz")
	defer SetPrefix("")
	home := t.TempDir()
	t.Setenv("RAIOZ_HOME", home)

	want := filepath.Join(home, "logs", "myapp", "api.log")
	if got := LogFile("myapp", "api"); got != want {
		t.Errorf("LogFile() = %s, want %s", got, want)
	}
}

// Host logs live under the state dir, never under /tmp: a tmpfs wipe on
// reboot deletes exactly the failed startup the dev came to read. Pins the
// property so a future move back to TempDir has to argue with a test.
func TestLogDirIsNotTemp(t *testing.T) {
	SetPrefix("raioz")
	defer SetPrefix("")
	home := t.TempDir()
	t.Setenv("RAIOZ_HOME", home)

	got := LogDir("myapp")
	if !strings.HasPrefix(got, RaiozStateDir()) {
		t.Errorf("LogDir() = %s, want it under RaiozStateDir() %s", got, RaiozStateDir())
	}
	if legacy := filepath.Join(TempDir("myapp"), "logs"); got == legacy {
		t.Errorf("LogDir() still resolves to the per-project temp dir: %s", got)
	}
}

func TestDepComposePath(t *testing.T) {
	SetPrefix("raioz")
	defer SetPrefix("")

	got := DepComposePath("myapp", "postgres")
	if !strings.Contains(got, "raioz-myapp") || !strings.HasSuffix(got, "docker-compose.yml") {
		t.Errorf("unexpected compose path: %s", got)
	}
}

func TestContainerPrefix(t *testing.T) {
	SetPrefix("acme")
	defer SetPrefix("")

	got := ContainerPrefix("shop")
	if got != "acme-shop-" {
		t.Errorf("expected acme-shop-, got %s", got)
	}
}

func TestTempDir_Isolated(t *testing.T) {
	SetPrefix("raioz")
	defer SetPrefix("")

	dir1 := TempDir("project-a")
	dir2 := TempDir("project-b")
	if dir1 == dir2 {
		t.Error("temp dirs should be different per project")
	}
	if !strings.Contains(dir1, "raioz-project-a") {
		t.Errorf("expected project name in temp dir, got %s", dir1)
	}
}

func TestPrefix_EnvVar(t *testing.T) {
	// Reset to test env var
	old := os.Getenv("RAIOZ_RUNTIME")
	defer os.Setenv("RAIOZ_RUNTIME", old)

	// SetPrefix is what matters, env var is for runtime package
	SetPrefix("test-org")
	if GetPrefix() != "test-org" {
		t.Errorf("expected test-org, got %s", GetPrefix())
	}
	SetPrefix("")
}
