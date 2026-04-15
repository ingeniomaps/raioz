package upcase

import (
	"raioz/internal/config"
)

// orderedServiceNames returns service names sorted by dependency order via
// Kahn's algorithm. Dependencies on infra are ignored here — only
// service-to-service edges matter, because infra is already started before
// this ordering runs.
//
// Extracted from orchestration.go to keep that file under the 400-line cap.
func orderedServiceNames(deps *config.Deps) []string {
	// Build adjacency list. graph[A] = [B, C] means "B and C depend on A",
	// so A must start first. inDegree[X] counts how many services must
	// start before X.
	graph := make(map[string][]string)
	inDegree := make(map[string]int)
	allServices := make(map[string]bool)

	for name, svc := range deps.Services {
		allServices[name] = true
		for _, dep := range svc.GetDependsOn() {
			if _, isService := deps.Services[dep]; isService {
				graph[dep] = append(graph[dep], name)
				inDegree[name]++
			}
		}
		if _, exists := inDegree[name]; !exists {
			inDegree[name] = 0
		}
	}

	// Kahn's algorithm: repeatedly pull nodes with no pending prerequisites,
	// emit them, and relax the edges of their dependents.
	var queue []string
	for name := range allServices {
		if inDegree[name] == 0 {
			queue = append(queue, name)
		}
	}

	var ordered []string
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		ordered = append(ordered, name)

		for _, dependent := range graph[name] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	return ordered
}
