package xml_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// RawAttrListOverride replays a parsed element's captured attribute list with
// modeled values substituted. It iterates the *captured* list, so a name in the
// override map that the source did not write is silently ignored: the model is
// updated, the save serializes the original attributes, and the edit disappears
// with no error.
//
// That is not hypothetical. SetResolved(true) on a modern comment read from a
// file did exactly this — @status is optional and absent on an unresolved
// comment, which is precisely the comment anyone resolves — so the resolve was
// dropped on every save. Reproduced end to end before the fix: Resolved()
// reported true in memory, the saved part carried no status, and reopening
// reported false.
//
// A call site is only safe when the overridden attribute is *guaranteed present*
// on any element that parsed into this model — normally because the schema makes
// it required. That is a fact about the schema, not about the Go source, so no
// static rule can decide it. What this guard does instead is force the decision
// to be written down once per call site, and fail when a new one appears
// undeclared.

// overrideKind is why a RawAttrListOverride call site cannot silently drop a
// modeled value.
type overrideKind int

const (
	// schemaRequired: every overridden name is mandatory on the element, so a
	// parsed element always carries it and the substitution always lands.
	schemaRequired overrideKind = iota
	// appendsWhenMissing: the caller adds the attribute itself when the source
	// did not write it, so an optional attribute is safe here.
	appendsWhenMissing
	// unreachableToday: the overridden name can legitimately be absent, but no
	// code path reaches the replay branch with a model value the source lacked.
	// Latent rather than live — and the reason has to say why.
	unreachableToday
)

type overrideSite struct {
	kind   overrideKind
	reason string
}

// overrideSites declares every call to RawAttrListOverride in the module. Keys
// are "<path>:<function>" — not line numbers, which rot on the first unrelated
// edit above them. A call site that is not listed fails the test, and a listed
// site that no longer exists fails it too, so the table cannot rot in either
// direction.
var overrideSites = map[string]overrideSite{
	"common/dml/xml_extension.go:Ext.marshalContent": {
		kind: schemaRequired,
		reason: "Overrides a16:creationId/@id, a:colId/@val, a:rowId/@val and " +
			"asvg:svgBlip/@r:embed. Each is required by its schema — an element " +
			"missing it would not have parsed into the typed field — so the name " +
			"is always in the captured list.",
	},
	"common/dml/chart/types.go:RelId.MarshalToBuilder": {
		kind: schemaRequired,
		reason: "Overrides @r:id on a relationship-reference element whose only " +
			"purpose is to carry it; the element cannot be parsed without one.",
	},
	"pptx/internal/oxml/extension.go:Extension.MarshalToBuilder": {
		kind: unreachableToday,
		reason: "p14:creationId/@val and p14:modId/@val are schema-required and safe. " +
			"p14:media is the exception: @r:embed and @r:link are an either/or pair, " +
			"so a source carrying one has no attribute for the other to substitute " +
			"into, and switching a parsed linked medium to embedded would be dropped. " +
			"Not reachable today — the only writer (pptx/media_embed.go) builds a " +
			"fresh P14Media with no CapturedAttrs, so the replay branch is not taken, " +
			"and no exported API moves a parsed medium between link and embed. " +
			"An API that does must append when the counterpart is absent.",
	},
	"pptx/internal/oxml/moderncomments_marshal.go:replayP188Attrs": {
		kind: appendsWhenMissing,
		reason: "The clearable list (@status) is authoritative both ways: an empty " +
			"model value removes the attribute, a non-empty one is appended when the " +
			"source lacked it. This is the site the rule was written for.",
	},
}

