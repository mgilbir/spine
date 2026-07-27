package vml

import (
	"encoding/xml"
	"io"
	"reflect"
	"strings"
	"testing"
)

// namespaceWellFormed reports the first element or attribute written with a
// prefix that no in-scope declaration binds. Go's decoder accepts such input
// (it reports the bare prefix as the name's Space), so a closed
// Unmarshal(Marshal(x)) loop validated with Go's own decoder can never see it
// — which is exactly how C398's unbound w: prefixes stayed green. Every
// marshal assertion in this file goes through here.
func namespaceWellFormed(s string) error {
	d := xml.NewDecoder(strings.NewReader(s))
	declared := map[string]bool{"http://www.w3.org/XML/1998/namespace": true}
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		for _, a := range start.Attr {
			if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
				declared[a.Value] = true
			}
		}
		if start.Name.Space != "" && !declared[start.Name.Space] {
			return errUnbound(start.Name.Space + ":" + start.Name.Local)
		}
		for _, a := range start.Attr {
			if a.Name.Space == "" || a.Name.Space == "xmlns" {
				continue
			}
			if !declared[a.Name.Space] {
				return errUnbound(a.Name.Space + ":" + a.Name.Local)
			}
		}
	}
}

type errUnbound string

func (e errUnbound) Error() string { return "unbound namespace prefix on " + string(e) }

// rootName returns the marshaled fragment's root element name as the strict
// reader resolves it.
func rootName(t *testing.T, s string) xml.Name {
	t.Helper()
	d := xml.NewDecoder(strings.NewReader(s))
	for {
		tok, err := d.Token()
		if err != nil {
			t.Fatalf("no root element in %q: %v", s, err)
		}
		if start, ok := tok.(xml.StartElement); ok {
			return start.Name
		}
	}
}

// C398(a): only five types carried an XMLName, so xml.Marshal wrote the Go
// type name as the element name and never declared the VML namespace —
// <Textbox style="x"> is neither VML nor recognizable to any consumer.
func TestMarshalUsesVMLElementNames(t *testing.T) {
	const (
		nsVML   = "urn:schemas-microsoft-com:vml"
		nsOffic = "urn:schemas-microsoft-com:office:office"
		nsWord  = "urn:schemas-microsoft-com:office:word"
		nsExcel = "urn:schemas-microsoft-com:office:excel"
	)
	tests := []struct {
		v    interface{}
		want xml.Name
	}{
		{&Group{}, xml.Name{Space: nsVML, Local: "group"}},
		{&Shape{}, xml.Name{Space: nsVML, Local: "shape"}},
		{&Shapetype{}, xml.Name{Space: nsVML, Local: "shapetype"}},
		{&Rect{}, xml.Name{Space: nsVML, Local: "rect"}},
		{&RoundRect{}, xml.Name{Space: nsVML, Local: "roundrect"}},
		{&Oval{}, xml.Name{Space: nsVML, Local: "oval"}},
		{&Line{}, xml.Name{Space: nsVML, Local: "line"}},
		{&Polyline{}, xml.Name{Space: nsVML, Local: "polyline"}},
		{&Curve{}, xml.Name{Space: nsVML, Local: "curve"}},
		{&Arc{}, xml.Name{Space: nsVML, Local: "arc"}},
		{&ImageEl{}, xml.Name{Space: nsVML, Local: "image"}},
		{&Fill{}, xml.Name{Space: nsVML, Local: "fill"}},
		{&Stroke{}, xml.Name{Space: nsVML, Local: "stroke"}},
		{&Shadow{}, xml.Name{Space: nsVML, Local: "shadow"}},
		{&Textbox{}, xml.Name{Space: nsVML, Local: "textbox"}},
		{&TextPath{}, xml.Name{Space: nsVML, Local: "textpath"}},
		{&ImageData{}, xml.Name{Space: nsVML, Local: "imagedata"}},
		{&PathEl{}, xml.Name{Space: nsVML, Local: "path"}},
		{&Formulas{}, xml.Name{Space: nsVML, Local: "formulas"}},
		{&Formula{}, xml.Name{Space: nsVML, Local: "f"}},
		{&Handles{}, xml.Name{Space: nsVML, Local: "handles"}},
		{&Handle{}, xml.Name{Space: nsVML, Local: "h"}},
		{&Background{}, xml.Name{Space: nsVML, Local: "background"}},
		{&Lock{}, xml.Name{Space: nsOffic, Local: "lock"}},
		{&Callout{}, xml.Name{Space: nsOffic, Local: "callout"}},
		{&Extrusion{}, xml.Name{Space: nsOffic, Local: "extrusion"}},
		{&SignatureLine{}, xml.Name{Space: nsOffic, Local: "signatureline"}},
		{&Diagram{}, xml.Name{Space: nsOffic, Local: "diagram"}},
		{&ShapeLayout{}, xml.Name{Space: nsOffic, Local: "shapelayout"}},
		{&ShapeDefaults{}, xml.Name{Space: nsOffic, Local: "shapedefaults"}},
		{&OLEObject{}, xml.Name{Space: nsOffic, Local: "OLEObject"}},
		{&Wrap{}, xml.Name{Space: nsWord, Local: "wrap"}},
		{&AnchorLock{}, xml.Name{Space: nsWord, Local: "anchorlock"}},
		{&BorderTop{}, xml.Name{Space: nsWord, Local: "bordertop"}},
		{&BorderBottom{}, xml.Name{Space: nsWord, Local: "borderbottom"}},
		{&BorderLeft{}, xml.Name{Space: nsWord, Local: "borderleft"}},
		{&BorderRight{}, xml.Name{Space: nsWord, Local: "borderright"}},
		{&ClientData{}, xml.Name{Space: nsExcel, Local: "ClientData"}},
	}
	for _, tc := range tests {
		typ := reflect.TypeOf(tc.v).Elem().Name()
		t.Run(typ, func(t *testing.T) {
			out, err := xml.Marshal(tc.v)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if err := namespaceWellFormed(string(out)); err != nil {
				t.Fatalf("%v\nXML: %s", err, out)
			}
			if got := rootName(t, string(out)); got != tc.want {
				t.Errorf("root = {%s %s}, want {%s %s}\nXML: %s",
					got.Space, got.Local, tc.want.Space, tc.want.Local, out)
			}
		})
	}
}

