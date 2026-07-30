package maporder_test

import (
	"fmt"
	"go/ast"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/mgilbir/spine/internal/maporder"
)

// exempt lists the map ranges that are order-dependent by the analyser's rules
// but order-independent in fact, each with the reason it is safe. An entry that
// no longer matches a finding fails the test, so the list cannot rot.
//
// Keys are "<path>:<func>:<ranged expression>:<kind>" — deliberately not line
// numbers, which would rot on the first unrelated edit above them.
var exempt = map[string]string{
	"common/xml/c14n.go:c14nElem.inheritedXMLAttrs:e.inheritedXML:collect": "" +
		"The slice is handed to c14nElem.render as `inherited`, which concatenates it " +
		"with the element's own attributes and sorts the result by (namespace URI, " +
		"local name) — the C14N attribute order. Every entry here carries the same URI " +
		"and a local name that is a distinct key of e.inheritedXML, so the sort is a " +
		"total order and no trace of the map order survives it.",

	"common/xml/rawattrs.go:prefixForAttr:ExtensionPrefixToNS:escape": "" +
		"A reverse lookup: it returns the prefix declared for a namespace URI. At most " +
		"one entry can match because the map is injective, which " +
		"TestReverseLookupMapsAreInjective checks rather than assumes.",

	"opc/content_types.go:PlainFlavor:macroFlavorMap:escape": "" +
		"The reverse of MacroFlavor: it returns the non-macro content type whose " +
		"macro-enabled counterpart is the argument. At most one entry can match because " +
		"the map is injective, checked by TestReverseLookupMapsAreInjective.",

	"xlsx/style.go:StyleManager.resolveNumFmtCode:builtinNumFmtCodes:escape": "" +
		"A reverse lookup from a built-in numFmtId to its format code. The twenty " +
		"built-in ids are distinct, so at most one entry can match; " +
		"TestReverseLookupMapsAreInjective checks it.",

	"docx/relsweep.go:Paragraph.sweepRemovedRelRefs:removed:collect": "" +
		"`drop` is passed straight to Document.dropPartRelationships, whose first act " +
		"is to turn it back into a set. Nothing reads its order, and the relationships " +
		"it filters are walked in their own slice order, not in the order of `drop`.",

	"pptx/presentation.go:Presentation.sweepInboundPartReferences:p.relationships:collect": "" +
		"`sources` is a snapshot taken so the map is not mutated under range (which " +
		"materializing a slide would do). Each iteration touches only its own source " +
		"part — stripSlideReferences edits that part's slide, dropRelationships filters " +
		"that part's relationship slice in place — so the parts are independent and the " +
		"resulting package is the same whatever order they are swept in.",

	"docx/validate.go:Document.knownPartNames:d.preservedParts:collect": "" +
		"d.preservedParts is filled only by the open loop, from d.reader.Files, and " +
		"knownPartNames adds reader.Files first — so `add` has already seen every key " +
		"and this range contributes no name and no order. (The header and footer names " +
		"below it are sorted, because those maps DO gain parts during a session; that " +
		"was C497.)",

	"xlsx/validate.go:Workbook.knownPartNames:w.preservedParts:collect": "" +
		"Same as the docx case: w.preservedParts is populated only by the open loop, " +
		"from w.reader.Files, which knownPartNames adds first, so every key is already " +
		"in `seen` by the time this range runs.",
}

// injectiveMaps are the package-level maps the reverse-lookup exemptions above
// rely on. A reverse lookup over a map with two keys sharing a value returns an
// arbitrary one of them; these entries assert the premise instead of trusting it.
var injectiveMaps = map[string]string{
	"common/xml:ExtensionPrefixToNS": "prefixForAttr returns the prefix for a URI",
	"opc:macroFlavorMap":             "PlainFlavor returns the plain type for a macro type",
	"xlsx:builtinNumFmtCodes":        "resolveNumFmtCode returns the format code for an id",
}

// landmarks are map ranges that must be seen and must come out clean. They are
// the guard's own smoke test: if the analyser stops resolving types, or stops
// recognising collect-then-sort, one of these changes state and says so.
var landmarks = []string{
	"docx/document.go:Document.saveRoundTrip:d.preservedParts:collect",
	"xlsx/workbook.go:Workbook.saveRoundTrip:w.preservedParts:collect",
	"opc/content_types.go:ContentTypes.orderedOverrides:ct.Overrides:collect",
	"pptx/activex.go:Presentation.ActiveXControls:p.otherParts:collect",
}

