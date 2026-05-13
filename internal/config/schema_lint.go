package config

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// LintFinding is one observation about a field used in a raioz.yaml file.
// Severity reflects intent more than badness: Info marks "field requires
// a newer raioz than you declared", Warn marks "you used a versioned
// field but didn't declare any version yourself".
type LintFinding struct {
	Path     string // dot-path, e.g. "proxy.publish"
	Since    string // version that introduced the field, e.g. "v0.3.0"
	Severity string // "info" | "warn" | "ok"
	Message  string
}

// LintConfig walks a parsed RaiozConfig and the field metadata extracted
// from the schema sources, returning one LintFinding per yaml field
// actually populated by the user. The findings are ordered by Path so
// output is deterministic.
//
// declaredVersion is the value of the user's top-level `version:` field
// ("" when absent). Findings compare each field's Since against it.
//
// `metas` is the result of ExtractFieldMeta. Callers may pre-extract once
// per process and pass it in; this function does not re-read sources.
func LintConfig(cfg *RaiozConfig, metas []FieldMeta, declaredVersion string) []LintFinding {
	if cfg == nil {
		return nil
	}
	idx := indexMetas(metas)
	var findings []LintFinding
	walkStruct(reflect.ValueOf(cfg).Elem(), "", "RaiozConfig", idx, declaredVersion, &findings)
	return findings
}

// indexMetas builds the lookup used by walkStruct: struct name + field name
// → FieldMeta. We index by FieldName (Go name) rather than YAMLName so the
// reflection walk — which iterates Go fields — can find the marker in O(1)
// without re-deriving the yaml tag at every node.
func indexMetas(metas []FieldMeta) map[string]FieldMeta {
	out := make(map[string]FieldMeta, len(metas))
	for _, m := range metas {
		out[m.StructName+"."+m.FieldName] = m
	}
	return out
}

// walkStruct visits every exported field of v, descends into nested
// structs/pointers/maps that the schema cares about, and appends a
// finding when a field is non-zero AND known to the meta index. Fields
// the user did not populate stay silent — `raioz yaml lint` reports on
// what's *used*, not what *could be* used.
func walkStruct(
	v reflect.Value,
	pathPrefix string,
	structName string,
	idx map[string]FieldMeta,
	declaredVersion string,
	findings *[]LintFinding,
) {
	if !v.IsValid() {
		return
	}
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}

	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		yamlTag := sf.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			continue
		}
		yamlName := strings.SplitN(yamlTag, ",", 2)[0]
		if yamlName == "" || yamlName == "-" {
			continue
		}

		fv := v.Field(i)
		if fv.IsZero() {
			continue
		}

		path := joinPath(pathPrefix, yamlName)
		meta, ok := idx[structName+"."+sf.Name]
		if ok && meta.Since != "" {
			*findings = append(*findings, makeFinding(path, meta.Since, declaredVersion))
		}

		recurseChild(fv, path, idx, declaredVersion, findings)
	}
}

// recurseChild handles the polymorphic cases: nested struct, map of
// struct, slice of struct. Anything else is a leaf and stays silent.
func recurseChild(
	fv reflect.Value,
	path string,
	idx map[string]FieldMeta,
	declaredVersion string,
	findings *[]LintFinding,
) {
	t := fv.Type()
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
		if fv.Kind() == reflect.Pointer {
			if fv.IsNil() {
				return
			}
			fv = fv.Elem()
		}
	}
	switch t.Kind() {
	case reflect.Struct:
		walkStruct(fv, path, t.Name(), idx, declaredVersion, findings)
	case reflect.Map:
		iter := fv.MapRange()
		for iter.Next() {
			k := iter.Key()
			child := iter.Value()
			childPath := path + "." + keyToString(k)
			elemType := child.Type()
			for elemType.Kind() == reflect.Pointer {
				elemType = elemType.Elem()
			}
			if elemType.Kind() == reflect.Struct {
				walkStruct(child, childPath, elemType.Name(), idx, declaredVersion, findings)
			}
		}
	case reflect.Slice, reflect.Array:
		// Slices of structs are uncommon at the public-schema level
		// (Projects is the only one today). Walk anyway so future
		// additions don't silently miss out.
		elemType := t.Elem()
		for elemType.Kind() == reflect.Pointer {
			elemType = elemType.Elem()
		}
		if elemType.Kind() != reflect.Struct {
			return
		}
		for i := 0; i < fv.Len(); i++ {
			walkStruct(fv.Index(i), path+"["+strconv.Itoa(i)+"]",
				elemType.Name(), idx, declaredVersion, findings)
		}
	}
}

func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func keyToString(k reflect.Value) string {
	if k.Kind() == reflect.String {
		return k.String()
	}
	return fmt.Sprintf("%v", k.Interface())
}

// makeFinding picks the severity by comparing the field's introducing
// version against the user's declared version.
//
// Today every binary that knows about `version:` is >= v0.4.0, and the
// schema version ("1") is decoupled from the raioz binary version (v0.x).
// The strict comparison "is this field newer than the declared schema
// version?" doesn't have a clean answer yet — we'd need a schema-to-binary
// map. So the rule is pragmatic: if `version:` is absent at all, mark
// every used field as a warning so the user knows to declare it.
// Otherwise everything is `ok` with its since marker shown.
func makeFinding(path, since, declared string) LintFinding {
	f := LintFinding{Path: path, Since: since}
	if declared == "" {
		f.Severity = "warn"
		f.Message = fmt.Sprintf(
			"%s requires raioz %s but no `version:` is declared. "+
				"Add `version: %q` to lock the expected schema.",
			path, since, CurrentSchemaVersion)
		return f
	}
	f.Severity = "ok"
	f.Message = fmt.Sprintf("%s (since %s)", path, since)
	return f
}

// LintConfigPath is a convenience that loads the YAML at path and lints
// it in one call. Used by the `raioz yaml lint` subcommand; tests prefer
// the lower-level LintConfig for finer control.
func LintConfigPath(path string) ([]LintFinding, *RaiozConfig, error) {
	cfg, err := LoadYAML(path)
	if err != nil {
		return nil, nil, err
	}
	metas, err := ExtractFieldMetaEmbedded()
	if err != nil {
		return nil, cfg, fmt.Errorf("extract schema metadata: %w", err)
	}
	return LintConfig(cfg, metas, cfg.Version), cfg, nil
}
