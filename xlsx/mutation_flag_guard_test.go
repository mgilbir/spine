package xlsx

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

// Tension T2, "mutation-not-flagged", is this codebase's most-repeated bug
// class: a mutator changes durable document state and returns success, but
// nothing records that the owning part must be regenerated, so the edit is
// discarded at save (C423) — or the mirror image, a path that flags when
// nothing changed, so an untouched part is regenerated and the byte-identical
// round-trip is lost (C425, C544).
//
// The guards below are deliberately structural rather than a pinned list of
// today's offenders: they fail when a NEW mutator is added without a flag, not
// only when a known one regresses. Every intentional exception is recorded in
// mutationFlagExempt with the reason it is safe, the way captureExemptAttrs
// does in pptx/internal/oxml — a justified exemption list beats silence.

// flagCalls are the calls that record "durable state changed, regenerate the
// owning part". Reaching any of them (directly or through a callee in this
// package) satisfies the guard.
var flagCalls = map[string]bool{
	"markDirty":         true, // Sheet: regenerate the worksheet part
	"markSheetDirty":    true, // Cell → Sheet.markDirty
	"markCommentsDirty": true, // Sheet: regenerate the comment parts + worksheet
	"markModified":      true, // StyleManager → Workbook.stylesDirty
	"dirtyMark":         true, // sheetComments: regenerate on save
}

// flagFields are the flag variables themselves. Assigning one counts as
// flagging, never as a mutation.
var flagFields = map[string]bool{
	"dirty":           true, // Sheet.dirty
	"stylesDirty":     true, // Workbook.stylesDirty
	"sheetsDirty":     true, // Workbook.sheetsDirty
	"needRelsRebuild": true, // local/derived rels-rebuild flag
	"vbaModified":     true, // Workbook.vbaModified
	"mutated":         true, // sheetComments.mutated
	"modified":        true, // generic "edited since open" marker
}

// modelAccessors return a pointer into a durable model owned by the receiver,
// so an assignment through the returned pointer is a mutation of that model.
var modelAccessors = map[string]bool{
	"ws":              true,
	"ensureWS":        true,
	"ensureWorksheet": true,
	"ensureSheetView": true,
	"ensurePane":      true,
}

// nonDurableFields are receiver fields that hold lazily-parsed caches or
// save-time bookkeeping. Writing one changes nothing that is serialized, so it
// must not be mistaken for a mutation — otherwise every reader that touches
// ws() would be reported.
var nonDurableFields = map[string]bool{
	"wsModel":               true, // lazily parsed worksheet model
	"wsParsed":              true, // lazy-parse memo
	"wsParseErr":            true, // lazy-parse failure memo
	"comments":              true, // lazily loaded comment model
	"sparklineCache":        true, // lazily parsed sparkline groups
	"persons":               true, // lazily loaded threaded-comment authors
	"personsLoaded":         true, // lazy-load memo
	"personsPartName":       true, // resolved on lazy load
	"tablePartsBaseline":    true, // save-time projection baseline (C257)
	"tablePartsBaselineSet": true,
	"reader":                true, // the source zip handle, released by Close
	"themeResolved":         true, // lazily resolved theme editor
	"themePartName":         true, // resolved on lazy load
}

