package xlsx

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/mgilbir/spine/opc"
)

// buildXLSXWithControls rebuilds minimal.xlsx with a legacy form control (a
// checkbox, defined in a VML drawing with a linked cell and joined to a
// worksheet <control> block) and an ActiveX control (an ax:ocx part plus its
// persistence binary).
func buildXLSXWithControls(t *testing.T) []byte {
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

	const (
		ctVML      = "application/vnd.openxmlformats-officedocument.vmlDrawing"
		ctCtrlProp = "application/vnd.ms-excel.controlproperties+xml"
		ctActiveX  = "application/vnd.ms-office.activeX+xml"
		ctActiveXB = "application/vnd.ms-office.activeX"
	)

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
			overrides := `<Override PartName="/xl/drawings/vmlDrawing1.vml" ContentType="` + ctVML + `"/>` +
				`<Override PartName="/xl/ctrlProps/ctrlProp1.xml" ContentType="` + ctCtrlProp + `"/>` +
				`<Override PartName="/xl/activeX/activeX1.xml" ContentType="` + ctActiveX + `"/>` +
				`<Override PartName="/xl/activeX/activeX1.bin" ContentType="` + ctActiveXB + `"/>`
			s = strings.Replace(s, "</Types>", overrides+"</Types>", 1)
		case "xl/worksheets/sheet1.xml":
			ref := `<legacyDrawing r:id="rIdVml"/>` +
				`<controls><control r:id="rIdCtrl" shapeId="1025" name="Check Box 1"/></controls>`
			s = strings.Replace(s, "</worksheet>", ref+"</worksheet>", 1)
		}
		write(f.Name, []byte(s))
	}

	write("xl/worksheets/_rels/sheet1.xml.rels", []byte(
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+
			`<Relationship Id="rIdVml" Type="`+opc.RelTypeVMLDrawing+`" Target="../drawings/vmlDrawing1.vml"/>`+
			`<Relationship Id="rIdCtrl" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/ctrlProp" Target="../ctrlProps/ctrlProp1.xml"/>`+
			`<Relationship Id="rIdAx" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/control" Target="../activeX/activeX1.xml"/>`+
			`</Relationships>`))

	write("xl/drawings/vmlDrawing1.vml", []byte(
		`<xml xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office" xmlns:x="urn:schemas-microsoft-com:office:excel">`+
			`<v:shape id="_x0000_s1025" type="#_x0000_t201">`+
			`<x:ClientData ObjectType="Checkbox">`+
			`<x:Anchor>1, 0, 2, 0, 3, 0, 4, 0</x:Anchor>`+
			`<x:FmlaLink>$B$2</x:FmlaLink>`+
			`<x:Checked>1</x:Checked>`+
			`</x:ClientData>`+
			`</v:shape>`+
			`</xml>`))
	write("xl/ctrlProps/ctrlProp1.xml", []byte(
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
			`<formControlPr xmlns="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main" objectType="CheckBox" checked="Checked" lockText="1" fmlaLink="$B$2"/>`))

	write("xl/activeX/activeX1.xml", []byte(
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
			`<ax:ocx ax:classid="{8BD21D40-EC42-11CE-9E0D-00AA006002F3}" ax:persistence="persistPropertyBag" `+
			`xmlns:ax="http://schemas.microsoft.com/office/2006/activeX" r:id="rId1" `+
			`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`+
			`<ax:ocxPr ax:name="_Version" ax:value="393216"/></ax:ocx>`))
	write("xl/activeX/_rels/activeX1.xml.rels", []byte(
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/activeXControlBinary" Target="activeX1.bin"/>`+
			`</Relationships>`))
	write("xl/activeX/activeX1.bin", []byte("\x00\x01ACTIVEX-PERSISTENCE\x02\x03"))

	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func TestFormControlsExtract(t *testing.T) {
	pkg := buildXLSXWithControls(t)
	wb, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	sheets := wb.Sheets()
	if len(sheets) == 0 {
		t.Fatal("no sheets")
	}
	controls := sheets[0].FormControls()
	if len(controls) != 1 {
		t.Fatalf("FormControls() = %d, want 1", len(controls))
	}
	c := controls[0]
	if c.Type != FormControlCheckBox {
		t.Errorf("Type = %q, want checkbox", c.Type)
	}
	if c.LinkedCell != "$B$2" {
		t.Errorf("LinkedCell = %q, want $B$2", c.LinkedCell)
	}
	if !c.Checked {
		t.Errorf("Checked = false, want true")
	}
	if c.Name != "Check Box 1" {
		t.Errorf("Name = %q, want Check Box 1", c.Name)
	}
	if c.CtrlPropPart != "/xl/ctrlProps/ctrlProp1.xml" {
		t.Errorf("CtrlPropPart = %q", c.CtrlPropPart)
	}
	if c.VMLPart != "/xl/drawings/vmlDrawing1.vml" {
		t.Errorf("VMLPart = %q", c.VMLPart)
	}
}

func TestActiveXControlsExtract(t *testing.T) {
	pkg := buildXLSXWithControls(t)
	wb, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	ctrls := wb.ActiveXControls()
	if len(ctrls) != 1 {
		t.Fatalf("ActiveXControls() = %d, want 1", len(ctrls))
	}
	c := ctrls[0]
	if c.Name != "/xl/activeX/activeX1.xml" {
		t.Errorf("Name = %q", c.Name)
	}
	if c.ClassID != "{8BD21D40-EC42-11CE-9E0D-00AA006002F3}" {
		t.Errorf("ClassID = %q", c.ClassID)
	}
	if c.Persistence != "persistPropertyBag" {
		t.Errorf("Persistence = %q", c.Persistence)
	}
	if c.BinaryName != "/xl/activeX/activeX1.bin" {
		t.Errorf("BinaryName = %q", c.BinaryName)
	}
	if !bytes.Contains(c.BinaryData, []byte("ACTIVEX-PERSISTENCE")) {
		t.Errorf("BinaryData not preserved: %q", c.BinaryData)
	}
}

func TestControlsRoundTripPreserved(t *testing.T) {
	pkg := buildXLSXWithControls(t)
	wb, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	// The control parts must survive the save verbatim (zipEntry fatals if a
	// named part is absent).
	for _, name := range []string{
		"xl/drawings/vmlDrawing1.vml",
		"xl/ctrlProps/ctrlProp1.xml",
		"xl/activeX/activeX1.xml",
		"xl/activeX/activeX1.bin",
	} {
		_ = zipEntry(t, out, name)
	}
	// Reopen and re-enumerate to confirm the model is stable.
	wb2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if len(wb2.Sheets()[0].FormControls()) != 1 {
		t.Errorf("form control lost on round trip")
	}
	if len(wb2.ActiveXControls()) != 1 {
		t.Errorf("activeX control lost on round trip")
	}
}
