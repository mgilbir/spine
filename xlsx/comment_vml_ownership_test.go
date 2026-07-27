package xlsx

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/mgilbir/spine/opc"
)

// buildXLSXWithFormControl rebuilds minimal.xlsx so its first sheet carries a
// legacy form-control (a checkbox) whose shape lives in a shared legacy VML
// drawing, with the matching worksheet <controls>/<control> block, its ctrlProps
// part, and the relationships tying them together.
func buildXLSXWithFormControl(t *testing.T) []byte {
	t.Helper()
	src, err := os.ReadFile("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("read minimal.xlsx: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("zip open: %v", err)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	write := func(name string, data []byte) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	const ctrlProps = "application/vnd.ms-excel.controlproperties+xml"
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		content := new(bytes.Buffer)
		if _, err := content.ReadFrom(rc); err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		_ = rc.Close()
		s := content.String()

		switch f.Name {
		case "[Content_Types].xml":
			s = strings.Replace(s, "</Types>",
				`<Default Extension="vml" ContentType="application/vnd.openxmlformats-officedocument.vmlDrawing"/>`+
					`<Override PartName="/xl/ctrlProps/ctrlProp1.xml" ContentType="`+ctrlProps+`"/></Types>`, 1)
		case "xl/worksheets/sheet1.xml":
			block := `<legacyDrawing r:id="rId200"/>` +
				`<controls><control shapeId="1025" r:id="rId201" name="Check Box 1"/></controls>`
			s = strings.Replace(s, "</worksheet>", block+"</worksheet>", 1)
		}
		write(f.Name, []byte(s))
	}

	write("xl/worksheets/_rels/sheet1.xml.rels", []byte(
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+
			`<Relationship Id="rId200" Type="`+opc.RelTypeVMLDrawing+`" Target="../drawings/vmlDrawing1.vml"/>`+
			`<Relationship Id="rId201" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/ctrlProp" Target="../ctrlProps/ctrlProp1.xml"/>`+
			`</Relationships>`))
	write("xl/drawings/vmlDrawing1.vml", []byte(controlVML))
	write("xl/ctrlProps/ctrlProp1.xml", []byte(
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
			`<formControlPr xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" objectType="CheckBox" checked="Checked" lockText="1" noThreeD="1"/>`))

	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

const controlVML = `<xml xmlns:v="urn:schemas-microsoft-com:vml"
 xmlns:o="urn:schemas-microsoft-com:office:office"
 xmlns:x="urn:schemas-microsoft-com:office:excel">
 <o:shapelayout v:ext="edit">
  <o:idmap v:ext="edit" data="1"/>
 </o:shapelayout><v:shapetype id="_x0000_t201" coordsize="21600,21600" o:spt="201"
  path="m0,0l0,21600,21600,21600,21600,0xe">
  <v:stroke joinstyle="miter"/>
  <v:path o:connecttype="rect"/>
 </v:shapetype><v:shape id="_x0000_s1025" type="#_x0000_t201" style='position:absolute;
  margin-left:48pt;margin-top:15pt;width:96pt;height:24pt' o:button="t"
  fillcolor="window [65]" strokecolor="windowText [64]" o:insetmode="auto">
  <v:fill color2="buttonFace [67]"/>
  <v:stroke color="windowText [64]"/>
  <x:ClientData ObjectType="Checkbox">
   <x:MoveWithCells/>
   <x:SizeWithCells/>
   <x:Anchor>1, 0, 1, 0, 3, 0, 4, 0</x:Anchor>
   <x:AutoFill>False</x:AutoFill>
   <x:FmlaLink>$B$2</x:FmlaLink>
   <x:Checked>1</x:Checked>
  </x:ClientData>
 </v:shape></xml>
`

// TestAddCommentPreservesFormControl is the C245 regression: adding a comment to
// a sheet that already carries a form-control VML shape must not destroy the
// control shape, and the comment note must be given a shape id distinct from
// (and above) the control's so the worksheet <control shapeId="1025"> keeps
// resolving to the control, not the note.
func TestAddCommentPreservesFormControl(t *testing.T) {
	pkg := buildXLSXWithFormControl(t)

	wb, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	s, _ := wb.Sheet(0)

	// Sanity: the fixture is a real form control before we touch it.
	if fcs := s.FormControls(); len(fcs) != 1 || fcs[0].Type != FormControlCheckBox {
		t.Fatalf("fixture form controls = %+v, want one checkbox", fcs)
	}

	s.AddComment("D4", "Reviewer", "please verify")

	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	vml := string(readZipPart(t, out, "xl/drawings/vmlDrawing1.vml"))
	// The control shape survives verbatim with its original id and type.
	if !strings.Contains(vml, `id="_x0000_s1025"`) || !strings.Contains(vml, `ObjectType="Checkbox"`) {
		t.Errorf("control shape lost from regenerated VML:\n%s", vml)
	}
	// The comment note is a Note shape with a distinct id above the control's.
	if !strings.Contains(vml, `ObjectType="Note"`) {
		t.Errorf("comment note shape missing from VML:\n%s", vml)
	}
	if strings.Count(vml, `type="#_x0000_t202"`) != 1 || !strings.Contains(vml, `id="_x0000_s1026"`) {
		t.Errorf("note shape did not get a distinct id (want _x0000_s1026):\n%s", vml)
	}
	// The Note shape must not have stolen the control's shape id 1025.
	noteIdx := strings.Index(vml, `ObjectType="Note"`)
	if i := strings.LastIndex(vml[:noteIdx], `id="_x0000_s`); i >= 0 {
		if strings.HasPrefix(vml[i:], `id="_x0000_s1025"`) {
			t.Errorf("note shape cross-wired onto control shape id 1025:\n%s", vml)
		}
	}

	// The worksheet still declares the control and its legacy drawing.
	ws := string(readZipPart(t, out, "xl/worksheets/sheet1.xml"))
	if !strings.Contains(ws, `shapeId="1025"`) || !strings.Contains(ws, `<legacyDrawing`) {
		t.Errorf("worksheet lost <control>/<legacyDrawing>:\n%s", ws)
	}

	// Reopen: the control still resolves as a checkbox and the comment is present.
	wb2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	s2, _ := wb2.Sheet(0)
	var checkbox *FormControl
	for i := range s2.FormControls() {
		if fc := s2.FormControls()[i]; fc.Type == FormControlCheckBox {
			checkbox = &fc
		}
	}
	if checkbox == nil || checkbox.Name != "Check Box 1" || checkbox.LinkedCell != "$B$2" {
		t.Errorf("after round trip checkbox control = %+v, want name=Check Box 1 link=$B$2", checkbox)
	}
	if got := len(s2.Comments()); got != 1 {
		t.Errorf("comment count after round trip = %d, want 1", got)
	}
}

// TestAddOLEThenCommentSingleLegacyVML is the C283 regression: adding an OLE
// object and then a comment to the same sheet must produce ONE coherent legacy
// VML drawing (referenced by a single <legacyDrawing>) holding both the OLE
// Pict shape and the comment Note shape with distinct shape ids, rather than two
// competing VML parts where the note boxes never render.
func TestAddOLEThenCommentSingleLegacyVML(t *testing.T) {
	w := Create()
	s := addSheetT(w, "Sheet1")
	if _, err := s.Cell("A1"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddOLEObject(OLEObjectSpec{Data: []byte("\xd0\xcf\x11\xe0 payload"), ProgID: "Word.Document.12", Anchor: "B2"}); err != nil {
		t.Fatalf("AddOLEObject: %v", err)
	}
	s.AddComment("D4", "Reviewer", "please verify")

	out, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	// Exactly one VML drawing part.
	zr, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatal(err)
	}
	var vmlParts []string
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, ".vml") {
			vmlParts = append(vmlParts, f.Name)
		}
	}
	if len(vmlParts) != 1 {
		t.Fatalf("want exactly one VML part, got %v", vmlParts)
	}

	vml := string(readZipPart(t, out, vmlParts[0]))
	if !strings.Contains(vml, `ObjectType="Pict"`) {
		t.Errorf("combined VML missing OLE Pict shape:\n%s", vml)
	}
	if !strings.Contains(vml, `ObjectType="Note"`) {
		t.Errorf("combined VML missing comment Note shape:\n%s", vml)
	}
	// OLE shape uses 1025; the note must use a distinct id above it.
	if !strings.Contains(vml, `id="_x0000_s1025"`) || !strings.Contains(vml, `id="_x0000_s1026"`) {
		t.Errorf("combined VML shape ids not distinct (want 1025 and 1026):\n%s", vml)
	}

	// The worksheet has a single <legacyDrawing> plus the oleObjects reference.
	ws := string(readZipPart(t, out, "xl/worksheets/sheet1.xml"))
	if strings.Count(ws, "<legacyDrawing") != 1 {
		t.Errorf("want exactly one <legacyDrawing>, ws:\n%s", ws)
	}
	if !strings.Contains(ws, "<oleObjects>") {
		t.Errorf("worksheet missing <oleObjects>:\n%s", ws)
	}

	// The single <legacyDrawing> resolves to the one VML part.
	rels := string(readZipPart(t, out, "xl/worksheets/_rels/sheet1.xml.rels"))
	if strings.Count(rels, "vmlDrawing") == 0 {
		t.Errorf("sheet rels missing VML relationship:\n%s", rels)
	}

	// Round-trips: the OLE object and the comment both survive reopen.
	rw, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if objs := rw.OLEObjects(); len(objs) != 1 {
		t.Errorf("OLEObjects after round trip = %d, want 1", len(objs))
	}
	s2, _ := rw.Sheet(0)
	if got := len(s2.Comments()); got != 1 {
		t.Errorf("comment count after round trip = %d, want 1", got)
	}
}

