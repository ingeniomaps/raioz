package graph

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// RenderASCII writes an ASCII representation of the graph.
func RenderASCII(g *Graph, w io.Writer) {
	fmt.Fprintf(w, "\n  %s\n\n", g.Project)

	// Group: services first, then dependencies
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

	// Render services with their dependencies
	for _, name := range services {
		node := g.Nodes[name]
		line := "  " + name
		if len(node.Edges) > 0 {
			line += " ──> " + strings.Join(node.Edges, ", ")
		}
		fmt.Fprintln(w, line)
	}

	if len(deps) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "  Dependencies:")
		for _, name := range deps {
			fmt.Fprintf(w, "    [%s]\n", name)
		}
	}

	fmt.Fprintln(w)
}
