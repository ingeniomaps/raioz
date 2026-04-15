package naming

import "testing"

func TestIsSharedDep_WorkspaceSet(t *testing.T) {
	SetPrefix("acme")
	defer SetPrefix("")

	if !IsSharedDep("") {
		t.Error("workspace=acme, no override → should be shared")
	}
}

func TestIsSharedDep_NoWorkspaceNoOverride(t *testing.T) {
	SetPrefix("")
	defer SetPrefix("")

	if IsSharedDep("") {
		t.Error("no workspace, no override → should NOT be shared")
	}
}

func TestIsSharedDep_ExplicitNameOverride(t *testing.T) {
	SetPrefix("")
	defer SetPrefix("")

	if !IsSharedDep("my-postgres") {
		t.Error("explicit name override → should be shared (user signaled intent)")
	}
}

func TestLabels_OmitsEmptyProject(t *testing.T) {
	got := Labels("acme", "", "postgres", KindDependency)
	if _, ok := got[LabelProject]; ok {
		t.Error("LabelProject should be omitted for shared deps (no project owner)")
	}
	if got[LabelWorkspace] != "acme" {
		t.Errorf("LabelWorkspace should still be set, got %q", got[LabelWorkspace])
	}
	if got[LabelKind] != KindDependency {
		t.Errorf("LabelKind wrong, got %q", got[LabelKind])
	}
}
