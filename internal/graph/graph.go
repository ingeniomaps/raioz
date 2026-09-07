// Package graph builds and renders dependency graphs from raioz config.
package graph

import "raioz/internal/domain/models"

// Node represents a service or dependency in the graph.
type Node struct {
	Name  string   `json:"name"`
	Kind  string   `json:"kind"` // "service" or "dependency"
	Edges []string `json:"dependsOn"`
}

// Graph is an adjacency list of named nodes.
type Graph struct {
	Project string           `json:"project"`
	Nodes   map[string]*Node `json:"nodes"`
}

// Build creates a Graph from a Deps config.
func Build(deps *models.Deps) *Graph {
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
