// Package graph builds and renders dependency graphs from raioz config.
package graph

import (
	"raioz/internal/config"
)

// Node represents a service or dependency in the graph.
type Node struct {
	Name    string   `json:"name"`
	Kind    string   `json:"kind"` // "service" or "dependency"
	Edges   []string `json:"dependsOn"`
}

// Graph is an adjacency list of named nodes.
type Graph struct {
	Project string           `json:"project"`
	Nodes   map[string]*Node `json:"nodes"`
}

// Build creates a Graph from a Deps config.
func Build(deps *config.Deps) *Graph {
	g := &Graph{
		Project: deps.Project.Name,
		Nodes:   make(map[string]*Node),
	}

	for name, svc := range deps.Services {
		g.Nodes[name] = &Node{
			Name:  name,
			Kind:  "service",
			Edges: svc.GetDependsOn(),
		}
	}

	for name := range deps.Infra {
		g.Nodes[name] = &Node{
			Name:  name,
			Kind:  "dependency",
			Edges: nil,
		}
	}

	return g
}

// TopologicalSort returns nodes in dependency order (leaves first).
func (g *Graph) TopologicalSort() []string {
	inDegree := make(map[string]int)
	for name := range g.Nodes {
		inDegree[name] = 0
	}

	for _, node := range g.Nodes {
		for _, dep := range node.Edges {
			if _, exists := g.Nodes[dep]; exists {
				inDegree[dep]++ // dep is depended upon
			}
		}
	}

	// Note: we want dependents first for display, so we sort
	// by "most depended on last" (Kahn's on reverse graph)
	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	var ordered []string
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		ordered = append(ordered, name)

		for _, edge := range g.Nodes[name].Edges {
			if _, exists := inDegree[edge]; exists {
				inDegree[edge]--
				if inDegree[edge] == 0 {
					queue = append(queue, edge)
				}
			}
		}
	}

	return ordered
}