// mutationFlagExempt lists the exported methods that mutate durable state
// without setting a regeneration flag, each with the reason that is correct.
// A method missing from both the flag set and this list fails the guard; an
// entry here that is no longer needed also fails, so the list cannot rot.
var mutationFlagExempt = map[string]string{
	// --- Sheet: the cell grid is materialized on lookup, by design (C425) ---
	"Sheet.Cell": "Cell is a materializing accessor: it creates the <row>/<c> for the " +
		"reference so the returned handle is writable. The phantom entries carry no " +
		"value, formula, inline string or style and are dropped again by the marshaler, " +
		"so a lookup-only call neither reaches the file nor inflates <dimension>. " +
		"Flagging here would dirty a sheet that was only read (C425).",
	"Sheet.CellByRowCol": "Delegates to Sheet.Cell; same reason.",

	// --- Sheet: state that lives in workbook.xml, which is always regenerated ---
	"Sheet.SetName": "Writes the sheet name into the workbook model. workbook.xml is " +
		"regenerated from the parsed model on every save, so the rename persists without " +
		"a sheet flag — and dirtying the sheet part would regenerate a worksheet that did " +
		"not change (C545).",
	"Sheet.SetVisibility": "Writes the workbook-level <sheet state> attribute; workbook.xml " +
		"is always regenerated. The worksheet part is untouched.",
	"Sheet.SetVisible": "Delegates to SetVisibility; same reason.",
	"Sheet.SetPrintArea": "Print areas are workbook-level defined names (_xlnm.Print_Area), " +
		"not worksheet content, so they persist from the always-regenerated workbook.xml. " +
		"This is also why they legitimately work on an opaque sheet, which markDirty refuses.",
	"Sheet.SetPrintTitles":   "Same as SetPrintArea, for _xlnm.Print_Titles.",
	"Sheet.ClearPrintArea":   "Same as SetPrintArea: clears the workbook-level defined name.",
	"Sheet.ClearPrintTitles": "Same as SetPrintTitles: clears the workbook-level defined name.",

	// --- Workbook: workbook.xml is regenerated from the model on every save ---
	"Workbook.AddDefinedName":          "Edits w.workbook (workbook.xml), which is always regenerated.",
	"Workbook.AddDefinedNameScoped":    "Edits w.workbook (workbook.xml), which is always regenerated.",
	"Workbook.AddDefinedNameFull":      "Edits w.workbook (workbook.xml), which is always regenerated.",
	"Workbook.RemoveDefinedName":       "Edits w.workbook (workbook.xml), which is always regenerated.",
	"Workbook.RemoveDefinedNameScoped": "Edits w.workbook (workbook.xml), which is always regenerated.",
	"Workbook.SetActiveSheet":          "Edits <bookViews> in w.workbook (workbook.xml), always regenerated.",
	"Workbook.SetForceFullCalc":        "Edits <calcPr> in w.workbook (workbook.xml), always regenerated.",
	"Workbook.Protect":                 "Edits <workbookProtection> in w.workbook (workbook.xml), always regenerated.",
	"Workbook.Unprotect":               "Edits <workbookProtection> in w.workbook (workbook.xml), always regenerated.",

	// --- Workbook: regeneration is decided by a snapshot diff, not a flag ---
	"Workbook.SetCustomProperty": "docProps/custom.xml regeneration is derived by " +
		"customPropertiesModified(), which diffs the live properties against the snapshot " +
		"taken at Open. A set flag would be redundant and could not be cleared by an " +
		"edit that restores the original value.",

	// --- Workbook: materializing a default model is a read, not a write ---
	"Workbook.Styles": "Materializes the in-memory default stylesheet for a package that " +
		"carries no styles part. That is a READ: setting stylesDirty here made a plain " +
		"wb.Styles().NamedStyles() add xl/styles.xml, a content-type override and a " +
		"workbook relationship to the saved package. The returned manager's onModify " +
		"flags styles.xml as soon as a mutating method appends, and saveNew writes " +
		"styles.xml for created workbooks whenever w.stylesheet is non-nil.",
	"Workbook.Theme": "Resolves the theme editor lazily; dml.ThemeEditor carries its own " +
		"Modified() flag, which the save path consults instead.",
}

// TestExportedMutatorsFlagState fails when an exported method mutates durable
// document state without any path to a regeneration flag. Adding a new mutator
// that forgets markDirty (the C423 shape) fails here without anyone having to
// remember to extend a table.
func TestExportedMutatorsFlagState(t *testing.T) {
	pkg := parsePackageForGuard(t)

	var offenders []string
	usedExemptions := map[string]bool{}
	for _, key := range sortedKeys(pkg.funcs) {
		f := pkg.funcs[key]
		if f.recv == "" || !ast.IsExported(f.name) {
			continue
		}
		if !pkg.reaches(key, func(x *guardFunc) bool { return x.mutates }) {
			continue
		}
		if pkg.reaches(key, func(x *guardFunc) bool { return x.flags }) {
			continue
		}
		if _, ok := mutationFlagExempt[key]; ok {
			usedExemptions[key] = true
			continue
		}
		offenders = append(offenders, key+" ("+f.pos+") mutates "+strings.Join(f.mutSites, ", "))
	}

	for _, o := range offenders {
		t.Errorf("%s but never reaches a regeneration flag.\n"+
			"Either call markDirty (or the flag that owns the part it writes), or add an "+
			"entry to mutationFlagExempt explaining why the change persists without one.", o)
	}

	for _, key := range sortedKeys(mutationFlagExempt) {
		if usedExemptions[key] {
			continue
		}
		if _, ok := pkg.funcs[key]; !ok {
			t.Errorf("mutationFlagExempt lists %s, which no longer exists; drop the entry", key)
			continue
		}
		t.Errorf("mutationFlagExempt lists %s, but it now reaches a regeneration flag "+
			"(or no longer mutates). Drop the entry so the list stays auditable.", key)
	}
}

