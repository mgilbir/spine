package xmlguard_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// entrypointSuppression exempts one bare parse, and must carry a reason.
const entrypointSuppression = "//xmlguard:lenient"

// TestPartsAreParsedThroughTheCheckedEntryPoint keeps the choice of parser a
// decision rather than a habit.
//
// A part read off a package has to be well-formed to its end and has to bind
// every prefix it uses. encoding/xml enforces neither: Decode stops at the root
// element's end tag and never looks at what follows, and an unbound prefix
// resolves to the prefix itself rather than failing, so the name quietly means
// something other than what it says. xmlb.Unmarshal applies both checks.
//
// Which formats got the checks was an accident of when each parser was written.
// docx and pptx largely had them; xlsx had six checked parses against fifteen
// bare ones, so the format with the most parse sites had the least checking, and
// nobody decided that. The audit that found it also found legacy comment parts
// in pptx and the chart and diagram parts on the same footing.
//
// Not every parse is a part. Reconstructed elements, synthesized wrappers and
// marshal-then-unmarshal deep copies are all legitimately lenient, and the
// part-level rule would be wrong for them — an element lifted out of a document
// resolves its prefixes against a root that is not in the bytes being parsed.
// Those carry `//xmlguard:lenient <reason>`, which is the point: the exemption
// is where the decision is, and it has to be argued.
func TestPartsAreParsedThroughTheCheckedEntryPoint(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()

	var findings []string
	files := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			// common/xml is the package that implements the checked entry point,
			// so it necessarily calls the standard library underneath.
			if skipDirs[rel] || (strings.HasPrefix(rel, ".") && rel != ".") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		files++
		f, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			return nil
		}
		findings = append(findings, bareParses(fset, f, filepath.ToSlash(rel))...)
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
	if files < 50 {
		t.Fatalf("only %d source files scanned; the walk is broken, not the tree", files)
	}

	for _, f := range findings {
		t.Errorf("%s\n\tthis parses with encoding/xml, which does not require the input to "+
			"be well-formed to its end or to bind the prefixes it uses. If it reads a part, "+
			"use xmlb.Unmarshal (or xmlb.UnmarshalWithSource). If it does not, mark it "+
			"`%s <reason>`.", f, entrypointSuppression)
	}
}

// bareParses reports calls to encoding/xml's Unmarshal that carry no exemption.
func bareParses(fset *token.FileSet, f *ast.File, rel string) []string {
	exempt := markedLines(fset, f, entrypointSuppression)
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Unmarshal" {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		// The standard library, under whichever name it is imported. xmlb is
		// the checked entry point and is what this rule asks for.
		if pkg.Name != "xml" && pkg.Name != "stdxml" {
			return true
		}
		line := fset.Position(call.Pos()).Line
		if exempt[line] {
			return true
		}
		out = append(out, rel+":"+strconv.Itoa(line))
		return true
	})
	return out
}
