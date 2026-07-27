package pptx

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// The presentation part is the one relationship scope in this package with two
// claimants that cannot see each other: the relationships already registered in
// p.relationships[presentationPartName], and p.nextRelID, which holds ids handed
// to slides by AddSlide whose relationships are only materialized at save time.
//
// C363 shipped because four import paths allocated from the first claimant alone
// and AddSlide from the second alone, so a merge handed one id to both a slide
// and an imported master; the save path kept the master's relationship and
// dropped the slide's as a duplicate, leaving p:sldId entries resolving to
// slideMaster and notesMaster relationships in a package Validate() called
// clean.
//
// Fixing the five call sites is not enough — the next presentation-level
// relationship type added would reintroduce it (audit tension T-E). These two
// guards make the wrong pattern unavailable instead: nothing may allocate a
// presentation rel id without going through nextPresentationRelID, and nothing
// outside the allocator may touch p.nextRelID.

// parsePackageFiles parses every non-test .go file of the pptx package.
func parsePackageFiles(t *testing.T) (*token.FileSet, map[string]*ast.File) {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read pptx dir: %v", err)
	}
	fset := token.NewFileSet()
	files := make(map[string]*ast.File)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files[name] = f
	}
	if len(files) == 0 {
		t.Fatal("pptx package parsed to zero files")
	}
	return fset, files
}

// callFunSelectors returns the set of selector expressions used as the callee of
// a call — i.e. method calls, which must not be confused with field reads of the
// same name (Slide.nextRelID is a method; Presentation.nextRelID is a field).
func callFunSelectors(n ast.Node) map[*ast.SelectorExpr]bool {
	out := map[*ast.SelectorExpr]bool{}
	ast.Inspect(n, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				out[sel] = true
			}
		}
		return true
	})
	return out
}

// isPresentationRelsExpr reports whether expr is an index into a relationships
// map keyed by the presentation part.
func isPresentationRelsExpr(expr ast.Expr) bool {
	ix, ok := expr.(*ast.IndexExpr)
	if !ok {
		return false
	}
	sel, ok := ix.X.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "relationships" {
		return false
	}
	switch k := ix.Index.(type) {
	case *ast.Ident:
		return k.Name == "presentationPartName" || k.Name == "presMainPartName"
	case *ast.BasicLit:
		return k.Value == `"/ppt/presentation.xml"`
	}
	return false
}

// TestNoBlindPresentationRelIDAllocation fails when any non-test source in the
// package hands the scope-blind nextRelationshipID helper the presentation
// part's relationship slice — directly or through a local variable. Such a call
// cannot see the ids AddSlide has already reserved in p.nextRelID (C363).
func TestNoBlindPresentationRelIDAllocation(t *testing.T) {
	fset, files := parsePackageFiles(t)
	for name, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				return true
			}
			// Locals bound to the presentation part's relationship slice.
			tainted := make(map[string]bool)
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				var lhs, rhs []ast.Expr
				switch s := n.(type) {
				case *ast.AssignStmt:
					lhs, rhs = s.Lhs, s.Rhs
				case *ast.ValueSpec:
					for _, id := range s.Names {
						lhs = append(lhs, id)
					}
					rhs = s.Values
				default:
					return true
				}
				for i := range lhs {
					if i >= len(rhs) {
						break
					}
					id, ok := lhs[i].(*ast.Ident)
					if !ok {
						continue
					}
					// Also taint through an append(presRels, ...) copy.
					src := rhs[i]
					if call, ok := src.(*ast.CallExpr); ok {
						if fnID, ok := call.Fun.(*ast.Ident); ok && fnID.Name == "append" && len(call.Args) > 0 {
							src = call.Args[0]
						}
					}
					if isPresentationRelsExpr(src) {
						tainted[id.Name] = true
					} else if inner, ok := src.(*ast.Ident); ok && tainted[inner.Name] {
						tainted[id.Name] = true
					}
				}
				return true
			})

			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				id, ok := call.Fun.(*ast.Ident)
				if !ok || id.Name != "nextRelationshipID" || len(call.Args) != 1 {
					return true
				}
				arg := call.Args[0]
				bad := isPresentationRelsExpr(arg)
				if ident, ok := arg.(*ast.Ident); ok && tainted[ident.Name] {
					bad = true
				}
				if bad {
					t.Errorf("%s: %s allocates a presentation relationship id with the scope-blind nextRelationshipID; "+
						"use p.nextPresentationRelID / p.addPresentationRel, which also consult p.nextRelID (C363)",
						fset.Position(call.Pos()), fn.Name.Name)
				}
				return true
			})
			return true
		})
		_ = name
	}
}

// TestNextRelIDConfinedToAllocator fails when any function other than the
// allocator and its open-time seeder reads or writes p.nextRelID. Every historic
// instance of C363 on the AddSlide side was a bare `rId%d, p.nextRelID` followed
// by `p.nextRelID++`, which reserves the id against the counter but not against
// the relationships already registered on the presentation part.
func TestNextRelIDConfinedToAllocator(t *testing.T) {
	allowed := map[string]bool{
		"nextPresentationRelID": true, // the single allocator
		"updateNextRelID":       true, // seeds the counter from parsed ids at open
	}
	fset, files := parsePackageFiles(t)
	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Body == nil || allowed[fn.Name.Name] {
				return true
			}
			methodCalls := callFunSelectors(fn.Body)
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "nextRelID" || methodCalls[sel] {
					return true
				}
				// A composite-literal field name is not a selector, so struct
				// initialization (nextRelID: 1) never reaches here.
				t.Errorf("%s: %s touches p.nextRelID directly; presentation relationship ids must be "+
					"allocated through p.nextPresentationRelID / p.addPresentationRel (C363)",
					fset.Position(sel.Pos()), fn.Name.Name)
				return true
			})
			return true
		})
	}
}
