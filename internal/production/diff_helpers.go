package production

import (
	"sort"
	"strconv"
	"strings"

	"raioz/internal/domain/models"
)

// Helper functions for comparison

func portsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	sort.Strings(a)
	sort.Strings(b)

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func volumesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	sort.Strings(a)
	sort.Strings(b)

	for i := range a {
		if normalizeVolume(a[i]) != normalizeVolume(b[i]) {
			return false
		}
	}

	return true
}

func normalizeVolume(vol string) string {
	// Normalize volume format for comparison
	vol = strings.TrimSpace(vol)
	// Remove leading ./ if present
	vol = strings.TrimPrefix(vol, "./")
	return vol
}

func dependsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	sort.Strings(a)
	sort.Strings(b)

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func isInfraService(name string) bool {
	infraKeywords := []string{"db", "database", "redis", "postgres", "mysql", "mongo", "rabbit", "kafka", "elastic"}
	nameLower := strings.ToLower(name)
	for _, keyword := range infraKeywords {
		if strings.Contains(nameLower, keyword) {
			return true
		}
	}
	return false
}

// localInfraPorts renders a dependency's port mapping in the `host:container`
// form production uses, so the two can be compared.
//
// `ports:` is taken as written. `expose:` + `publish:` — what raioz.yaml
// uses now — is paired up: pinned host ports give `host:container`; with
// `publish: true` raioz picks the host port, so only the container side
// can disagree and the production host port is borrowed for the match.
func localInfraPorts(inf models.Infra, prodPorts []string) []string {
	if len(inf.Ports) > 0 {
		return inf.Ports
	}
	if inf.Publish == nil || len(inf.Expose) == 0 {
		return []string{}
	}
	prodHostFor := map[string]string{}
	for _, mapping := range prodPorts {
		if host, container, ok := strings.Cut(mapping, ":"); ok {
			prodHostFor[container] = host
		}
	}
	ports := make([]string, 0, len(inf.Expose))
	for i, container := range inf.Expose {
		containerPort := strconv.Itoa(container)
		switch {
		case i < len(inf.Publish.Ports):
			ports = append(ports, strconv.Itoa(inf.Publish.Ports[i])+":"+containerPort)
		case inf.Publish.Auto && prodHostFor[containerPort] != "":
			ports = append(ports, prodHostFor[containerPort]+":"+containerPort)
		default:
			ports = append(ports, containerPort)
		}
	}
	return ports
}
