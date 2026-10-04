package orchestrate

import (
	"context"
	"strconv"

	"raioz/internal/docker"
	"raioz/internal/domain/interfaces"
)

// imageExposedPort reads the port an image declares with EXPOSE. Package
// var so tests answer without a daemon.
var imageExposedPort = docker.GetImageExposedPort

// declaredPortArgs publishes a Dockerfile service's `port:` on the host.
// The service said "I am on this port": without the mapping the container
// only listens inside the Docker network, and a project with no proxy has
// no way to reach it. It binds loopback, like the proxy does, and maps to
// the port the image exposes — the declared number when the image exposes
// none. Explicit `ports:` mappings are the user's own and are left alone.
func declaredPortArgs(ctx context.Context, svc interfaces.ServiceContext, image string) []string {
	if svc.HostPort <= 0 || len(svc.Ports) > 0 {
		return nil
	}
	containerPort := svc.HostPort
	if exposed, err := imageExposedPort(ctx, image); err == nil && exposed > 0 {
		containerPort = exposed
	}
	return []string{"-p", "127.0.0.1:" + strconv.Itoa(svc.HostPort) + ":" + strconv.Itoa(containerPort)}
}
