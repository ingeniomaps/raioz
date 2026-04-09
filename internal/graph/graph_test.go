package graph

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"raioz/internal/config"
)

func testDeps() *config.Deps {
	return &config.Deps{
		Project: config.Project{Name: "test-app"},
		Services: map[string]config.Service{
			"api": {
				DependsOn: []string{"postgres", "redis"},
			},
			"frontend": {
				DependsOn: []string{"api"},
			},
			"worker": {
				DependsOn: []string{"postgres"},
			},
		},
		Infra: map[string]config.InfraEntry{
			"postgres": {Inline: &config.Infra{Image: "postgres"}},
			"redis":    {Inline: &config.Infra{Image: "redis"}},
		},
	}
}

func TestBuild(t *testing.T) {
	g := Build(testDeps())

	if g.Project != "test-app" {
		t.Errorf("expected project 'test-app', got '%s'", g.Project)
	}
	if len(g.Nodes) != 5 {
		t.Errorf("expected 5 nodes, got %d", len(g.Nodes))
	}
	if g.Nodes["api"].Kind != "service" {
		t.Error("api should be service")
	}
	if g.Nodes["postgres"].Kind != "dependency" {
		t.Error("postgres should be dependency")
	}
	if len(g.Nodes["api"].Edges) != 2 {
		t.Errorf("api should have 2 edges, got %d", len(g.Nodes["api"].Edges))
	}
}

func TestTopologicalSort(t *testing.T) {
	g := Build(testDeps())
	sorted := g.TopologicalSort()

	if len(sorted) != 5 {
		t.Errorf("expected 5 nodes in sort, got %d", len(sorted))
	}

	// frontend depends on api, so frontend should come before api in our sort
	// (we sort dependents first, leaves last)
	posOf := make(map[string]int)
	for i, name := range sorted {
		posOf[name] = i
	}

	// frontend depends on api → frontend should appear before api
	if posOf["frontend"] > posOf["api"] {
		t.Errorf("frontend should appear before api in topological order, got: %v", sorted)
	}
}

func TestRenderASCII(t *testing.T) {
	g := Build(testDeps())
	var buf bytes.Buffer
	RenderASCII(g, &buf)

	output := buf.String()
	if !strings.Contains(output, "test-app") {
		t.Error("expected project name in ASCII output")
	}
	if !strings.Contains(output, "api") {
		t.Error("expected 'api' in ASCII output")
	}
	if !strings.Contains(output, "Dependencies:") {
		t.Error("expected 'Dependencies:' section")
	}
}

func TestRenderDOT(t *testing.T) {
	g := Build(testDeps())
	var buf bytes.Buffer
	RenderDOT(g, &buf)

	output := buf.String()
	if !strings.Contains(output, "digraph") {
		t.Error("expected 'digraph' in DOT output")
	}
	if !strings.Contains(output, "shape=cylinder") {
		t.Error("expected cylinder shape for dependencies")
	}
	if !strings.Contains(output, "->") {
		t.Error("expected edges in DOT output")
	}
}

func TestRenderJSON(t *testing.T) {
	g := Build(testDeps())
	var buf bytes.Buffer
	if err := RenderJSON(g, &buf); err != nil {
		t.Fatalf("RenderJSON failed: %v", err)
	}

	var out JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if out.Project != "test-app" {
		t.Errorf("expected project 'test-app', got '%s'", out.Project)
	}
	if len(out.Nodes) != 5 {
		t.Errorf("expected 5 nodes, got %d", len(out.Nodes))
	}
	if len(out.Edges) < 4 {
		t.Errorf("expected at least 4 edges, got %d", len(out.Edges))
	}
}