// TestAddCommentRefValidation is the C288 regression: AddComment must canonicalize
// its cell ref and replace an existing comment on the same cell (no duplicates),
// and must reject a syntactically invalid ref rather than emit a ref-less note.
func TestAddCommentRefValidation(t *testing.T) {
	w := Create()
	s := addSheetT(w, "Sheet1")
	if _, err := s.Cell("A1"); err != nil {
		t.Fatal(err)
	}

	// A case-variant duplicate must collapse to one comment on A1.
	if c := s.AddComment("a1", "First", "one"); c == nil || c.Ref() != "A1" {
		t.Fatalf("AddComment(a1) = %+v, want canonical ref A1", c)
	}
	if c := s.AddComment("A1", "Second", "two"); c == nil || c.Ref() != "A1" {
		t.Fatalf("AddComment(A1) = %+v", c)
	}

	// An invalid ref is rejected (nil), not accepted as a ref-less note.
	if c := s.AddComment("NOTACELL", "Third", "bad"); c != nil {
		t.Errorf("AddComment(NOTACELL) = %+v, want nil", c)
	}

	if got := len(s.Comments()); got != 1 {
		t.Fatalf("comment count = %d, want 1 (duplicate replaced, invalid rejected)", got)
	}

	out, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	legacy := string(readZipPart(t, out, "xl/comments1.xml"))
	if n := strings.Count(legacy, `ref="A1"`); n != 1 {
		t.Errorf("legacy comments has %d <comment ref=\"A1\">, want 1:\n%s", n, legacy)
	}
	threaded := string(readZipPart(t, out, "xl/threadedComments/threadedComment1.xml"))
	if n := strings.Count(threaded, `ref="A1"`); n != 1 {
		t.Errorf("threaded comments has %d ref=\"A1\", want 1:\n%s", n, threaded)
	}

	// Reopen: exactly one comment, and Cell.Comment resolves it case-insensitively.
	rw, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	s2, _ := rw.Sheet(0)
	if got := len(s2.Comments()); got != 1 {
		t.Errorf("comment count after round trip = %d, want 1", got)
	}
	cell, _ := s2.Cell("A1")
	if cm := cell.Comment(); cm == nil || cm.Text() != "two" {
		t.Errorf("Cell(A1).Comment() = %+v, want the replacement text \"two\"", cm)
	}
}
