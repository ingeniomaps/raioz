package config

import (
	"strconv"
	"strings"
	"testing"
)

// TestSchemaSinceMarkers is the load-bearing linter behind `make
// check-since`. It walks every public schema struct (see SchemaFiles)
// and fails if any yaml-tagged field is missing a `// since: vX.Y.Z`
// marker. The marker is the only signal callers and `raioz yaml lint`
// have for "when was this field introduced", so a missing marker
// quietly breaks the field-evolution policy documented in
// docs/CONFIG_REFERENCE.md.
//
// Add a field → add the marker; the test fails if you forget, with
// the struct name and source line to fix.
func TestSchemaSinceMarkers(t *testing.T) {
	metas, err := ExtractFieldMeta("../..")
	if err != nil {
		t.Fatalf("ExtractFieldMeta: %v", err)
	}

	if len(metas) == 0 {
		t.Fatal("ExtractFieldMeta returned zero fields — the parser is " +
			"misconfigured or SchemaFiles is empty")
	}

	var missing []string
	for _, m := range metas {
		if m.Since == "" {
			missing = append(missing, m.StructName+"."+m.FieldName+
				" ("+m.File+":"+strconv.Itoa(m.Line)+")")
		}
	}

	if len(missing) > 0 {
		t.Errorf("%d schema field(s) missing `// since: vX.Y.Z` marker:\n  %s\n\n"+
			"Add the marker after the struct tag, e.g.:\n"+
			"  Publish *bool `yaml:\"publish,omitempty\"` // since: v0.3.0\n\n"+
			"See docs/CONFIG_REFERENCE.md#field-evolution-policy.",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// TestSchemaRemovalRequiresPriorDeprecation enforces ADR-045's
// "deprecation window before removal" discipline. A field that
// gets a `// removed: vX.Y.Z` marker without first carrying a
// `// deprecated: vX.Y.Z` would hard-error at load without ever
// warning the user — exactly the silent breakage the ADR exists
// to prevent.
func TestSchemaRemovalRequiresPriorDeprecation(t *testing.T) {
	metas, err := ExtractFieldMeta("../..")
	if err != nil {
		t.Fatalf("ExtractFieldMeta: %v", err)
	}
	violations := ValidateRemoval(metas)
	if len(violations) == 0 {
		return
	}
	msgs := make([]string, 0, len(violations))
	for _, v := range violations {
		msgs = append(msgs, "  "+v.Error())
	}
	t.Errorf(
		"%d schema field(s) declare `// removed:` without a prior "+
			"`// deprecated:` marker (ADR-045 deprecation-window rule):\n%s",
		len(violations), strings.Join(msgs, "\n"),
	)
}

// TestSchemaSinceFormat catches markers that parsed but use a shape the
// rest of the tooling can't compare semantically (e.g., "0.3.0" without
// the leading v, or "v1" without minor/patch).
func TestSchemaSinceFormat(t *testing.T) {
	metas, err := ExtractFieldMeta("../..")
	if err != nil {
		t.Fatalf("ExtractFieldMeta: %v", err)
	}

	for _, m := range metas {
		if m.Since == "" {
			continue
		}
		if !strings.HasPrefix(m.Since, "v") {
			t.Errorf("%s.%s: since=%q must start with 'v' (e.g. v0.3.0)",
				m.StructName, m.FieldName, m.Since)
		}
		parts := strings.SplitN(strings.TrimPrefix(m.Since, "v"), ".", 3)
		if len(parts) != 3 {
			t.Errorf("%s.%s: since=%q must have three components (v0.3.0)",
				m.StructName, m.FieldName, m.Since)
		}
	}
}
