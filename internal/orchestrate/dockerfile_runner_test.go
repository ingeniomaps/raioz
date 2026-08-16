package orchestrate

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/naming"
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
		Detection: models.DetectResult{
			Runtime:    models.RuntimeDockerfile,
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

// A service with `env:` must reach the container as --env-file, and the file
// flag must precede the -e flags so the discovery vars raioz computes win over
// a stale value in the file.
func TestDockerfileRunner_Start_EnvFile(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	dir := t.TempDir()
	argsFile := dir + "/args.txt"
	script := dir + "/fake-docker.sh"
	if err := writeExecutable(script, "#!/bin/sh\necho \"$@\" >> "+argsFile+"\n"); err != nil {
		t.Fatalf("writeExecutable: %v", err)
	}
	withRuntimeBinary(t, script)

	r := &DockerfileRunner{}
	svc := makeDockerfileSvc(t)
	svc.EnvVars = map[string]string{"FOO": "bar"}
	svc.EnvFilePaths = []string{"/abs/path/.env"}

	if err := r.Start(context.Background(), svc); err != nil {
		t.Fatalf("Start: %v", err)
	}

	out, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "--env-file /abs/path/.env") {
		t.Errorf("expected --env-file flag in docker run args; got: %q", got)
	}
	if ef, ev := strings.Index(got, "--env-file"), strings.Index(got, "-e FOO=bar"); ef < 0 || ev < 0 || ef > ev {
		t.Errorf("--env-file must precede -e; env-file=%d -e=%d in %q", ef, ev, got)
	}
}

// fakeDockerReconcile fakes the runtime binary: `inspect` echoes the given
// "state|managed-label" payload ("" makes it fail, i.e. container absent) and
// every invocation appends its argv to the returned args file.
func fakeDockerReconcile(t *testing.T, inspectPayload string) string {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	dir := t.TempDir()
	argsFile := dir + "/args.txt"
	inspectBranch := "exit 1"
	if inspectPayload != "" {
		inspectBranch = "echo '" + inspectPayload + "'"
	}
	script := dir + "/fake-docker.sh"
	body := "#!/bin/sh\n" +
		"echo \"$@\" >> " + argsFile + "\n" +
		"if [ \"$1\" = inspect ]; then " + inspectBranch + "; fi\n" +
		"exit 0\n"
	if err := writeExecutable(script, body); err != nil {
		t.Fatalf("writeExecutable: %v", err)
	}
	withRuntimeBinary(t, script)
	return argsFile
}

func readFakeDockerArgs(t *testing.T, argsFile string) string {
	t.Helper()
	out, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	return string(out)
}

// A stopped container with the same deterministic name used to make
// `docker run --name` fail with exit 125. Start must force-remove it and
// then create the container as usual.
func TestDockerfileRunner_Start_RemovesStoppedContainer(t *testing.T) {
	argsFile := fakeDockerReconcile(t, "exited|true")

	r := &DockerfileRunner{}
	svc := makeDockerfileSvc(t)

	if err := r.Start(context.Background(), svc); err != nil {
		t.Fatalf("Start: %v", err)
	}

	got := readFakeDockerArgs(t, argsFile)
	if !strings.Contains(got, "rm -f raioz-proj-api") {
		t.Errorf("expected stale container removal; got: %q", got)
	}
	if !strings.Contains(got, "run -d --name raioz-proj-api") {
		t.Errorf("expected docker run after reconcile; got: %q", got)
	}
	if rm, run := strings.Index(got, "rm -f"), strings.Index(got, "run -d"); rm < 0 || run < 0 || rm > run {
		t.Errorf("rm must precede run; rm=%d run=%d in %q", rm, run, got)
	}
}

// An already-running raioz container means the service is up: reuse it
// instead of rebuilding and recreating (same contract as ImageRunner.Start).
func TestDockerfileRunner_Start_ReusesRunningContainer(t *testing.T) {
	argsFile := fakeDockerReconcile(t, "running|true")

	r := &DockerfileRunner{}
	svc := makeDockerfileSvc(t)

	if err := r.Start(context.Background(), svc); err != nil {
		t.Fatalf("Start: %v", err)
	}

	got := readFakeDockerArgs(t, argsFile)
	if strings.Contains(got, "rm -f") {
		t.Errorf("running container must not be removed; got: %q", got)
	}
	if strings.Contains(got, "build") || strings.Contains(got, "run -d") {
		t.Errorf("running container must short-circuit build+run; got: %q", got)
	}
}

