package production

import (
	"sort"
	"strings"
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
	if strings.HasPrefix(vol, "./") {
		vol = vol[2:]
	}
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
