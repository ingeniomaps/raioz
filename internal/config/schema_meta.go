package config

import (
	_ "embed"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

// embeddedYAMLTypes carries the source of yaml_types.go into the runtime
// binary so the `raioz yaml lint` subcommand can extract `since:` markers
// without having access to the source tree. The build-time linter test
// reads from disk via SchemaFiles instead — both paths reuse parseSource.
//
//go:embed yaml_types.go
var embeddedYAMLTypes []byte

// FieldMeta describes one yaml-tagged struct field together with the raioz
// version in which it was introduced. Produced by ExtractFieldMeta at test
// time (lint) and at runtime (yaml lint subcommand). The version string is
// taken verbatim from a `// since: vX.Y.Z` trailing comment next to the
// field declaration in the source.
type FieldMeta struct {
	// StructName is the Go type that declares the field, e.g. "RaiozConfig",
	// "YAMLService", "ProxyConfig".
	StructName string
	// FieldName is the Go field name, e.g. "Publish".
	FieldName string
	// YAMLName is the YAML key the field marshals to, e.g. "publish".
	YAMLName string
	// Since is the introducing version (e.g. "v0.3.0"), or empty if the
	// field has no marker.
	Since string
	// Deprecated is the version at which the field started warning at
	// load (e.g. "v0.9.0"), or empty when not yet deprecated.
	// Pairs with Replacement for the suggested migration target.
	Deprecated string
	// Removed is the version at which the field hard-errors at load
	// (e.g. "v1.0.0"), or empty when still accepted. ValidateRemoval
	// requires a prior Deprecated marker so the user always sees a
	// warning window.
	Removed string
	// Replacement names the field/feature operators should migrate to.
	// Optional but recommended for deprecated/removed entries.
	Replacement string
	// File is the source file containing the declaration. Useful for
	// "missing marker" error messages.
	File string
	// Line is the 1-indexed line in File where the field is declared.
	Line int
}

// Recognized marker forms. All tolerant of leading whitespace and
// trailing prose so the comment can carry both a marker AND a human
// note: `// since: v0.3.0 — added by issue #N`. Earlier
// `deprecated:`, `removed:`, and `replacement:`.
var (
	sinceRe       = regexp.MustCompile(`since:\s*(v\d+\.\d+\.\d+)`)
	deprecatedRe  = regexp.MustCompile(`deprecated:\s*(v\d+\.\d+\.\d+)`)
	removedRe     = regexp.MustCompile(`removed:\s*(v\d+\.\d+\.\d+)`)
	replacementRe = regexp.MustCompile(`replacement:\s*(\S+)`)
)

// SchemaFiles is the curated list of source files that declare the public
// `raioz.yaml` schema. The parser only inspects these files — adding a new
// schema type without adding it here means the linter won't enforce
// `since:` on its fields, which is exactly the failure mode we're trying
// to prevent. Keep this list authoritative.
var SchemaFiles = []string{
	"internal/config/yaml_types.go",
	"internal/domain/models/config_proxy.go",
}

// ExtractFieldMeta walks the AST of every file in SchemaFiles and returns
// FieldMeta entries for each yaml-tagged exported field. repoRoot is the
// repository root so the function works from any working directory; pass
// "" to resolve relative to the caller's cwd (test mode).
func ExtractFieldMeta(repoRoot string) ([]FieldMeta, error) {
	var out []FieldMeta
	fset := token.NewFileSet()
	for _, rel := range SchemaFiles {
		path := rel
		if repoRoot != "" {
			path = filepath.Join(repoRoot, rel)
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", rel, err)
		}
		out = append(out, fieldsFromFile(file, rel, fset)...)
	}
	sortMetas(out)
	return out, nil
}

// ExtractFieldMetaEmbedded parses the source files embedded into the
// binary. Used at runtime (raioz yaml lint) when the user's cwd has no
// access to the raioz source tree. Currently only yaml_types.go is
// embedded — fields declared in internal/domain/models/ (ProxyConfig,
// RoutingConfig) will not appear here, which means `raioz yaml lint`
// silently skips them. Acceptable today because those fields are stable
// and rarely changed; revisit when one of them grows a new variant.
func ExtractFieldMetaEmbedded() ([]FieldMeta, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(
		fset, "yaml_types.go", embeddedYAMLTypes, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse embedded yaml_types.go: %w", err)
	}
	out := fieldsFromFile(file, "yaml_types.go", fset)
	sortMetas(out)
	return out, nil
}

func fieldsFromFile(file *ast.File, rel string, fset *token.FileSet) []FieldMeta {
	var out []FieldMeta
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, field := range st.Fields.List {
				out = append(out, metasForField(ts.Name.Name, field, rel, fset)...)
			}
		}
	}
	return out
}

func sortMetas(metas []FieldMeta) {
	sort.Slice(metas, func(i, j int) bool {
		if metas[i].StructName != metas[j].StructName {
			return metas[i].StructName < metas[j].StructName
		}
		return metas[i].FieldName < metas[j].FieldName
	})
}

// metasForField returns one FieldMeta per field name in a Go declaration
// (`Foo, Bar int` produces two). Skips fields without a yaml tag or with
// `yaml:"-"`.
func metasForField(
	structName string, field *ast.Field, file string, fset *token.FileSet,
) []FieldMeta {
	if field.Tag == nil {
		return nil
	}
	tagValue := strings.Trim(field.Tag.Value, "`")
	st := reflect.StructTag(tagValue)
	yamlTag := st.Get("yaml")
	if yamlTag == "" || yamlTag == "-" {
		return nil
	}
	yamlName := strings.SplitN(yamlTag, ",", 2)[0]
	if yamlName == "" || yamlName == "-" {
		return nil
	}

	markers := extractMarkers(field)
	line := fset.Position(field.Pos()).Line

	out := make([]FieldMeta, 0, len(field.Names))
	for _, name := range field.Names {
		if !name.IsExported() {
			continue
		}
		out = append(out, FieldMeta{
			StructName:  structName,
			FieldName:   name.Name,
			YAMLName:    yamlName,
			Since:       markers.since,
			Deprecated:  markers.deprecated,
			Removed:     markers.removed,
			Replacement: markers.replacement,
			File:        file,
			Line:        line,
		})
	}
	return out
}

type fieldMarkers struct {
	since, deprecated, removed, replacement string
}

// extractMarkers reads the trailing comment of a struct field and the
// optional leading doc block, returning the four recognized markers.
// All markers are independent — a field may declare any subset.
//
// top of the original since: form.
func extractMarkers(field *ast.Field) fieldMarkers {
	var m fieldMarkers
	candidates := []*ast.CommentGroup{field.Comment, field.Doc}
	for _, cg := range candidates {
		if cg == nil {
			continue
		}
		for _, c := range cg.List {
			text := strings.TrimPrefix(c.Text, "//")
			text = strings.TrimPrefix(text, "/*")
			text = strings.TrimSuffix(text, "*/")
			if mt := sinceRe.FindStringSubmatch(text); len(mt) == 2 && m.since == "" {
				m.since = mt[1]
			}
			if mt := deprecatedRe.FindStringSubmatch(text); len(mt) == 2 && m.deprecated == "" {
				m.deprecated = mt[1]
			}
			if mt := removedRe.FindStringSubmatch(text); len(mt) == 2 && m.removed == "" {
				m.removed = mt[1]
			}
			if mt := replacementRe.FindStringSubmatch(text); len(mt) == 2 && m.replacement == "" {
				m.replacement = mt[1]
			}
		}
	}
	return m
}