func TestRawAttrListOverrideSitesAreDeclared(t *testing.T) {
	root := moduleRoot(t)

	found := map[string]int{}
	files := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		files++
		for _, key := range overrideCallSites(path, filepath.ToSlash(rel)) {
			found[key]++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	if files < 200 {
		t.Fatalf("guard scanned only %d files; the walk is broken, not the code clean", files)
	}

	// The guard is worthless if it resolves nothing: the helper has callers, and
	// finding none means the detection broke rather than the callers vanished.
	if len(found) == 0 {
		t.Fatal("no RawAttrListOverride call sites found at all; the detection is broken")
	}

	var undeclared []string
	for key := range found {
		if _, ok := overrideSites[key]; !ok {
			undeclared = append(undeclared, key)
		}
	}
	sort.Strings(undeclared)
	for _, key := range undeclared {
		t.Errorf("undeclared RawAttrListOverride call site: %s\n"+
			"\tThe helper substitutes only into attributes the SOURCE wrote: a name the\n"+
			"\tparsed element lacked is silently ignored and the modeled value is lost on\n"+
			"\tsave. Decide which applies and add an entry to overrideSites:\n"+
			"\t  - every overridden name is schema-required (so always captured), or\n"+
			"\t  - this caller appends the attribute when the source lacked it, or\n"+
			"\t  - the drop is unreachable, with the reason why.", key)
	}

	var latent []string
	for key, site := range overrideSites {
		if found[key] == 0 {
			t.Errorf("overrideSites has a stale entry %q: no call site matches it, so remove it", key)
		}
		if strings.TrimSpace(site.reason) == "" {
			t.Errorf("%s is declared with no reason; the reason is the whole point of the entry", key)
		}
		if site.kind == unreachableToday {
			latent = append(latent, key)
		}
	}

	// Surface the latent sites on every run rather than letting them sit in a
	// table nobody opens. These are the ones where the drop is real and only the
	// reachability argument holds — exactly the argument an added API breaks.
	sort.Strings(latent)
	for _, key := range latent {
		t.Logf("latent (safe only because unreachable): %s\n\t%s", key, overrideSites[key].reason)
	}
}

// TestRawAttrListOverrideGuardFires is the positive control: an undeclared call
// site must be reported. Without it, a refactor that broke the detection would
// leave a guard that passes on everything.
func TestRawAttrListOverrideGuardFires(t *testing.T) {
	const src = `package p

import xmlb "github.com/mgilbir/spine/common/xml"

func (e *Thing) MarshalToBuilder() {
	_ = xmlb.RawAttrListOverride(e.CapturedAttrs, map[string]string{"val": e.Val})
}

func clean() {}
`
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "planted.go", src, 0); err != nil {
		t.Fatalf("parsing planted source: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "planted.go")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	got := overrideCallSites(path, "planted.go")
	want := "planted.go:Thing.MarshalToBuilder"
	if len(got) != 1 || got[0] != want {
		t.Errorf("overrideCallSites = %v, want exactly [%s]", got, want)
	}
	if _, declared := overrideSites[want]; declared {
		t.Errorf("the planted key %q is in overrideSites, which would mask the control", want)
	}
}

// overrideCallSites returns "<rel>:<func>" for every RawAttrListOverride call in
// one file.
func overrideCallSites(path, rel string) []string {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil
	}

	var out []string
	var stack []string
	var walk func(ast.Node)
	walk = func(root ast.Node) {
		ast.Inspect(root, func(n ast.Node) bool {
			switch node := n.(type) {
			case nil:
				return false
			case *ast.FuncDecl:
				name := node.Name.Name
				if node.Recv != nil && len(node.Recv.List) > 0 {
					name = receiverTypeName(node.Recv.List[0].Type) + "." + name
				}
				stack = append(stack, name)
				if node.Body != nil {
					walk(node.Body)
				}
				stack = stack[:len(stack)-1]
				return false
			case *ast.CallExpr:
				if isOverrideCall(node) && len(stack) > 0 {
					out = append(out, fmt.Sprintf("%s:%s", rel, stack[len(stack)-1]))
				}
			}
			return true
		})
	}
	walk(file)
	return out
}

// isOverrideCall reports whether a call is to RawAttrListOverride, through any
// import alias, or unqualified inside common/xml itself.
func isOverrideCall(call *ast.CallExpr) bool {
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		return fn.Sel.Name == "RawAttrListOverride"
	case *ast.Ident:
		return fn.Name == "RawAttrListOverride"
	}
	return false
}
