package oxml

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The CapturedAttrs convention has been adopted one finding at a time
// (SequenceTimeNode but not Iterate, AnimateScale but not Audio, ...), so the
// gaps stayed invisible until a wild file found one — design tension T-C of the
// 2026-07-27 audit. This test closes the class mechanically instead: it walks
// every struct in the package and flags any *value-typed* numeric or boolean
// attribute carrying `omitempty` on a type that does not carry CapturedAttrs.
//
// Such a field cannot represent "the producer wrote the default explicitly":
// omitempty deletes the attribute on the way out, which is a byte drift always
// and a semantic change whenever the XSD default is not the Go zero value
// (spokes=4, numSld, animBg, showComments, ...).
//
// A type that legitimately has no capture must be listed in
// captureExemptAttrs with the reason. Adding a new value-typed default-valued
// attribute without either a capture or an entry here fails this test.
var captureExemptAttrs = map[string]string{
	// --- presProps / viewProps -------------------------------------------
	// These two parts are copied through as raw bytes (nothing references the
	// typed structs on the save path), and every attribute below is XSD
	// default-FALSE or default-0, so omitempty round-trips every non-default
	// value. The default-TRUE attributes of the same parts are modeled as *bool
	// (C526). If either part ever gains a regeneration path, these become real
	// and need CapturedAttrs.
	"WebProperties.ShowAnimation":             "presProps passes through raw; XSD default false",
	"WebProperties.AllowPng":                  "presProps passes through raw; XSD default false",
	"WebProperties.RelyOnVml":                 "presProps passes through raw; XSD default false",
	"PrintProperties.HiddenSlides":            "presProps passes through raw; XSD default false",
	"PrintProperties.ScaleToFitPaper":         "presProps passes through raw; XSD default false",
	"PrintProperties.FrameSlides":             "presProps passes through raw; XSD default false",
	"ShowProperties.Loop":                     "presProps passes through raw; XSD default false",
	"ShowProperties.ShowNarration":            "presProps passes through raw; XSD default false",
	"ShowInfoKiosk.Restart":                   "presProps passes through raw; XSD default 0",
	"NormalViewProperties.SnapVertSplitter":   "viewProps passes through raw; XSD default false",
	"NormalViewProperties.PreferSingleView":   "viewProps passes through raw; XSD default false",
	"NormalViewPortion.Sz":                    "viewProps passes through raw; XSD default 0",
	"CommonSlideViewProperties.SnapToObjects": "viewProps passes through raw; XSD default false",
	"CommonSlideViewProperties.ShowGuides":    "viewProps passes through raw; XSD default false",
	"CommonViewProperties.VarScale":           "viewProps passes through raw; XSD default false",
	"OutlineViewSlideEntry.Collapse":          "viewProps passes through raw; XSD default false",
	"Guide.Pos":                               "viewProps passes through raw; XSD default 0",

	// --- part roots -------------------------------------------------------
	// Root attributes are captured verbatim by OriginalRootAttrs and replayed by
	// StartElementWithRootAttrsMerged, which covers the explicit-zero case; the
	// remaining gap for these is *clearing* a parsed value (tension T-D), not
	// dropping one.
	"SlideLayout.Preserve":  "root attrs captured by SlideLayout.OriginalRootAttrs",
	"SlideLayout.UserDrawn": "root attrs captured by SlideLayout.OriginalRootAttrs",
	"SlideMaster.Preserve":  "root attrs captured by SlideMaster.OriginalRootAttrs",

	// --- id-list entries --------------------------------------------------
	// ST_SlideLayoutId / ST_SlideMasterId have a minimum of 2147483648, so 0 is
	// never a legal value to preserve; the hand-written MarshalToBuilder omits
	// it deliberately (C355).
	"SlideLayoutID.ID": "ST_SlideLayoutId minimum is 2147483648; 0 is never valid",
	"SlideMasterID.ID": "ST_SlideMasterId minimum is 2147483648; 0 is never valid",
}

func TestCaptureCoverage_ValueTypedDefaultAttributes(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		t.Fatal("no package files found; the guard would pass vacuously")
	}

	var missing []string
	unusedExempt := make(map[string]bool, len(captureExemptAttrs))
	for k := range captureExemptAttrs {
		unusedExempt[k] = true
	}

	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return true
			}
			captured := false
			for _, fl := range st.Fields.List {
				for _, nm := range fl.Names {
					if nm.Name == "CapturedAttrs" {
						captured = true
					}
				}
			}
			if captured {
				return true
			}
			for _, fl := range st.Fields.List {
				if fl.Tag == nil {
					continue
				}
				tv, err := strconv.Unquote(fl.Tag.Value)
				if err != nil {
					continue
				}
				parts := strings.Split(reflect.StructTag(tv).Get("xml"), ",")
				isAttr, omit := false, false
				for _, p := range parts[1:] {
					switch p {
					case "attr":
						isAttr = true
					case "omitempty":
						omit = true
					}
				}
				if !isAttr || !omit {
					continue
				}
				id, ok := fl.Type.(*ast.Ident)
				if !ok {
					continue // pointer, named or qualified type: not the pattern
				}
				switch {
				case id.Name == "bool",
					strings.HasPrefix(id.Name, "int"),
					strings.HasPrefix(id.Name, "uint"),
					strings.HasPrefix(id.Name, "float"):
				default:
					continue
				}
				for _, nm := range fl.Names {
					key := ts.Name.Name + "." + nm.Name
					delete(unusedExempt, key)
					if _, ok := captureExemptAttrs[key]; ok {
						continue
					}
					missing = append(missing, key+" ("+id.Name+" `"+parts[0]+"`) at "+
						fset.Position(fl.Pos()).String())
				}
			}
			return true
		})
	}

	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("value-typed default-valued attributes with no CapturedAttrs on their type "+
			"(an explicit \"0\" written by the producer is deleted on re-marshal). "+
			"Add CapturedAttrs + UnmarshalXML to the type, model the field as a pointer, "+
			"or add a justified entry to captureExemptAttrs:\n  %s",
			strings.Join(missing, "\n  "))
	}

	var stale []string
	for k := range unusedExempt {
		stale = append(stale, k)
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("captureExemptAttrs entries no longer match any field (remove them):\n  %s",
			strings.Join(stale, "\n  "))
	}
}
