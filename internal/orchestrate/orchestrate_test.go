package orchestrate

import (
	"testing"

	"raioz/internal/detect"
)

func TestSelectRunner_Compose(t *testing.T) {
	d := &Dispatcher{
		compose:    &ComposeRunner{},
		dockerfile: &DockerfileRunner{},
		host:       &HostRunner{},
		image:      &ImageRunner{},
	}

	runner, err := d.selectRunner(detect.RuntimeCompose)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := runner.(*ComposeRunner); !ok {
		t.Error("expected ComposeRunner")
	}
}

func TestSelectRunner_Dockerfile(t *testing.T) {
	d := &Dispatcher{
		compose:    &ComposeRunner{},
		dockerfile: &DockerfileRunner{},
		host:       &HostRunner{},
		image:      &ImageRunner{},
	}

	runner, err := d.selectRunner(detect.RuntimeDockerfile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := runner.(*DockerfileRunner); !ok {
		t.Error("expected DockerfileRunner")
	}
}

func TestSelectRunner_HostRuntimes(t *testing.T) {
	d := &Dispatcher{
		compose:    &ComposeRunner{},
		dockerfile: &DockerfileRunner{},
		host:       &HostRunner{},
		image:      &ImageRunner{},
	}

	hostRuntimes := []detect.Runtime{
		detect.RuntimeNPM,
		detect.RuntimeGo,
		detect.RuntimeMake,
		detect.RuntimePython,
		detect.RuntimeRust,
	}

	for _, rt := range hostRuntimes {
		runner, err := d.selectRunner(rt)
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", rt, err)
		}
		if _, ok := runner.(*HostRunner); !ok {
			t.Errorf("expected HostRunner for %s", rt)
		}
	}
}

func TestSelectRunner_Image(t *testing.T) {
	d := &Dispatcher{
		compose:    &ComposeRunner{},
		dockerfile: &DockerfileRunner{},
		host:       &HostRunner{},
		image:      &ImageRunner{},
	}

	runner, err := d.selectRunner(detect.RuntimeImage)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := runner.(*ImageRunner); !ok {
		t.Error("expected ImageRunner")
	}
}

func TestSelectRunner_Unknown(t *testing.T) {
	d := &Dispatcher{
		compose:    &ComposeRunner{},
		dockerfile: &DockerfileRunner{},
		host:       &HostRunner{},
		image:      &ImageRunner{},
	}

	_, err := d.selectRunner(detect.RuntimeUnknown)
	if err == nil {
		t.Error("expected error for unknown runtime")
	}
}

func TestHostRunner_GetSetPID(t *testing.T) {
	r := &HostRunner{}

	if pid := r.GetPID("api"); pid != 0 {
		t.Errorf("expected 0, got %d", pid)
	}

	r.SetPID("api", 12345)

	if pid := r.GetPID("api"); pid != 12345 {
		t.Errorf("expected 12345, got %d", pid)
	}
}