// Floors on how much the sweep covered. A guard that type-checks nothing and
// reports clean is the failure this repository keeps hitting, so these are
// generous enough not to nag and tight enough that a collapse is loud.
const (
	minPackages  = 15
	minFiles     = 250
	minFuncs     = 3500
	minExprs     = 150000
	minMapRanges = 70
)

var (
	once   sync.Once
	result *maporder.Analysis
	loaded error
)

// analyze runs the sweep once for the whole test binary: loading and walking
// eighteen packages costs a little under two seconds, which is worth paying
// once rather than four times.
func analyze(t *testing.T) *maporder.Analysis {
	t.Helper()
	once.Do(func() {
		result, loaded = maporder.Analyze(repoRoot(), maporder.Patterns)
	})
	if loaded != nil {
		t.Fatalf("analysing the repository: %v", loaded)
	}
	return result
}

func repoRoot() string { return filepath.Join("..", "..") }

// TestNoOrderDependentMapRanges is the guard.
//
// Go randomises map iteration order, so ranging a map and emitting as you go
// produces different bytes on every run. That has happened for real twice —
// docx.Charts() returning headers and footers in map order (C497), and the pptx
// media dedup scan picking whichever byte-identical part it saw first (C515) —
// and both were found by accident rather than by anything that was watching.
func TestNoOrderDependentMapRanges(t *testing.T) {
	an := analyze(t)
	used := map[string]bool{}
	for _, f := range an.Findings {
		if _, ok := exempt[f.Key]; ok {
			used[f.Key] = true
			continue
		}
		expr := strings.TrimSuffix(strings.SplitN(f.Key, ":", 3)[2], ":"+f.Kind)
		t.Errorf("%s: %s iterates %s in map order (%s): %s\n"+
			"    Collect the keys, sort them, then iterate the sorted slice — or add %q to "+
			"the exempt map in this file with the reason the order cannot reach the output.",
			f.Pos, f.Func, expr, f.Kind, f.Detail, f.Key)
	}
	for _, key := range sortedKeys(exempt) {
		if used[key] {
			continue
		}
		t.Errorf("exempt lists %q, but the sweep no longer reports it. "+
			"Drop the entry so the list stays readable and auditable.", key)
	}
}

// TestGuardCatchesPlantedViolations runs the analyser over testdata/planted,
// which holds one of each defect shape beside the order-independent patterns
// they are easily confused with. It fails if a violation stops being reported
// (the guard has gone blind) or if a safe pattern starts being reported (the
// guard has become noise that people will switch off).
func TestGuardCatchesPlantedViolations(t *testing.T) {
	an, err := maporder.Analyze(repoRoot(), []string{
		"./internal/maporder/testdata/planted",
		"./internal/maporder/testdata/plantedhelper",
	})
	if err != nil {
		t.Fatalf("analysing the planted fixture: %v", err)
	}
	const prefix = "internal/maporder/testdata/planted"
	wantKinds := map[string]string{
		"badEmitToBuffer":       maporder.KindEmit,
		"badEmitToBuilder":      maporder.KindEmit,
		"badEmitThroughHelper":  maporder.KindEmit,
		"badEmitAcrossPackages": maporder.KindEmit,
		"badCollectUnsorted":    maporder.KindCollect,
		"badSelectFirstMatch":   maporder.KindEscape,
		"badEscapeOutward":      maporder.KindEscape,
	}
	got := map[string]string{}
	for _, f := range an.Findings {
		if !strings.HasPrefix(f.Key, prefix) {
			t.Errorf("finding outside the fixture: %s", f.Key)
			continue
		}
		got[f.Func] = f.Kind
		if strings.HasPrefix(f.Func, "ok") {
			t.Errorf("%s: reported the order-independent %s as %s (%s). "+
				"A guard that flags safe code is a guard people turn off.",
				f.Pos, f.Func, f.Kind, f.Detail)
		}
	}
	for fn, kind := range wantKinds {
		switch {
		case got[fn] == "":
			t.Errorf("planted violation %s was NOT reported; the guard is blind to it", fn)
		case got[fn] != kind:
			t.Errorf("planted violation %s reported as %q, want %q", fn, got[fn], kind)
		}
	}
	if an.MapRanges < len(wantKinds) {
		t.Errorf("fixture sweep saw %d map ranges, expected at least %d — type information "+
			"is probably missing", an.MapRanges, len(wantKinds))
	}
}

