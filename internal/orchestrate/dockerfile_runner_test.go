package orchestrate

import (
	"context"
	"os/exec"
	"testing"

	"raioz/internal/detect"
	"raioz/internal/domain/interfaces"
	"raioz/internal/runtime"
)

// withRuntimeBinary swaps runtime.Binary() for the duration of the test,
// restoring the default when t finishes. We use builtin commands like
// "true" / "false" to simulate docker subprocesses without hitting the
// real daemon.
func withRuntimeBinary(t *testing.T, bin string) {
	t.Helper()
	prev := runtime.Binary()
	runtime.SetBinary(bin)
	t.Cleanup(func() { runtime.SetBinary(prev) })
}

func makeDockerfileSvc(t *testing.T) interfaces.ServiceContext {
	t.Helper()
	dir := t.TempDir()
	return interfaces.ServiceContext{
		Name:          "api",
		Path:          dir,
		ProjectName:   "proj",
		ContainerName: "raioz-proj-api",
		NetworkName:   "proj-net",
		Ports:         []string{"8080:8080"},
		EnvVars:       map[string]string{"FOO": "bar"},
		Detection: detect.DetectResult{
			Runtime:    detect.RuntimeDockerfile,
			Dockerfile: "Dockerfile",
		},
	}
}

func TestDockerfileRunner_Start_Success(t *testing.T) {
	if _, err := exec.LookPath("true"); err != nil {
		t.Skip("true not available")
	}
	withRuntimeBinary(t, "true")

	r := &DockerfileRunner{}
	svc := makeDockerfileSvc(t)

	if err := r.Start(context.Background(), svc); err != nil {
		t.Errorf("Start: %v", err)
	}
}

func TestDockerfileRunner_Start_NoPortsNoEnv(t *testing.T) {
	if _, err := exec.LookPath("true"); err != nil {
		t.Skip("true not available")
	}
	withRuntimeBinary(t, "true")

	r := &DockerfileRunner{}
	svc := makeDockerfileSvc(t)
	svc.Ports = nil
	svc.EnvVars = nil

	if err := r.Start(context.Background(), svc); err != nil {
		t.Errorf("Start: %v", err)
	}
}

func TestDockerfileRunner_Start_BuildFails(t *testing.T) {
	if _, err := exec.LookPath("false"); err != nil {
		t.Skip("false not available")
	}
	withRuntimeBinary(t, "false")

	r := &DockerfileRunner{}
	svc := makeDockerfileSvc(t)

	if err := r.Start(context.Background(), svc); err == nil {
		t.Error("expected error when build fails")
	}
}

func TestDockerfileRunner_Stop(t *testing.T) {
	if _, err := exec.LookPath("true"); err != nil {
		t.Skip("true not available")
	}
	withRuntimeBinary(t, "true")

	r := &DockerfileRunner{}
	svc := makeDockerfileSvc(t)

	// Stop never returns error — it ignores subprocess failures by design.
	if err := r.Stop(context.Background(), svc); err != nil {
		t.Errorf("Stop: %v", err)
	}
}

func TestDockerfileRunner_Stop_IgnoresFailures(t *testing.T) {
	if _, err := exec.LookPath("false"); err != nil {
		t.Skip("false not available")
	}
	withRuntimeBinary(t, "false")

	r := &DockerfileRunner{}
	svc := makeDockerfileSvc(t)

	// Even when the docker binary fails, Stop should swallow errors.
	if err := r.Stop(context.Background(), svc); err != nil {
		t.Errorf("Stop should not return error even when binary fails: %v", err)
	}
}

func TestDockerfileRunner_Restart_Success(t *testing.T) {
	if _, err := exec.LookPath("true"); err != nil {
		t.Skip("true not available")
	}
	withRuntimeBinary(t, "true")

	r := &DockerfileRunner{}
	svc := makeDockerfileSvc(t)

	if err := r.Restart(context.Background(), svc); err != nil {
		t.Errorf("Restart: %v", err)
	}
}

