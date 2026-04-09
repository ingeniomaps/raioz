package watch

import (
	"path/filepath"
	"strings"
)

// Resolver maps a changed file path to the service that owns it.
type Resolver struct {
	// servicePaths maps service name → absolute path prefix
	servicePaths map[string]string
}

// NewResolver creates a Resolver from a map of service names to their directories.
func NewResolver(servicePaths map[string]string) *Resolver {
	// Normalize all paths to absolute
	normalized := make(map[string]string, len(servicePaths))
	for name, path := range servicePaths {
		abs, err := filepath.Abs(path)
		if err != nil {
			abs = path
		}
		normalized[name] = abs
	}
	return &Resolver{servicePaths: normalized}
}

// Resolve returns the service name that owns the changed file, or empty string.
func (r *Resolver) Resolve(changedPath string) string {
	abs, err := filepath.Abs(changedPath)
	if err != nil {
		abs = changedPath
	}

	// Find the longest prefix match (most specific service)
	bestMatch := ""
	bestLen := 0
	for name, svcPath := range r.servicePaths {
		if strings.HasPrefix(abs, svcPath) && len(svcPath) > bestLen {
			bestMatch = name
			bestLen = len(svcPath)
		}
	}
	return bestMatch
}
