package naming

import "testing"

func TestLabels_WithWorkspace(t *testing.T) {
	got := Labels("acme", "e-commerce", "api", KindService)

	if got[LabelManaged] != "true" {
		t.Errorf("LabelManaged = %q, want true", got[LabelManaged])
	}
	if got[LabelWorkspace] != "acme" {
		t.Errorf("LabelWorkspace = %q, want acme", got[LabelWorkspace])
	}
	if got[LabelProject] != "e-commerce" {
		t.Errorf("LabelProject = %q, want e-commerce", got[LabelProject])
	}
	if got[LabelService] != "api" {
		t.Errorf("LabelService = %q, want api", got[LabelService])
	}
	if got[LabelKind] != KindService {
		t.Errorf("LabelKind = %q, want %s", got[LabelKind], KindService)
	}
}

func TestLabels_NoWorkspace(t *testing.T) {
	got := Labels("", "solo", "api", KindService)

	if _, ok := got[LabelWorkspace]; ok {
		t.Error("LabelWorkspace should be omitted when workspace is empty")
	}
	if got[LabelProject] != "solo" {
		t.Errorf("LabelProject = %q, want solo", got[LabelProject])
	}
}

func TestLabels_NoService(t *testing.T) {
	got := Labels("acme", "proj", "", KindProxy)

	if _, ok := got[LabelService]; ok {
		t.Error("LabelService should be omitted when service is empty")
	}
	if got[LabelKind] != KindProxy {
		t.Errorf("LabelKind = %q, want %s", got[LabelKind], KindProxy)
	}
}

func TestWorkspaceName_Default(t *testing.T) {
	SetPrefix("")
	defer SetPrefix("")

	if got := WorkspaceName(); got != "" {
		t.Errorf("expected empty WorkspaceName with default prefix, got %q", got)
	}
}

func TestWorkspaceName_Custom(t *testing.T) {
	SetPrefix("acme")
	defer SetPrefix("")

	if got := WorkspaceName(); got != "acme" {
		t.Errorf("expected acme, got %q", got)
	}
}

func TestDepContainer_NameOverride(t *testing.T) {
	SetPrefix("acme")
	defer SetPrefix("")

	got := DepContainer("proj", "postgres", "my-postgres")
	if got != "my-postgres" {
		t.Errorf("expected literal override, got %q", got)
	}
}

func TestDepContainer_WorkspaceShared(t *testing.T) {
	SetPrefix("acme")
	defer SetPrefix("")

	got := DepContainer("proj", "postgres", "")
	if got != "acme-postgres" {
		t.Errorf("expected workspace-shared acme-postgres, got %q", got)
	}
}

func TestDepContainer_NoWorkspaceFallsBackToPerProject(t *testing.T) {
	SetPrefix("")
	defer SetPrefix("")

	got := DepContainer("proj", "postgres", "")
	if got != "raioz-proj-postgres" {
		t.Errorf("expected per-project fallback raioz-proj-postgres, got %q", got)
	}
}
