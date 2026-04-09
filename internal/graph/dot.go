package graph

import (
	"fmt"
	"io"
	"sort"
)

// RenderDOT writes a Graphviz DOT representation of the graph.
func RenderDOT(g *Graph, w io.Writer) {
	fmt.Fprintf(w, "digraph %q {\n", g.Project)
	fmt.Fprintln(w, "    rankdir=LR;")
	fmt.Fprintln(w, "    node [shape=box, style=rounded];")
	fmt.Fprintln(w)

	// Services
	var services, deps []string
	for name, node := range g.Nodes {
		if node.Kind == "service" {
			services = append(services, name)
		} else {
			deps = append(deps, name)
		}
	}
	sort.Strings(services)
	sort.Strings(deps)

	if len(services) > 0 {
		fmt.Fprintln(w, "    // Services")
		for _, name := range services {
			fmt.Fprintf(w, "    %q [label=%q];\n", name, name)
		}
	}

	if len(deps) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "    // Dependencies")
		for _, name := range deps {
			fmt.Fprintf(w, "    %q [label=%q, shape=cylinder];\n", name, name)
		}
	}

	// Edges
	fmt.Fprintln(w)
	fmt.Fprintln(w, "    // Edges")
	allNodes := append(services, deps...)
	sort.Strings(allNodes)
	for _, name := range allNodes {
		node := g.Nodes[name]
		for _, edge := range node.Edges {
			fmt.Fprintf(w, "    %q -> %q;\n", name, edge)
		}
	}

	fmt.Fprintln(w, "}")
}
