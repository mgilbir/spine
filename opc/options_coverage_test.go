package opc

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Every field of ReaderOptions must have a With* constructor and a value in
// Default.
//
// The first half was violated when MaxEncryptedInputSize existed only as a
// mutable package variable with no per-Reader override, because the options
// were written by reading the struct rather than by asking what the package
// lets you tune. Deriving the field list from the source is what makes this
// durable: a field added tomorrow fails here until it is exposed.

func TestEveryReaderOptionsFieldHasAConstructor(t *testing.T) {
	fields, withs := readerOptionsSurface(t)

	if len(fields) == 0 {
		t.Fatal("found no ReaderOptions fields; the parse is broken, not the code clean")
	}
	for _, f := range fields {
		want := "With" + f
		if !withs[want] {
			t.Errorf("ReaderOptions.%s has no %s constructor.\n"+
				"\tThe struct and the functional options are two forms of the same\n"+
				"\tconfiguration; a field reachable through only one of them gets\n"+
				"\ttested through only one of them.", f, want)
		}
	}
	for w := range withs {
		// WithReaderOptions replaces the whole configuration rather than one
		// field, so it has no counterpart to match.
		if w == "WithReaderOptions" {
			continue
		}
		field := strings.TrimPrefix(w, "With")
		found := false
		for _, f := range fields {
			if f == field {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s does not correspond to any ReaderOptions field", w)
		}
	}
}

// Every field must also be set by Default, or it silently resolves to the zero
// value — which for these bounds means "disabled". A field added without a
// default would quietly turn a limit off for every caller.
func TestEveryReaderOptionsFieldHasADefault(t *testing.T) {
	def := DefaultReaderOptions()
	zero := ReaderOptions{}
	if def == zero {
		t.Fatal("Default() returns the zero value, so every bound is disabled")
	}
	if def.MaxDecompressedPartSize <= 0 || def.MaxDecompressedPackageSize <= 0 ||
		def.MaxPackageEntries <= 0 || def.MaxNestingDepth <= 0 || def.MaxEncryptedInputSize <= 0 {
		t.Errorf("Default() leaves a bound disabled: %+v", def)
	}
	if def.AllowMissingDataIntegrity {
		t.Error("Default() must require integrity verification (C361)")
	}
}

// readerOptionsSurface returns the exported field names of ReaderOptions and
// the set of With* constructors, both read from the package source.
func readerOptionsSurface(t *testing.T) ([]string, map[string]bool) {
	t.Helper()
	var fields []string
	withs := map[string]bool{}

	for _, file := range parsePackage(t) {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.TypeSpec:
				if node.Name.Name != "ReaderOptions" {
					return true
				}
				st, ok := node.Type.(*ast.StructType)
				if !ok {
					return true
				}
				for _, f := range st.Fields.List {
					for _, name := range f.Names {
						if name.IsExported() {
							fields = append(fields, name.Name)
						}
					}
				}
			case *ast.FuncDecl:
				if node.Recv != nil || !strings.HasPrefix(node.Name.Name, "With") {
					return true
				}
				// Only count funcs that actually return a ReaderOption.
				if node.Type.Results == nil || len(node.Type.Results.List) != 1 {
					return true
				}
				if id, ok := node.Type.Results.List[0].Type.(*ast.Ident); ok && id.Name == "ReaderOption" {
					withs[node.Name.Name] = true
				}
			}
			return true
		})
	}
	sort.Strings(fields)
	return fields, withs
}

func parsePackage(t *testing.T) []*ast.File {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading package dir: %v", err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(".", name), nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		files = append(files, f)
	}
	return files
}