// C398(b): the w: prefixes inside the innerxml capture of w:txbxContent were
// re-emitted verbatim while Go declared WordprocessingML as a *default*
// namespace, leaving every w: name unbound. xmllint exits 1 on this and
// ElementTree raises "unbound prefix"; Go's lenient decoder round-tripped it,
// which is why the tests passed.
func TestTextboxWithWordContentIsNamespaceWellFormed(t *testing.T) {
	input := `<textbox xmlns="urn:schemas-microsoft-com:vml" style="mso-fit-shape-to-text:t">` +
		`<w:txbxContent xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:p><w:r><w:t>hello</w:t></w:r></w:p>` +
		`</w:txbxContent></textbox>`
	var tb Textbox
	if err := xml.Unmarshal([]byte(input), &tb); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := xml.Marshal(&tb)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := namespaceWellFormed(string(out)); err != nil {
		t.Fatalf("%v\nXML: %s", err, out)
	}
	if !strings.Contains(string(out), "hello") {
		t.Errorf("text content lost: %s", out)
	}
}

// C398(c): a ##local child of a v:textbox is unqualified by definition. Once
// the textbox itself declares the VML namespace as the default, re-emitting
// the child bare silently moves it into the VML namespace.
func TestTextboxLocalChildStaysUnqualified(t *testing.T) {
	// The ##local case only arises when the textbox itself is prefixed, which
	// is how every real producer writes it: an unprefixed <div> under a
	// v:-prefixed parent is in no namespace at all.
	input := `<v:textbox xmlns:v="urn:schemas-microsoft-com:vml" id="tb1">` +
		`<div style="text-align:left">alpha</div></v:textbox>`
	var tb Textbox
	if err := xml.Unmarshal([]byte(input), &tb); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := xml.Marshal(&tb)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := namespaceWellFormed(string(out)); err != nil {
		t.Fatalf("%v\nXML: %s", err, out)
	}

	var back Textbox
	if err := xml.Unmarshal(out, &back); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if len(back.LocalContent) != 1 {
		t.Fatalf("LocalContent = %d, want 1\nXML: %s", len(back.LocalContent), out)
	}
	if got := back.LocalContent[0].Name; got.Space != "" || got.Local != "div" {
		t.Errorf("local child re-homed to {%s %s}, want unqualified div\nXML: %s",
			got.Space, got.Local, out)
	}
}

