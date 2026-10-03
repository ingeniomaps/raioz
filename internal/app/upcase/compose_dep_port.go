package upcase

import (
	"context"
	"os"
	"strconv"
	"strings"

	"raioz/internal/domain/models"

	"gopkg.in/yaml.v3"
)

// composeDepService is the little of a compose service the port lookup
// reads.
type composeDepService struct {
	Image  string `yaml:"image"`
	Expose []any  `yaml:"expose"`
	Ports  []any  `yaml:"ports"`
}

// composeDepPort works out the port a `compose:` dependency listens on
// inside the Docker network, and the image it runs, from its own compose
// files. The service is the one named like the dependency, or the only one
// in the file; with several and no match there is no telling which the
// dependency "is", and the answer is 0.
//
// The port is the first `expose:`, else the container side of the first
// `ports:` entry, else what the image itself exposes. Values that need
// compose interpolation (`${PORT}`) are skipped rather than guessed.
func composeDepPort(ctx context.Context, files []string, depName string) (port int, image string) {
	svc, ok := composeDepServiceFor(files, depName)
	if !ok {
		return 0, ""
	}

	for _, e := range svc.Expose {
		if p := containerPortOf(e); p > 0 {
			return p, svc.Image
		}
	}
	for _, p := range svc.Ports {
		if cp := containerPortOf(p); cp > 0 {
			return cp, svc.Image
		}
	}
	if svc.Image != "" && !strings.Contains(svc.Image, "$") {
		if p, err := imageExposedPortFn(ctx, svc.Image); err == nil && p > 0 {
			return p, svc.Image
		}
	}
	return 0, svc.Image
}

// containerPortOf reads the container side of a compose `ports:` / `expose:`
// entry: an int, "5432", "5433:5432", "127.0.0.1:5433:5432/tcp", or the
// long form `{target: 5432}`.
func containerPortOf(entry any) int {
	switch v := entry.(type) {
	case int:
		return v
	case string:
		spec := strings.Split(v, "/")[0]
		parts := strings.Split(spec, ":")
		port, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil {
			return 0
		}
		return port
	case map[string]any:
		if target, ok := v["target"].(int); ok {
			return target
		}
	}
	return 0
}

// composeDepServiceFor returns the compose service a `compose:` dependency
// stands for: the one named like it, or the only one in its files.
func composeDepServiceFor(files []string, depName string) (composeDepService, bool) {
	services := map[string]composeDepService{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var doc struct {
			Services map[string]composeDepService `yaml:"services"`
		}
		if yaml.Unmarshal(data, &doc) != nil {
			continue
		}
		for name, svc := range doc.Services {
			services[name] = svc
		}
	}
	if svc, ok := services[depName]; ok {
		return svc, true
	}
	if len(services) == 1 {
		for _, only := range services {
			return only, true
		}
	}
	return composeDepService{}, false
}

// DependencyImage returns the image a dependency runs: the declared
// `image:`, or for a `compose:` dependency the one its compose file names.
// Empty when it cannot be told. It is what decides whether the dependency
// speaks HTTP and deserves a proxy route.
func DependencyImage(name string, inline *models.Infra) string {
	if inline == nil {
		return ""
	}
	if inline.Image != "" {
		return inline.Image
	}
	if len(inline.Compose) > 0 {
		if svc, ok := composeDepServiceFor(inline.Compose, name); ok {
			return svc.Image
		}
	}
	return ""
}