// TestSheetDirtyIsOnlySetByMarkDirty guards the invariant markDirty encodes:
// an opaque (chartsheet/dialogsheet/macrosheet) sheet is preserved verbatim and
// must never be flagged, because the save path's sheet loops that decide
// whether to drop calcChain.xml and rebuild the workbook relationships do not
// re-check opaque. Assigning s.dirty directly bypasses that guard — the bug
// AddImage had (C423) and markCommentsDirty had after it.
func TestSheetDirtyIsOnlySetByMarkDirty(t *testing.T) {
	pkg := parsePackageForGuard(t)
	for _, key := range sortedKeys(pkg.funcs) {
		f := pkg.funcs[key]
		if f.name == "markDirty" {
			continue
		}
		for _, site := range f.dirtyAssigns {
			t.Errorf("%s assigns .dirty directly at %s; call s.markDirty() instead so the "+
				"opaque-sheet guard (C241/C423) is not bypassed", key, site)
		}
	}
}

// ---------------------------------------------------------------------------
// AST plumbing
// ---------------------------------------------------------------------------

type guardFunc struct {
	key          string
	name         string
	recv         string
	pos          string
	mutates      bool
	flags        bool
	calls        map[string]bool
	mutSites     []string
	dirtyAssigns []string
}

type guardPkg struct {
	funcs  map[string]*guardFunc
	byName map[string][]string
}

func parsePackageForGuard(t *testing.T) *guardPkg {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading package directory: %v", err)
	}
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		t.Fatal("no non-test sources found in the package directory")
	}
	g := &guardPkg{funcs: map[string]*guardFunc{}, byName: map[string][]string{}}
	for _, file := range files {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			recv, recvVar := "", ""
			if fd.Recv != nil && len(fd.Recv.List) == 1 {
				// Value receivers build detached copies (the ConditionalRule
				// chaining builders); only pointer receivers can mutate.
				if _, isPtr := fd.Recv.List[0].Type.(*ast.StarExpr); !isPtr {
					continue
				}
				recv = receiverTypeName(fd.Recv.List[0].Type)
				if len(fd.Recv.List[0].Names) > 0 {
					recvVar = fd.Recv.List[0].Names[0].Name
				}
			}
			key := fd.Name.Name
			if recv != "" {
				key = recv + "." + fd.Name.Name
			}
			f := &guardFunc{
				key:   key,
				name:  fd.Name.Name,
				recv:  recv,
				pos:   shortPos(fset.Position(fd.Pos()).String()),
				calls: map[string]bool{},
			}
			analyzeGuardFunc(fset, fd, recvVar, recv, f)
			g.funcs[key] = f
		}
	}
	for key := range g.funcs {
		g.byName[key] = append(g.byName[key], key)
		if i := strings.IndexByte(key, '.'); i >= 0 {
			bare := key[i+1:]
			g.byName[bare] = append(g.byName[bare], key)
		}
	}
	return g
}

// reaches reports whether key, or anything it calls in this package, satisfies
// pick. Callee resolution is by name only (the guard runs without type info),
// so a call on a non-receiver value matches every same-named method — which
// makes reachability generous rather than strict.
func (g *guardPkg) reaches(key string, pick func(*guardFunc) bool) bool {
	seen := map[string]bool{}
	var walk func(string) bool
	walk = func(k string) bool {
		if seen[k] {
			return false
		}
		seen[k] = true
		f := g.funcs[k]
		if f == nil {
			return false
		}
		if pick(f) {
			return true
		}
		for _, c := range sortedSet(f.calls) {
			for _, cand := range g.byName[c] {
				if walk(cand) {
					return true
				}
			}
		}
		return false
	}
	return walk(key)
}