// C399 (re-opens C343): the presence-flag fix converted four fields and left
// the rest as string+omitempty, so <x:Visible/> — exactly how Excel marks an
// always-shown comment — was parsed to "" and dropped on the way out. This
// table enumerates every xsd:string child of CT_ClientData in vml-excel.xsd,
// so the class cannot be half-fixed a third time.
var clientDataStringFlags = []string{
	"MoveWithCells", "SizeWithCells", "Anchor", "Locked", "DefaultSize",
	"PrintObject", "Disabled", "AutoFill", "AutoLine", "AutoPict", "FmlaMacro",
	"TextHAlign", "TextVAlign", "LockText", "JustLastX", "SecretEdit",
	"Default", "Help", "Cancel", "Dismiss", "Visible", "RowHidden",
	"ColHidden", "MultiLine", "VScroll", "ValidIds", "FmlaRange", "NoThreeD2",
	"SelType", "MultiSel", "LCT", "ListItem", "DropStyle", "Colored",
	"FmlaLink", "FmlaPict", "NoThreeD", "FirstButton", "FmlaGroup", "MapOCX",
	"CF", "Camera", "RecalcAlways", "AutoScale", "DDE", "UIObj", "ScriptText",
	"ScriptExtended", "FmlaTxbx",
}

func TestClientDataPresenceFlagsAllSurvive(t *testing.T) {
	for _, flag := range clientDataStringFlags {
		t.Run(flag, func(t *testing.T) {
			input := `<ClientData xmlns="urn:schemas-microsoft-com:office:excel" ObjectType="Note">` +
				`<` + flag + `/></ClientData>`
			var cd ClientData
			if err := xml.Unmarshal([]byte(input), &cd); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			out, err := xml.Marshal(&cd)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if !hasElement(string(out), flag) {
				t.Errorf("presence flag %s dropped on re-marshal: %s", flag, out)
			}

			// Absent must stay absent.
			var empty ClientData
			bare := `<ClientData xmlns="urn:schemas-microsoft-com:office:excel" ObjectType="Note"/>`
			if err := xml.Unmarshal([]byte(bare), &empty); err != nil {
				t.Fatalf("unmarshal bare: %v", err)
			}
			eo, err := xml.Marshal(&empty)
			if err != nil {
				t.Fatalf("marshal bare: %v", err)
			}
			if strings.Contains(string(eo), flag) {
				t.Errorf("absent flag %s materialized: %s", flag, eo)
			}
		})
	}
}

// The real-world case the finding names: Excel's always-shown comment.
func TestClientDataExcelAlwaysVisibleNote(t *testing.T) {
	input := `<ClientData xmlns="urn:schemas-microsoft-com:office:excel" ObjectType="Note">` +
		`<MoveWithCells/><SizeWithCells/><Visible/><NoThreeD/><VScroll/>` +
		`<Anchor>1, 15, 0, 2, 3, 15, 3, 16</Anchor>` +
		`<Row>0</Row><Column>1</Column></ClientData>`
	var cd ClientData
	if err := xml.Unmarshal([]byte(input), &cd); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := xml.Marshal(&cd)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{"Visible", "NoThreeD", "VScroll", "MoveWithCells", "SizeWithCells"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("%s dropped: %s", want, out)
		}
	}
	if !strings.Contains(string(out), "1, 15, 0, 2, 3, 15, 3, 16") {
		t.Errorf("Anchor value lost: %s", out)
	}
}

