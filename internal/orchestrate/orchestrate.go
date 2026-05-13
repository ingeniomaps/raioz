// Package orchestrate dispatches service lifecycle operations to the correct
// runner based on the detected runtime (compose, dockerfile, host, image).
package orchestrate

import (
	"context"
	"fmt"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
)

// Dispatcher routes service operations to the correct runner.
type Dispatcher struct {
	compose    *ComposeRunner
	dockerfile *DockerfileRunner
	host       *HostRunner
	image      *ImageRunner
}

// NewDispatcher creates a Dispatcher with the given docker runner.
func NewDispatcher(docker interfaces.DockerRunner) *Dispatcher {
	return &Dispatcher{
		compose:    &ComposeRunner{docker: docker},
		dockerfile: &DockerfileRunner{},
		host:       &HostRunner{},
		image:      &ImageRunner{docker: docker},
	}
}

// Start starts a service using the appropriate runner for its runtime.
func (d *Dispatcher) Start(ctx context.Context, svc interfaces.ServiceContext) error {
	runner, err := d.selectRunner(svc.Detection.Runtime)
	if err != nil {
		return fmt.Errorf("service '%s': %w", svc.Name, err)
	}
	return runner.Start(ctx, svc)
}

// Stop stops a service using the appropriate runner for its runtime.
func (d *Dispatcher) Stop(ctx context.Context, svc interfaces.ServiceContext) error {
	runner, err := d.selectRunner(svc.Detection.Runtime)
	if err != nil {
		return fmt.Errorf("service '%s': %w", svc.Name, err)
	}
	return runner.Stop(ctx, svc)
}

// Restart restarts a service.
func (d *Dispatcher) Restart(ctx context.Context, svc interfaces.ServiceContext) error {
	runner, err := d.selectRunner(svc.Detection.Runtime)
	if err != nil {
		return fmt.Errorf("service '%s': %w", svc.Name, err)
	}
	return runner.Restart(ctx, svc)
}

// Status returns the status of a service.
func (d *Dispatcher) Status(ctx context.Context, svc interfaces.ServiceContext) (string, error) {
	runner, err := d.selectRunner(svc.Detection.Runtime)
	if err != nil {
		return "unknown", fmt.Errorf("service '%s': %w", svc.Name, err)
	}
	return runner.Status(ctx, svc)
}

// Logs streams logs from a service.
func (d *Dispatcher) Logs(ctx context.Context, svc interfaces.ServiceContext, follow bool, tail int) error {
	runner, err := d.selectRunner(svc.Detection.Runtime)
	if err != nil {
		return fmt.Errorf("service '%s': %w", svc.Name, err)
	}
	return runner.Logs(ctx, svc, follow, tail)
}

// runner is the internal interface that each runtime-specific runner implements.
type runner interface {
	Start(ctx context.Context, svc interfaces.ServiceContext) error
	Stop(ctx context.Context, svc interfaces.ServiceContext) error
	Restart(ctx context.Context, svc interfaces.ServiceContext) error
	Status(ctx context.Context, svc interfaces.ServiceContext) (string, error)
	Logs(ctx context.Context, svc interfaces.ServiceContext, follow bool, tail int) error
}

// GetHostPID returns the PID of a host service, or 0 if not tracked.
func (d *Dispatcher) GetHostPID(serviceName string) int {
	return d.host.GetPID(serviceName)
}

func (d *Dispatcher) selectRunner(runtime models.Runtime) (runner, error) {
	switch runtime {
	case models.RuntimeCompose:
		return d.compose, nil
	case models.RuntimeDockerfile:
		return d.dockerfile, nil
	case models.RuntimeNPM, models.RuntimeGo, models.RuntimeMake,
		models.RuntimeJust, models.RuntimeTask,
		models.RuntimePython, models.RuntimeRust, models.RuntimePHP,
		models.RuntimeJava, models.RuntimeDotnet, models.RuntimeRuby,
		models.RuntimeElixir, models.RuntimeDart, models.RuntimeSwift,
		models.RuntimeScala, models.RuntimeClojure, models.RuntimeZig,
		models.RuntimeGleam, models.RuntimeHaskell, models.RuntimeDeno,
		models.RuntimeBun:
		return d.host, nil
	case models.RuntimeImage:
		return d.image, nil
	default:
		return nil, fmt.Errorf("unsupported runtime: %s", runtime)
	}
}
