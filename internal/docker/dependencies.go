package docker

import (
	"fmt"

	"raioz/internal/config"
)

// ValidateDependencyCycle checks for circular dependencies in services
// Returns an error if a cycle is detected, with a clear message
func ValidateDependencyCycle(deps *config.Deps) error {
	// Build adjacency list: service -> list of dependencies
	graph := make(map[string][]string)

	// Add all services to graph
	for name := range deps.Services {
		graph[name] = []string{}
	}

	// Add dependencies to graph (service-level and docker-level)
	for name, svc := range deps.Services {
		for _, dep := range svc.GetDependsOn() {
			graph[name] = append(graph[name], dep)
		}
	}

	// Check for cycles using DFS
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var cycle []string

	var dfs func(node string, path []string) bool
	dfs = func(node string, path []string) bool {
		// Check if node is in recursion stack (cycle detected)
		if recStack[node] {
			// Found a cycle, build cycle path
			cycle = buildCyclePath(path, node)
			return true
		}

		// If already visited in a different path, skip
		if visited[node] {
			return false
		}

		// Mark as visited and add to recursion stack
		visited[node] = true
		recStack[node] = true
		path = append(path, node)

		// Visit all dependencies
		for _, dep := range graph[node] {
			if dfs(dep, path) {
				return true
			}
		}

		// Remove from recursion stack
		recStack[node] = false
		return false
	}

	// Check all services
	for name := range graph {
		if !visited[name] {
			if dfs(name, []string{}) {
				return fmt.Errorf(
					"circular dependency detected: %s",
					formatCycle(cycle),
				)
			}
		}
	}

	return nil
}

// buildCyclePath builds the cycle path from the DFS path and the node that closes the cycle
func buildCyclePath(path []string, cycleNode string) []string {
	// Find where the cycle starts
	cycleStart := -1
	for i, node := range path {
		if node == cycleNode {
			cycleStart = i
			break
		}
	}

	if cycleStart == -1 {
		return append(path, cycleNode)
	}

	// Build cycle: from cycleStart to end, plus cycleNode
	cycle := path[cycleStart:]
	cycle = append(cycle, cycleNode)

	return cycle
}

// formatCycle formats a cycle path into a readable string
func formatCycle(cycle []string) string {
	if len(cycle) == 0 {
		return "unknown cycle"
	}

	if len(cycle) == 1 {
		return fmt.Sprintf("%s -> %s (self-dependency)", cycle[0], cycle[0])
	}

	result := cycle[0]
	for i := 1; i < len(cycle); i++ {
		result += " -> " + cycle[i]
	}

	return result
}

// GetAllServiceNames returns all service and infra names that can be dependencies
func GetAllServiceNames(deps *config.Deps) map[string]bool {
	names := make(map[string]bool)

	for name := range deps.Services {
		names[name] = true
	}

	for name := range deps.Infra {
		names[name] = true
	}

	return names
}