// C467: a group modeled only shape children, so its o:lock, w10:wrap,
// x:ClientData, v:textbox, v:fill/stroke/shadow children were discarded.
func TestGroupKeepsNonShapeChildren(t *testing.T) {
	input := `<group xmlns="urn:schemas-microsoft-com:vml"` +
		` xmlns:o="urn:schemas-microsoft-com:office:office"` +
		` xmlns:w10="urn:schemas-microsoft-com:office:word"` +
		` xmlns:x="urn:schemas-microsoft-com:office:excel" id="g1" coordsize="100,100">` +
		`<fill on="t" color="#fff"/><stroke on="t"/><shadow on="t"/>` +
		`<shape id="s1"/>` +
		`<textbox id="tb"/>` +
		`<o:lock aspectratio="t"/>` +
		`<w10:wrap type="square"/><w10:anchorlock/>` +
		`<x:ClientData ObjectType="Group"/>` +
		`</group>`
	var g Group
	if err := xml.Unmarshal([]byte(input), &g); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if g.Fill == nil {
		t.Error("v:fill dropped")
	}
	if g.Stroke == nil {
		t.Error("v:stroke dropped")
	}
	if g.Shadow == nil {
		t.Error("v:shadow dropped")
	}
	if g.Textbox == nil {
		t.Error("v:textbox dropped")
	}
	if g.Lock == nil {
		t.Error("o:lock dropped")
	}
	if g.Wrap == nil {
		t.Error("w10:wrap dropped")
	}
	if g.AnchorLock == nil {
		t.Error("w10:anchorlock dropped")
	}
	if g.ClientData == nil {
		t.Error("x:ClientData dropped")
	}
	out, err := xml.Marshal(&g)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := namespaceWellFormed(string(out)); err != nil {
		t.Fatalf("%v\nXML: %s", err, out)
	}
}

// C467: a shape's o:gfxdata, o:bwmode and o:allowincell were observably
// dropped.
func TestShapeKeepsOfficeAttributes(t *testing.T) {
	input := `<shape xmlns="urn:schemas-microsoft-com:vml"` +
		` xmlns:o="urn:schemas-microsoft-com:office:office" id="s1"` +
		` alt="a picture" o:gfxdata="UEsDBBQ" o:bwmode="auto" o:allowincell="f"/>`
	var s Shape
	if err := xml.Unmarshal([]byte(input), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.Alt != "a picture" {
		t.Errorf("alt = %q, want %q", s.Alt, "a picture")
	}
	if s.OGfxData != "UEsDBBQ" {
		t.Errorf("o:gfxdata = %q, want UEsDBBQ", s.OGfxData)
	}
	if s.OBwMode != "auto" {
		t.Errorf("o:bwmode = %q, want auto", s.OBwMode)
	}
	if s.OAllowInCell != "f" {
		t.Errorf("o:allowincell = %q, want f", s.OAllowInCell)
	}
	out, err := xml.Marshal(&s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := namespaceWellFormed(string(out)); err != nil {
		t.Fatalf("%v\nXML: %s", err, out)
	}
	for _, want := range []string{"UEsDBBQ", "auto", "a picture"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("%q dropped on re-marshal: %s", want, out)
		}
	}
}

// C468 is documented, not fixed: this package normalizes child order to
// struct-field (schema) order, so it can never be byte-faithful. The test
// pins the documented behavior so the godoc cannot drift from it silently.
func TestShapeChildOrderIsNormalizedToSchemaOrder(t *testing.T) {
	input := `<shape xmlns="urn:schemas-microsoft-com:vml"` +
		` xmlns:x="urn:schemas-microsoft-com:office:excel" id="s1">` +
		`<fill on="t"/><shadow on="t"/><path v="m0,0"/><textbox id="tb"/>` +
		`<x:ClientData ObjectType="Note"/></shape>`
	var s Shape
	if err := xml.Unmarshal([]byte(input), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := xml.Marshal(&s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := childOrder(t, string(out))
	// Struct-field order is fill, stroke, shadow, textbox, ..., path,
	// ClientData; the source wrote no v:stroke, so it is absent here. Note
	// the source's path-before-textbox order is not what comes back.
	want := []string{"fill", "shadow", "textbox", "path", "ClientData"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("child order = %v, want %v (documented normalization)\nXML: %s", got, want, out)
	}
}

// childOrder returns the local names of the root element's direct children.
func childOrder(t *testing.T, s string) []string {
	t.Helper()
	d := xml.NewDecoder(strings.NewReader(s))
	depth := 0
	var out []string
	for {
		tok, err := d.Token()
		if err != nil {
			return out
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 {
				out = append(out, tok.Name.Local)
			}
		case xml.EndElement:
			depth--
		}
	}
}

// hasElement reports whether s contains an element with the given local name,
// in any form (self-closing, expanded, with or without an xmlns declaration).
func hasElement(s, local string) bool {
	d := xml.NewDecoder(strings.NewReader(s))
	for {
		tok, err := d.Token()
		if err != nil {
			return false
		}
		if start, ok := tok.(xml.StartElement); ok && start.Name.Local == local {
			return true
		}
	}
}
