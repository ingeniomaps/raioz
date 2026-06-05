package naming

import "testing"

// TestDepComposeProjectNameFor_Issue069 locks in the shared-vs-per-project
// compose scope: a workspace-shared dep drops the project segment so every
// consumer agrees on one project (the last-one-out teardown then matches),
// while a per-project dep keeps the segment to avoid --remove-orphans
// sweeping another project's same-named dep.
func TestDepComposeProjectNameFor_Issue069(t *testing.T) {
	SetPrefix("conorbi")
	defer SetPrefix("")

	if got := DepComposeProjectNameFor("observability", "cache", true); got != "conorbi-dep-cache" {
		t.Errorf("shared: got %q, want conorbi-dep-cache", got)
	}
	if got := SharedDepComposeProjectName("cache"); got != "conorbi-dep-cache" {
		t.Errorf("SharedDepComposeProjectName: got %q, want conorbi-dep-cache", got)
	}
	if got := DepComposeProjectNameFor("observability", "cache", false); got != "conorbi-observability-dep-cache" {
		t.Errorf("per-project: got %q, want conorbi-observability-dep-cache", got)
	}
}
