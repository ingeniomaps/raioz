package docker

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	rt "raioz/internal/runtime"
)

// writeFakeProbeBinary writes an executable shell script that emits
// `stdout` to stdout, writes its argv (one per line) to argsFile, and
// exits with `exitCode`. Used to drive IsProjectActive without a real
// docker daemon.
func writeFakeProbeBinary(t *testing.T, dir, stdout, argsFile string, exitCode int) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake binary scripts are POSIX-only")
	}
	body := "#!/bin/sh\n" +
		"for a in \"$@\"; do echo \"$a\" >> " + argsFile + "; done\n" +
		"printf '%s' '" + stdout + "'\n" +
		"exit " + itoa(exitCode) + "\n"
	p := filepath.Join(dir, "fakedocker")
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	return p
}

func itoa(i int) string {
	switch i {
	case 0:
		return "0"
	case 1:
		return "1"
	default:
		return "2"
	}
}

func withFakeRuntime(t *testing.T, path string) {
	t.Helper()
	prev := rt.Binary()
	rt.SetBinary(path)
	t.Cleanup(func() { rt.SetBinary(prev) })
}

func readArgs(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read args file: %v", err)
	}
	var out []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// --- behavior --------------------------------------------------------------

func TestIsProjectActive_Active(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	withFakeRuntime(t, writeFakeProbeBinary(t, dir, "hypixo-keycloak\n", argsFile, 0))

	got, err := IsProjectActive(context.Background(), "hypixo", "keycloak")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("expected active=true when docker reports a container")
	}
}

func TestIsProjectActive_Inactive(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	withFakeRuntime(t, writeFakeProbeBinary(t, dir, "", argsFile, 0))

	got, err := IsProjectActive(context.Background(), "hypixo", "keycloak")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected active=false when docker output is empty")
	}
}

func TestIsProjectActive_DockerError(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	withFakeRuntime(t, writeFakeProbeBinary(t, dir, "", argsFile, 1))

	_, err := IsProjectActive(context.Background(), "hypixo", "keycloak")
	if err == nil || !strings.Contains(err.Error(), "docker ps failed") {
		t.Errorf("expected docker ps failure, got %v", err)
	}
}

func TestIsProjectActive_EmptyProject(t *testing.T) {
	_, err := IsProjectActive(context.Background(), "hypixo", "")
	if err == nil || !strings.Contains(err.Error(), "project name") {
		t.Errorf("expected empty-project error, got %v", err)
	}
}

// --- args wiring -----------------------------------------------------------

func TestIsProjectActive_FilterArgs_WithWorkspace(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	withFakeRuntime(t, writeFakeProbeBinary(t, dir, "", argsFile, 0))

	if _, err := IsProjectActive(context.Background(), "hypixo", "keycloak"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := readArgs(t, argsFile)
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"label=com.raioz.managed=true",
		"label=com.raioz.project=keycloak",
		"label=com.raioz.workspace=hypixo",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q\nfull: %s", want, joined)
		}
	}
}

func TestIsProjectActive_FilterArgs_WithoutWorkspace(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	withFakeRuntime(t, writeFakeProbeBinary(t, dir, "", argsFile, 0))

	if _, err := IsProjectActive(context.Background(), "", "keycloak"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := readArgs(t, argsFile)
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "com.raioz.workspace") {
		t.Errorf("workspace filter should be omitted when ws is empty\nfull: %s", joined)
	}
	if !strings.Contains(joined, "com.raioz.project=keycloak") {
		t.Errorf("project filter missing\nfull: %s", joined)
	}
}