func TestDockerfileRunner_Restart_StartFails(t *testing.T) {
	if _, err := exec.LookPath("false"); err != nil {
		t.Skip("false not available")
	}
	withRuntimeBinary(t, "false")

	r := &DockerfileRunner{}
	svc := makeDockerfileSvc(t)

	// Stop swallows the error, but subsequent Start will fail.
	if err := r.Restart(context.Background(), svc); err == nil {
		t.Error("expected error when Start fails during restart")
	}
}

func TestDockerfileRunner_Status_Stopped(t *testing.T) {
	// When docker inspect fails (binary = "false"), Status returns "stopped".
	if _, err := exec.LookPath("false"); err != nil {
		t.Skip("false not available")
	}
	withRuntimeBinary(t, "false")

	r := &DockerfileRunner{}
	svc := makeDockerfileSvc(t)

	status, err := r.Status(context.Background(), svc)
	if err != nil {
		t.Errorf("Status: %v", err)
	}
	if status != "stopped" {
		t.Errorf("expected stopped, got %s", status)
	}
}

func TestDockerfileRunner_Status_Running(t *testing.T) {
	// Use a fake docker binary that prints "running\n" as a shell script.
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	dir := t.TempDir()
	script := dir + "/fake-docker.sh"
	if err := writeExecutable(script, "#!/bin/sh\necho running\n"); err != nil {
		t.Fatalf("writeExecutable: %v", err)
	}
	withRuntimeBinary(t, script)

	r := &DockerfileRunner{}
	svc := makeDockerfileSvc(t)

	status, err := r.Status(context.Background(), svc)
	if err != nil {
		t.Errorf("Status: %v", err)
	}
	if status != "running" {
		t.Errorf("expected running, got %s", status)
	}
}

func TestDockerfileRunner_Status_NotRunningOutput(t *testing.T) {
	// Fake docker that outputs "exited" — should map to "stopped".
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	dir := t.TempDir()
	script := dir + "/fake-docker.sh"
	if err := writeExecutable(script, "#!/bin/sh\necho exited\n"); err != nil {
		t.Fatalf("writeExecutable: %v", err)
	}
	withRuntimeBinary(t, script)

	r := &DockerfileRunner{}
	svc := makeDockerfileSvc(t)

	status, err := r.Status(context.Background(), svc)
	if err != nil {
		t.Errorf("Status: %v", err)
	}
	if status != "stopped" {
		t.Errorf("expected stopped, got %s", status)
	}
}

func TestDockerfileRunner_Logs_Success(t *testing.T) {
	if _, err := exec.LookPath("true"); err != nil {
		t.Skip("true not available")
	}
	withRuntimeBinary(t, "true")

	r := &DockerfileRunner{}
	svc := makeDockerfileSvc(t)

	if err := r.Logs(context.Background(), svc, false, 0); err != nil {
		t.Errorf("Logs: %v", err)
	}
}

func TestDockerfileRunner_Logs_FollowAndTail(t *testing.T) {
	if _, err := exec.LookPath("true"); err != nil {
		t.Skip("true not available")
	}
	withRuntimeBinary(t, "true")

	r := &DockerfileRunner{}
	svc := makeDockerfileSvc(t)

	// Exercise the follow=true and tail>0 branches.
	if err := r.Logs(context.Background(), svc, true, 100); err != nil {
		t.Errorf("Logs: %v", err)
	}
}

func TestDockerfileRunner_Logs_CommandFails(t *testing.T) {
	if _, err := exec.LookPath("false"); err != nil {
		t.Skip("false not available")
	}
	withRuntimeBinary(t, "false")

	r := &DockerfileRunner{}
	svc := makeDockerfileSvc(t)

	if err := r.Logs(context.Background(), svc, false, 0); err == nil {
		t.Error("expected error when logs command fails")
	}
}