func analyzeGuardFunc(fset *token.FileSet, fd *ast.FuncDecl, recvVar, recvType string, f *guardFunc) {
	// Locals that hold a pointer into the receiver's durable model. Iterated to
	// a fixed point so a chain (sv := s.ensureSheetView(); p := sv.Pane) is
	// fully tainted regardless of declaration order.
	tainted := map[string]bool{}
	for i := 0; i < 4; i++ {
		ast.Inspect(fd, func(n ast.Node) bool {
			switch s := n.(type) {
			case *ast.AssignStmt:
				if len(s.Rhs) == len(s.Lhs) {
					for i, rhs := range s.Rhs {
						if isModelExpr(rhs, recvVar, tainted) {
							if id, ok := s.Lhs[i].(*ast.Ident); ok {
								tainted[id.Name] = true
							}
						}
					}
				} else if len(s.Rhs) == 1 && isModelExpr(s.Rhs[0], recvVar, tainted) {
					for _, l := range s.Lhs {
						if id, ok := l.(*ast.Ident); ok {
							tainted[id.Name] = true
						}
					}
				}
			case *ast.RangeStmt:
				if isModelExpr(s.X, recvVar, tainted) || isModelLValue(s.X, recvVar, tainted) {
					if id, ok := s.Value.(*ast.Ident); ok {
						tainted[id.Name] = true
					}
				}
			}
			return true
		})
	}

	note := func(e ast.Expr) {
		f.mutates = true
		if len(f.mutSites) < 3 {
			f.mutSites = append(f.mutSites, shortPos(fset.Position(e.Pos()).String()))
		}
	}
	ast.Inspect(fd, func(n ast.Node) bool {
		// A closure body is a callback the caller may never run (the
		// StyleManager onModify hook is the motivating case), so its writes are
		// not this function's effects.
		if _, isLit := n.(*ast.FuncLit); isLit {
			return false
		}
		switch s := n.(type) {
		case *ast.AssignStmt:
			for _, l := range s.Lhs {
				sel, isSel := l.(*ast.SelectorExpr)
				if isSel && sel.Sel.Name == "dirty" {
					f.dirtyAssigns = append(f.dirtyAssigns, shortPos(fset.Position(l.Pos()).String()))
				}
				if isSel && flagFields[sel.Sel.Name] {
					f.flags = true
					continue
				}
				if isSel && nonDurableFields[sel.Sel.Name] {
					continue
				}
				if isModelLValue(l, recvVar, tainted) {
					note(l)
				}
			}
		case *ast.IncDecStmt:
			if isModelLValue(s.X, recvVar, tainted) {
				note(s.X)
			}
		case *ast.CallExpr:
			if sel, ok := s.Fun.(*ast.SelectorExpr); ok {
				if flagCalls[sel.Sel.Name] {
					f.flags = true
				}
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == recvVar && recvType != "" {
					f.calls[recvType+"."+sel.Sel.Name] = true
				} else {
					f.calls[sel.Sel.Name] = true
				}
			}
			if id, ok := s.Fun.(*ast.Ident); ok {
				f.calls[id.Name] = true
				if id.Name == "delete" && len(s.Args) > 0 && isModelLValue(s.Args[0], recvVar, tainted) {
					note(s.Args[0])
				}
			}
		}
		return true
	})
}

// isModelExpr reports whether e evaluates to a pointer into the receiver's
// durable model.
func isModelExpr(e ast.Expr, recvVar string, tainted map[string]bool) bool {
	switch t := e.(type) {
	case *ast.CallExpr:
		if sel, ok := t.Fun.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == recvVar && modelAccessors[sel.Sel.Name] {
				return true
			}
		}
	case *ast.UnaryExpr:
		if t.Op == token.AND {
			return isModelLValue(t.X, recvVar, tainted) || isModelExpr(t.X, recvVar, tainted)
		}
	case *ast.SelectorExpr:
		return isModelLValue(t, recvVar, tainted)
	case *ast.IndexExpr:
		return isModelLValue(t, recvVar, tainted) || isModelExpr(t.X, recvVar, tainted)
	case *ast.Ident:
		return tainted[t.Name]
	case *ast.ParenExpr:
		return isModelExpr(t.X, recvVar, tainted)
	}
	return false
}

// isModelLValue reports whether e is an assignable location inside the
// receiver's durable model. A bare identifier is excluded on purpose:
// rebinding a local that happens to hold a model pointer changes nothing.
func isModelLValue(e ast.Expr, recvVar string, tainted map[string]bool) bool {
	switch t := e.(type) {
	case *ast.SelectorExpr:
		if id, ok := t.X.(*ast.Ident); ok && id.Name == recvVar {
			return true
		}
		return isModelExpr(t.X, recvVar, tainted) || isModelLValue(t.X, recvVar, tainted)
	case *ast.IndexExpr:
		return isModelExpr(t.X, recvVar, tainted) || isModelLValue(t.X, recvVar, tainted)
	case *ast.StarExpr:
		return isModelExpr(t.X, recvVar, tainted) || isModelLValue(t.X, recvVar, tainted)
	case *ast.ParenExpr:
		return isModelLValue(t.X, recvVar, tainted)
	}
	return false
}

func receiverTypeName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	case *ast.Ident:
		return t.Name
	}
	return ""
}

func shortPos(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