// A running container without the raioz labels comes from a version that
// created it before ADR-001 was honored here: `down` can never stop it, so
// reusing it would make it immortal. Replace it instead.
func TestDockerfileRunner_Start_ReplacesUnmanagedRunningContainer(t *testing.T) {
	argsFile := fakeDockerReconcile(t, "running|<no value>")

	r := &DockerfileRunner{}
	svc := makeDockerfileSvc(t)

	if err := r.Start(context.Background(), svc); err != nil {
		t.Fatalf("Start: %v", err)
	}

	got := readFakeDockerArgs(t, argsFile)
	if !strings.Contains(got, "rm -f raioz-proj-api") {
		t.Errorf("unlabeled container must be replaced, not reused; got: %q", got)
	}
	if !strings.Contains(got, "run -d --name raioz-proj-api") {
		t.Errorf("expected docker run after replacing; got: %q", got)
	}
}

// Every container this runner creates must carry the raioz identity labels
// (ADR-001) — without them `raioz down` cannot find the container and
// reports success while the service keeps running.
func TestDockerfileRunner_Start_StampsManagedLabels(t *testing.T) {
	argsFile := fakeDockerReconcile(t, "")

	r := &DockerfileRunner{}
	svc := makeDockerfileSvc(t)

	if err := r.Start(context.Background(), svc); err != nil {
		t.Fatalf("Start: %v", err)
	}

	got := readFakeDockerArgs(t, argsFile)
	for key, want := range naming.Labels("", svc.ProjectName, svc.Name, naming.KindService) {
		if !strings.Contains(got, "--label "+key+"="+want) {
			t.Errorf("missing --label %s=%s in docker run args; got: %q", key, want, got)
		}
	}
}

// No container with that name: reconcile is a no-op and Start proceeds
// straight to build + run, the pre-fix behavior.
func TestDockerfileRunner_Start_AbsentContainerBuildsAndRuns(t *testing.T) {
	argsFile := fakeDockerReconcile(t, "")

	r := &DockerfileRunner{}
	svc := makeDockerfileSvc(t)

	if err := r.Start(context.Background(), svc); err != nil {
		t.Fatalf("Start: %v", err)
	}

	got := readFakeDockerArgs(t, argsFile)
	if strings.Contains(got, "rm -f") {
		t.Errorf("absent container must not trigger removal; got: %q", got)
	}
	if !strings.Contains(got, "build -t raioz-api") || !strings.Contains(got, "run -d") {
		t.Errorf("expected build and run; got: %q", got)
	}
}

// A failing `docker rm -f` must abort Start: running the create anyway would
// only surface the same name conflict with a less useful message.
func TestDockerfileRunner_Start_RemoveFailureAborts(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	dir := t.TempDir()
	script := dir + "/fake-docker.sh"
	body := "#!/bin/sh\n" +
		"if [ \"$1\" = inspect ]; then echo exited; exit 0; fi\n" +
		"if [ \"$1\" = rm ]; then echo 'permission denied'; exit 1; fi\n" +
		"exit 0\n"
	if err := writeExecutable(script, body); err != nil {
		t.Fatalf("writeExecutable: %v", err)
	}
	withRuntimeBinary(t, script)

	r := &DockerfileRunner{}
	svc := makeDockerfileSvc(t)

	err := r.Start(context.Background(), svc)
	if err == nil {
		t.Fatal("expected error when removing the stale container fails")
	}
	if !strings.Contains(err.Error(), "docker rm raioz-proj-api") {
		t.Errorf("error should name the failed removal; got: %v", err)
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

// Regression: Logs previously assigned cmd.Stdout from a freshly
// constructed (always-nil) exec.Cmd, so output was silently dropped.
// Capture os.Stdout and assert a marker from the fake docker reaches
// the user.
func TestDockerfileRunner_Logs_WritesToStdout(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	dir := t.TempDir()
	const marker = "raioz-dockerfile-logs-marker"
	script := dir + "/fake-docker.sh"
	if err := writeExecutable(script, "#!/bin/sh\necho "+marker+"\n"); err != nil {
		t.Fatalf("writeExecutable: %v", err)
	}
	withRuntimeBinary(t, script)

	r := &DockerfileRunner{}
	svc := makeDockerfileSvc(t)

	out := captureOrchestrateStdout(t, func() {
		if err := r.Logs(context.Background(), svc, false, 0); err != nil {
			t.Fatalf("Logs: %v", err)
		}
	})
	if !strings.Contains(out, marker) {
		t.Fatalf("Logs stdout missing marker %q; captured: %q", marker, out)
	}
}

// captureOrchestrateStdout reroutes os.Stdout for fn and returns
// whatever fn wrote. Local to this package — host_runner already
// uses the same os.Stdout assignment, so a shared helper would
// add cross-package coupling for one test.
func captureOrchestrateStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fn()
	_ = w.Close()
	<-done
	return buf.String()
}