// TestGuardSeesEnough pins how much the sweep covered. Type-checking from
// source is the part most likely to degrade quietly: an import that stops
// resolving leaves the expression map empty, every `range` looks untyped, and
// the guard reports a clean repository for ever.
func TestGuardSeesEnough(t *testing.T) {
	an := analyze(t)
	for _, c := range []struct {
		name string
		got  int
		min  int
	}{
		{"packages", an.Packages, minPackages},
		{"files", an.Files, minFiles},
		{"functions", an.Funcs, minFuncs},
		{"typed expressions", an.Exprs, minExprs},
		{"map ranges", an.MapRanges, minMapRanges},
	} {
		if c.got < c.min {
			t.Errorf("sweep covered %d %s, expected at least %d: the analysis has "+
				"degraded and a clean result no longer means anything", c.got, c.name, c.min)
		}
	}

	seen := map[string]bool{}
	for _, k := range an.Loops {
		seen[k] = true
	}
	reported := map[string]bool{}
	for _, f := range an.Findings {
		reported[f.Key] = true
	}
	for _, l := range landmarks {
		key := strings.TrimSuffix(l, ":"+maporder.KindCollect)
		if !seen[key] {
			t.Errorf("landmark map range %s was not analysed at all; either it was "+
				"rewritten (update the landmark) or the sweep stopped reaching it", key)
			continue
		}
		if reported[l] {
			t.Errorf("landmark map range %s is now reported — it used to collect-then-sort. "+
				"Either it regressed, or the classification did", key)
		}
	}
}

// TestReverseLookupMapsAreInjective checks the premise the reverse-lookup
// exemptions rest on: a `for k, v := range m { if v == want { return k } }` is
// deterministic only while no two keys of m share a value. Adding a duplicate
// value would make those three functions nondeterministic without touching
// them, so the check lives here rather than in a comment.
func TestReverseLookupMapsAreInjective(t *testing.T) {
	found := map[string]bool{}
	for _, p := range analyze(t).Prog.Packages {
		for _, f := range p.Files {
			for _, decl := range f.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok {
					continue
				}
				for _, spec := range gd.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
						continue
					}
					key := maporder.ShortPath(p.ImportPath) + ":" + vs.Names[0].Name
					if _, want := injectiveMaps[key]; !want {
						continue
					}
					found[key] = true
					lit, ok := vs.Values[0].(*ast.CompositeLit)
					if !ok {
						t.Errorf("%s is no longer a composite literal; the injectivity "+
							"check cannot read it", key)
						continue
					}
					checkInjective(t, p, key, lit)
				}
			}
		}
	}
	for _, key := range sortedKeys(injectiveMaps) {
		if !found[key] {
			t.Errorf("injectiveMaps names %s (%s), which no longer exists",
				key, injectiveMaps[key])
		}
	}
}

func checkInjective(t *testing.T, p *maporder.Package, key string, lit *ast.CompositeLit) {
	t.Helper()
	byValue := map[string]string{}
	n := 0
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		tv, ok := p.Info.Types[kv.Value]
		if !ok || tv.Value == nil {
			t.Errorf("%s: entry %s has no constant value; the injectivity check "+
				"cannot verify it", key, exprText(kv.Key))
			continue
		}
		n++
		v := tv.Value.ExactString()
		if first, dup := byValue[v]; dup {
			t.Errorf("%s maps both %s and %s to %s. The reverse lookup that reads this "+
				"map is now nondeterministic — it returns whichever key the runtime "+
				"visits first (%s).", key, first, exprText(kv.Key), v, injectiveMaps[key])
			continue
		}
		byValue[v] = exprText(kv.Key)
	}
	if n < 5 {
		t.Errorf("%s: only %d entries had readable constant values; the injectivity "+
			"check has stopped working", key, n)
	}
}

func exprText(e ast.Expr) string {
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	if bl, ok := e.(*ast.BasicLit); ok {
		return bl.Value
	}
	return fmt.Sprintf("%T", e)
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
