package diagram

import (
	"encoding/xml"
	"strings"
	"testing"
)

// TestPrSetKeepsElementChildren pins C485: CT_ElemPropSet is not attribute-only
// — it carries dgm:presLayoutVars and dgm:style — and the attr-only model
// discarded both on any parse→marshal, while diagram_test.go advertised that
// these types round-trip.
func TestPrSetKeepsElementChildren(t *testing.T) {
	src := `<dgm:prSet xmlns:dgm="` + NsDiagram + `" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" presAssocID="1" phldrT="Text">` +
		`<dgm:presLayoutVars><dgm:chMax val="4"/><dgm:dir val="norm"/></dgm:presLayoutVars>` +
		`<dgm:style><a:lnRef idx="2"><a:schemeClr val="accent1"/></a:lnRef><a:fillRef idx="1"><a:schemeClr val="accent1"/></a:fillRef><a:effectRef idx="0"><a:schemeClr val="accent1"/></a:effectRef><a:fontRef idx="minor"><a:schemeClr val="lt1"/></a:fontRef></dgm:style>` +
		`</dgm:prSet>`

	var ps PrSet
	if err := xml.Unmarshal([]byte(src), &ps); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ps.PresLayoutVars == nil {
		t.Fatal("presLayoutVars dropped on parse (C485)")
	}
	if ps.PresLayoutVars.ChMax == nil || ps.PresLayoutVars.ChMax.Val != 4 {
		t.Errorf("chMax = %+v, want val=4", ps.PresLayoutVars.ChMax)
	}
	if ps.Style == nil {
		t.Fatal("style dropped on parse (C485)")
	}
	if ps.Style.FillRef == nil || ps.Style.FillRef.Idx != 1 {
		t.Errorf("style fillRef = %+v, want idx=1", ps.Style.FillRef)
	}

	out, err := xml.Marshal(&ps)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(out)
	for _, want := range []string{"presLayoutVars", "chMax", "style", "fillRef"} {
		if !strings.Contains(got, want) {
			t.Errorf("re-marshaled prSet lost %s (C485): %s", want, got)
		}
	}
	// The attributes must still survive alongside the new children.
	if !strings.Contains(got, `presAssocID="1"`) || !strings.Contains(got, `phldrT="Text"`) {
		t.Errorf("prSet attributes lost: %s", got)
	}
}

// TestColorListKeepsAllKindsInOrder pins the other half of C485: CT_Colors is a
// positional repeated a:EG_ColorChoice, and modeling only srgbClr/schemeClr
// deleted the other four kinds outright and regrouped what was left.
func TestColorListKeepsAllKindsInOrder(t *testing.T) {
	src := `<dgm:fillClrLst xmlns:dgm="` + NsDiagram + `" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" meth="repeat" hueDir="cw">` +
		`<a:schemeClr val="accent1"/>` +
		`<a:srgbClr val="FF0000"/>` +
		`<a:sysClr val="windowText" lastClr="000000"/>` +
		`<a:schemeClr val="accent2"/>` +
		`<a:prstClr val="black"/>` +
		`<a:hslClr hue="14400000" sat="50000" lum="50000"/>` +
		`<a:scrgbClr r="50000" g="50000" b="50000"/>` +
		`</dgm:fillClrLst>`

	var cl ColorList
	if err := xml.Unmarshal([]byte(src), &cl); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(cl.SysClr) != 1 || len(cl.PrstClr) != 1 || len(cl.HslClr) != 1 || len(cl.ScRgbClr) != 1 {
		t.Fatalf("non-srgb/scheme kinds dropped (C485): sys=%d prst=%d hsl=%d scrgb=%d",
			len(cl.SysClr), len(cl.PrstClr), len(cl.HslClr), len(cl.ScRgbClr))
	}

	out, err := xml.Marshal(cl)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := []string{"schemeClr", "srgbClr", "sysClr", "schemeClr", "prstClr", "hslClr", "scrgbClr"}
	if got := colorKindOrder(t, string(out)); !equalStrings(got, want) {
		t.Errorf("color order = %v, want %v (C485/C401 shape)", got, want)
	}
	if !strings.Contains(string(out), `meth="repeat"`) || !strings.Contains(string(out), `hueDir="cw"`) {
		t.Errorf("attributes lost: %s", out)
	}
}

// TestBuildSurfacesSerializationErrors pins C487: buildDataModel returned
// b.Bytes() without checking Finish, so a Builder error shipped a truncated
// part; Build had no error return to surface it.
func TestBuildSurfacesSerializationErrors(t *testing.T) {
	parts, err := Build(KindList, []BuildNode{{Text: "one"}, {Text: "two", Children: []BuildNode{{Text: "child"}}}})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	// A complete part: the error check must not truncate the happy path.
	for _, p := range [][]byte{parts.Data, parts.Layout, parts.QuickStyle, parts.Colors} {
		if len(p) == 0 {
			t.Fatal("Build produced an empty part")
		}
	}
	if !strings.HasSuffix(string(parts.Data), "</dgm:dataModel>") {
		t.Errorf("data part is truncated: ...%s", tail(string(parts.Data), 80))
	}
	// It must still be parseable end to end.
	var dm DataModel
	if err := xml.Unmarshal(parts.Data, &dm); err != nil {
		t.Fatalf("generated data model does not parse: %v", err)
	}
	if dm.PtLst == nil || len(dm.PtLst.Pt) != 4 {
		t.Errorf("expected doc root + 3 nodes, got %+v", dm.PtLst)
	}
}

// colorKindOrder lists the EG_ColorChoice kinds at the top level of a CT_Colors
// fragment, in document order.
func colorKindOrder(t *testing.T, fragment string) []string {
	t.Helper()
	d := xml.NewDecoder(strings.NewReader(fragment))
	var out []string
	depth := 0
	for {
		tok, err := d.Token()
		if err != nil {
			return out
		}
		switch tk := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 {
				if _, ok := clrNameKind[tk.Name.Local]; ok {
					out = append(out, tk.Name.Local)
				}
			}
		case xml.EndElement:
			depth--
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
