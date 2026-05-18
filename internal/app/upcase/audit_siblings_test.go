package upcase

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/domain/models"
)

// writeAuditSiblingYAML writes a minimal raioz.yaml at <root>/<dirName>/raioz.yaml.
// Returns the absolute directory path the consumer would set as `project:`.
func writeAuditSiblingYAML(t *testing.T, root, dirName, body string) string {
	t.Helper()
	dir := filepath.Join(root, dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "raioz.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// Transitive walk visits sibling-of-sibling — the v0.10.0 expansion.
// Two hops: consumer → A → B. Both yamls audited.
func TestAuditSiblings_TransitiveTwoHops(t *testing.T) {
	root := t.TempDir()
	// B is the deepest: just an image dep, no nested siblings.
	bDir := writeAuditSiblingYAML(t, root, "b", ""+
		"workspace: w\n"+
		"project: b\n"+
		"services:\n"+
		"  b:\n"+
		"    path: .\n"+
		"dependencies:\n"+
		"  pg:\n"+
		"    image: postgres:16\n")

	// A declares B as a sibling-dep.
	aDir := writeAuditSiblingYAML(t, root, "a", ""+
		"workspace: w\n"+
		"project: a\n"+
		"services:\n"+
		"  a:\n"+
		"    path: .\n"+
		"dependencies:\n"+
		"  b:\n"+
		"    project: "+bDir+"\n")

	// Consumer deps include A directly.
	deps := &models.Deps{
		Infra: map[string]models.InfraEntry{
			"a": {Inline: &models.Infra{Project: aDir}},
		},
	}

	if err := auditSiblingYAMLs(deps); err != nil {
		t.Fatalf("audit failed unexpectedly: %v", err)
	}
}

// Nested unpinned-image fails the audit. Confirms transitive depth
// actually checks H3 on the nested yaml.
func TestAuditSiblings_NestedUnpinnedFails(t *testing.T) {
	root := t.TempDir()
	// B carries the violation: image without a tag.
	bDir := writeAuditSiblingYAML(t, root, "b", ""+
		"workspace: w\n"+
		"project: b\n"+
		"services:\n"+
		"  b:\n"+
		"    path: .\n"+
		"dependencies:\n"+
		"  pg:\n"+
		"    image: postgres\n") // no tag — H3 fail

	aDir := writeAuditSiblingYAML(t, root, "a", ""+
		"workspace: w\n"+
		"project: a\n"+
		"services:\n"+
		"  a:\n"+
		"    path: .\n"+
		"dependencies:\n"+
		"  b:\n"+
		"    project: "+bDir+"\n")

	deps := &models.Deps{
		Infra: map[string]models.InfraEntry{
			"a": {Inline: &models.Infra{Project: aDir}},
		},
	}

	err := auditSiblingYAMLs(deps)
	if err == nil {
		t.Fatal("expected audit failure on nested unpinned image")
	}
	if !strings.Contains(err.Error(), "dep \"a\" → dep \"b\"") {
		t.Errorf("error must trace the chain to the nested dep; got %q", err.Error())
	}
}

// A cycle (A depends on B depends on A) must not loop forever. The
// visited-set keyed by absolute yaml path is the loop guard.
func TestAuditSiblings_CyclesTerminate(t *testing.T) {
	root := t.TempDir()
	aDir := filepath.Join(root, "a")
	bDir := filepath.Join(root, "b")
	if err := os.MkdirAll(aDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(bDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(aDir, "raioz.yaml"), []byte(""+
		"workspace: w\n"+
		"project: a\n"+
		"services:\n"+
		"  a:\n"+
		"    path: .\n"+
		"dependencies:\n"+
		"  b:\n"+
		"    project: "+bDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bDir, "raioz.yaml"), []byte(""+
		"workspace: w\n"+
		"project: b\n"+
		"services:\n"+
		"  b:\n"+
		"    path: .\n"+
		"dependencies:\n"+
		"  a:\n"+
		"    project: "+aDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := &models.Deps{
		Infra: map[string]models.InfraEntry{
			"a": {Inline: &models.Infra{Project: aDir}},
		},
	}

	// If the visited set is broken this hangs / blows the stack.
	// Wrapping in a goroutine with a timeout would be heavier than
	// needed — if this returns at all, the loop guard works.
	if err := auditSiblingYAMLs(deps); err != nil {
		t.Fatalf("cyclic audit must terminate cleanly; got %v", err)
	}
}

// No sibling deps at all → no-op success.
func TestAuditSiblings_NoSiblings(t *testing.T) {
	deps := &models.Deps{
		Infra: map[string]models.InfraEntry{
			"pg": {Inline: &models.Infra{Image: "postgres", Tag: "16"}},
		},
	}
	if err := auditSiblingYAMLs(deps); err != nil {
		t.Errorf("audit with no siblings should succeed; got %v", err)
	}
}
