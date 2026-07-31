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

// Every field of ReaderOptions must have a With* constructor, and every bound
// must have a value in DefaultReaderOptions.
//
// The first half was violated when MaxEncryptedInputSize existed only as a
// mutable package variable with no per-Reader override, because the options
// were written by reading the struct rather than by asking what the package
// lets you tune. Deriving the field list from the source is what makes this
// durable: a field added tomorrow fails here until it is exposed.
//
// Unexported fields count too. The password is one — it is held as a *string
// out of fmt's reach — and a field the options cannot set would be dead weight
// whichever case its name is in.

func TestEveryReaderOptionsFieldHasAConstructor(t *testing.T) {
	fields, withs := readerOptionsSurface(t)

	if len(fields) == 0 {
		t.Fatal("found no ReaderOptions fields; the parse is broken, not the code clean")
	}
	for _, f := range fields {
		want := "With" + strings.ToUpper(f[:1]) + f[1:]
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
			if strings.EqualFold(f, field) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s does not correspond to any ReaderOptions field", w)
		}
	}
}

// Every bound must also be set by DefaultReaderOptions, or it silently resolves
// to the zero value — which for these means "disabled". A field added without a
// default would quietly turn a limit off for every caller.
func TestEveryReaderOptionsFieldHasADefault(t *testing.T) {
	def := DefaultReaderOptions()
	zero := ReaderOptions{}
	if def == zero {
		t.Fatal("DefaultReaderOptions returns the zero value, so every bound is disabled")
	}
	if def.MaxDecompressedPartSize <= 0 || def.MaxDecompressedPackageSize <= 0 ||
		def.MaxPackageEntries <= 0 || def.MaxNestingDepth <= 0 || def.MaxEncryptedInputSize <= 0 {
		t.Errorf("DefaultReaderOptions leaves a bound disabled: %+v", def)
	}
	if def.AllowMissingDataIntegrity {
		t.Error("DefaultReaderOptions must require integrity verification (C361)")
	}
	// The password is the one field whose default must stay empty, and it is
	// exempt from the rule above rather than overlooked by it: every other
	// field is a bound whose zero value weakens the reader, while a default
	// password would mean an encrypted package opened with a secret the caller
	// never supplied. Absent, an encrypted input reports ErrEncrypted.
	if def.password != nil {
		t.Error("DefaultReaderOptions carries a password; it must be supplied per open, never defaulted")
	}
}

// readerOptionsSurface returns the field names of ReaderOptions (exported and
// not) and the set of With* constructors, both read from the package source.
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
					// Unexported fields included: the password is one, and a
					// knob is a knob whichever case its name is in.
					for _, name := range f.Names {
						fields = append(fields, name.Name)
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
