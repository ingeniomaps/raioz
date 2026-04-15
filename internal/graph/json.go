package graph

import (
	"encoding/json"
	"io"
)

// JSONOutput is the JSON representation of the graph.
type JSONOutput struct {
	Project string     `json:"project"`
	Nodes   []JSONNode `json:"nodes"`
	Edges   []JSONEdge `json:"edges"`
}

// JSONNode is a node in the JSON output.
type JSONNode struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	DependsOn []string `json:"dependsOn"`
}

// JSONEdge is an edge in the JSON output.
type JSONEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// RenderJSON writes a JSON representation of the graph.
func RenderJSON(g *Graph, w io.Writer) error {
	out := JSONOutput{Project: g.Project}

	for _, node := range g.Nodes {
		out.Nodes = append(out.Nodes, JSONNode{
			Name:      node.Name,
			Kind:      node.Kind,
			DependsOn: node.Edges,
		})
		for _, edge := range node.Edges {
			out.Edges = append(out.Edges, JSONEdge{From: node.Name, To: edge})
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
