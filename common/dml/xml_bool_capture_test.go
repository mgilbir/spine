package dml

import (
	"encoding/xml"
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

const boolCapNS = `xmlns="http://schemas.openxmlformats.org/drawingml/2006/main"`

// C587: a producer that spells an xsd:boolean "true" must get "true" back on a
// zero-modification save. Two Common Crawl decks
// (870e964cdd91bc41, c80542f5b7d14881) failed part-level byte identity on
// exactly this: <a:blipFill rotWithShape="true"> regenerated as
// rotWithShape="1".
func TestBoolLexicalForm_PreservedWhenUnmodified(t *testing.T) {
	tests := []struct {
		name  string
		local string
		input string
		newV  func() interface{}
		want  string
	}{
		{
			name:  "blipFill rotWithShape true",
			local: "blipFill",
			input: `<blipFill ` + boolCapNS + ` rotWithShape="true"><stretch/></blipFill>`,
			newV:  func() interface{} { return &BlipFillXML{} },
			want:  `rotWithShape="true"`,
		},
		{
			name:  "blipFill rotWithShape false",
			local: "blipFill",
			input: `<blipFill ` + boolCapNS + ` rotWithShape="false"><stretch/></blipFill>`,
			newV:  func() interface{} { return &BlipFillXML{} },
			want:  `rotWithShape="false"`,
		},
		{
			name:  "gradFill rotWithShape true",
			local: "gradFill",
			input: `<gradFill ` + boolCapNS + ` rotWithShape="true"><lin ang="0" scaled="false"/></gradFill>`,
			newV:  func() interface{} { return &GradFill{} },
			want:  `rotWithShape="true"`,
		},
		{
			name:  "lin scaled false",
			local: "gradFill",
			input: `<gradFill ` + boolCapNS + ` rotWithShape="true"><lin ang="0" scaled="false"/></gradFill>`,
			newV:  func() interface{} { return &GradFill{} },
			want:  `scaled="false"`,
		},
		{
			name:  "blur grow false (XSD default TRUE)",
			local: "blur",
			input: `<blur ` + boolCapNS + ` rad="12700" grow="false"/>`,
			newV:  func() interface{} { return &BlurXML{} },
			want:  `grow="false"`,
		},
		{
			name:  "clrChange useA false (XSD default TRUE)",
			local: "clrChange",
			input: `<clrChange ` + boolCapNS + ` useA="false"><clrFrom><srgbClr val="FFFFFF"/></clrFrom>` +
				`<clrTo><srgbClr val="000000"/></clrTo></clrChange>`,
			newV: func() interface{} { return &ClrChange{} },
			want: `useA="false"`,
		},
		{
			name:  "cNvPicPr preferRelativeResize false (XSD default TRUE)",
			local: "cNvPicPr",
			input: `<cNvPicPr ` + boolCapNS + ` preferRelativeResize="false"/>`,
			newV:  func() interface{} { return &CNvPicPr{} },
			want:  `preferRelativeResize="false"`,
		},
		{
			name:  "nvPr userDrawn true",
			local: "nvPr",
			input: `<nvPr ` + boolCapNS + ` userDrawn="true"/>`,
			newV:  func() interface{} { return &PMLNvPr{} },
			want:  `userDrawn="true"`,
		},
		{
			name:  "rtl val true",
			local: "rtl",
			input: `<rtl ` + boolCapNS + ` val="true"/>`,
			newV:  func() interface{} { return &TextRtl{} },
			want:  `val="true"`,
		},
		{
			name:  "blipFill (a:pic flavor) rotWithShape true",
			local: "blipFill",
			input: `<blipFill ` + boolCapNS + ` rotWithShape="true"><stretch/></blipFill>`,
			newV:  func() interface{} { return &BlipFill{} },
			want:  `rotWithShape="true"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := tt.newV()
			if err := xml.Unmarshal([]byte(tt.input), v); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			out := buildFragment(t, tt.local, v)
			if !strings.Contains(out, tt.want) {
				t.Errorf("lexical form not preserved: want %s in %s", tt.want, out)
			}
		})
	}
}

// A model built in code has no capture, so it emits the canonical "1"/"0".
// This is the pre-existing behaviour and must not change.
func TestBoolLexicalForm_ProgrammaticIsCanonical(t *testing.T) {
	yes := true
	v := &BlipFillXML{RotWithShape: &yes, Stretch: &StretchXML{}}
	out := buildFragment(t, "blipFill", v)
	if !strings.Contains(out, `rotWithShape="1"`) {
		t.Errorf("programmatic value must emit canonical 1: %s", out)
	}
}

// A caller who changes the value gets the model's value, not the capture: the
// two lexemes are no longer equivalent, so replay re-renders (see
// xmlb.ReplayCapturedAttrsClearing).
func TestBoolLexicalForm_ModelWinsAfterEdit(t *testing.T) {
	var v BlipFillXML
	in := `<blipFill ` + boolCapNS + ` rotWithShape="true"><stretch/></blipFill>`
	if err := xml.Unmarshal([]byte(in), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	no := false
	v.RotWithShape = &no
	out := buildFragment(t, "blipFill", &v)
	if !strings.Contains(out, `rotWithShape="0"`) {
		t.Errorf("edited value must beat the capture: %s", out)
	}
	if strings.Contains(out, `rotWithShape="true"`) {
		t.Errorf("stale captured lexeme replayed after edit: %s", out)
	}
}

// Clearing a value-typed bool still deletes the attribute (C586's
// cleared-beats-capture rule): the capture must not resurrect it.
func TestBoolLexicalForm_ClearedValueTypedBoolStillDrops(t *testing.T) {
	var v PMLNvPr
	if err := xml.Unmarshal([]byte(`<nvPr `+boolCapNS+` userDrawn="true"/>`), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !v.UserDrawn {
		t.Fatalf("userDrawn=\"true\" did not parse into the model")
	}
	v.UserDrawn = false
	out := buildFragment(t, "nvPr", &v)
	if strings.Contains(out, "userDrawn") {
		t.Errorf("cleared userDrawn must be deleted, capture must not replay it: %s", out)
	}
}

// A nil *bool is "we hold no value", not "the caller cleared it": replay keeps
// the captured attribute. This is what keeps the XSD default-TRUE booleans
// safe — deleting an explicit grow="false" would flip it to true.
func TestBoolLexicalForm_NilPointerKeepsCapture(t *testing.T) {
	var v BlurXML
	if err := xml.Unmarshal([]byte(`<blur `+boolCapNS+` rad="12700" grow="false"/>`), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	v.Grow = nil
	out := buildFragment(t, "blur", &v)
	if !strings.Contains(out, `grow="false"`) {
		t.Errorf("nil pointer must not delete a default-TRUE attribute: %s", out)
	}
}

// boolCaptureExemptTypes lists structs carrying a bool/*bool attribute that
// deliberately have no CapturedAttrs, with the reason. Both entries are whole
// packages whose types are never parsed and re-marshaled as the same instance,
// so there is no producer lexeme to preserve.
var boolCaptureExemptTypes = map[string]string{
	// chart.Parse decodes a c:chartSpace into the lossy builder model
	// (chart.Chart) and MarshalChartXML builds a *fresh* dmlchart.ChartSpace
	// from it; no parsed chart instance is ever re-marshaled, and chart parts
	// are carried through as raw bytes on a zero-modification save.
	"chart.Boolean":           "chart parse and marshal never share an instance",
	"chart.NumFmt":            "chart parse and marshal never share an instance",
	"chart.HeaderFooterChart": "chart parse and marshal never share an instance",
	"chart.PageSetup":         "chart parse and marshal never share an instance",

	// diagram.ParseDataModel is read-only (SmartArt inspection) and
	// diagram.Build writes parts from a programmatic model; diagram parts are
	// carried through as raw bytes on a zero-modification save.
	"diagram.BoolVal":      "diagram parts are read-only on parse, built fresh on write",
	"diagram.PrSet":        "diagram parts are read-only on parse, built fresh on write",
	"diagram.CategoryData": "diagram parts are read-only on parse, built fresh on write",
	"diagram.SampleData":   "diagram parts are read-only on parse, built fresh on write",
	"diagram.StyleData":    "diagram parts are read-only on parse, built fresh on write",
	"diagram.LayoutShape":  "diagram parts are read-only on parse, built fresh on write",
}

// The lexical-form class has been closed one field at a time four times over
// (percentages, date1904, tints, animation floats, and now rotWithShape). This
// guard closes it mechanically for DrawingML booleans: every struct in this
// package tree with a bool or *bool attribute must carry CapturedAttrs, or be
// listed in boolCaptureExemptTypes with a reason.
//
// Unlike the pptx/internal/oxml coverage guard, which is about *semantic* loss
// and so only looks at value-typed omitempty fields, this one covers pointers
// too: a *bool loses no value, but it does lose the producer's spelling.
func TestBoolAttrCaptureCoverage(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Dir(thisFile)
	pkgs := []string{root, filepath.Join(root, "chart"), filepath.Join(root, "diagram")}

	var missing []string
	unusedExempt := make(map[string]bool, len(boolCaptureExemptTypes))
	for k := range boolCaptureExemptTypes {
		unusedExempt[k] = true
	}
	seenAny := false

	for _, dir := range pkgs {
		pkgName := filepath.Base(dir)
		if dir == root {
			pkgName = "dml"
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		fset := token.NewFileSet()
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", name, err)
			}
			ast.Inspect(f, func(n ast.Node) bool {
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
				for _, fl := range st.Fields.List {
					if fl.Tag == nil {
						continue
					}
					tv, err := strconv.Unquote(fl.Tag.Value)
					if err != nil {
						continue
					}
					parts := strings.Split(reflect.StructTag(tv).Get("xml"), ",")
					isAttr := false
					for _, p := range parts[1:] {
						if p == "attr" {
							isAttr = true
						}
					}
					if !isAttr || !isBoolType(fl.Type) {
						continue
					}
					seenAny = true
					key := pkgName + "." + ts.Name.Name
					delete(unusedExempt, key)
					if captured {
						continue
					}
					if _, ok := boolCaptureExemptTypes[key]; ok {
						continue
					}
					for _, nm := range fl.Names {
						missing = append(missing, key+"."+nm.Name+" (`"+parts[0]+"`) at "+
							fset.Position(fl.Pos()).String())
					}
				}
				return true
			})
		}
	}
	if !seenAny {
		t.Fatal("no boolean attributes found; the guard would pass vacuously")
	}

	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("xsd:boolean attributes on types with no CapturedAttrs: a producer's "+
			"\"true\"/\"false\" spelling is renormalized to \"1\"/\"0\" and the part cannot "+
			"come back byte-identical (C587). Add CapturedAttrs + an UnmarshalXML hook in "+
			"xml_bool_capture.go, or a justified entry in boolCaptureExemptTypes:\n  %s",
			strings.Join(missing, "\n  "))
	}

	var stale []string
	for k := range unusedExempt {
		stale = append(stale, k)
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("boolCaptureExemptTypes entries no longer match any type (remove them):\n  %s",
			strings.Join(stale, "\n  "))
	}
}

// isBoolType reports whether a struct field's type is bool or *bool.
func isBoolType(e ast.Expr) bool {
	if star, ok := e.(*ast.StarExpr); ok {
		e = star.X
	}
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "bool"
}
