package docx

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"
)

// buildDocxWithActiveX rebuilds minimal.docx with an ActiveX control: an ax:ocx
// part, its persistence binary, a control relationship from the document, and a
// w:control reference in the body.
func buildDocxWithActiveX(t *testing.T) []byte {
	t.Helper()
	src, err := os.ReadFile("testdata/minimal.docx")
	if err != nil {
		t.Fatalf("read minimal.docx: %v", err)
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
		relControl = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/control"
		relAxBin   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/activeXControlBinary"
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
			s = strings.Replace(s, "</Types>",
				`<Override PartName="/word/activeX/activeX1.xml" ContentType="`+contentTypeActiveXXML+`"/>`+
					`<Override PartName="/word/activeX/activeX1.bin" ContentType="`+contentTypeActiveXBin+`"/></Types>`, 1)
		case "word/_rels/document.xml.rels":
			s = strings.Replace(s, "</Relationships>",
				`<Relationship Id="rId100" Type="`+relControl+`" Target="activeX/activeX1.xml"/></Relationships>`, 1)
		case "word/document.xml":
			ref := `<w:p><w:r><w:object><w:control r:id="rId100" w:name="CheckBox1" w:shapeid="_x0000_i1025"/></w:object></w:r></w:p>`
			s = strings.Replace(s, "</w:body>", ref+"</w:body>", 1)
		}
		write(f.Name, []byte(s))
	}

	write("word/activeX/activeX1.xml", []byte(
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
			`<ax:ocx ax:classid="{8BD21D40-EC42-11CE-9E0D-00AA006002F3}" ax:persistence="persistPropertyBag" `+
			`xmlns:ax="http://schemas.microsoft.com/office/2006/activeX" r:id="rId1" `+
			`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`+
			`<ax:ocxPr ax:name="_Version" ax:value="393216"/></ax:ocx>`))
	write("word/activeX/_rels/activeX1.xml.rels", []byte(
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+
			`<Relationship Id="rId1" Type="`+relAxBin+`" Target="activeX1.bin"/>`+
			`</Relationships>`))
	write("word/activeX/activeX1.bin", []byte("\x00\x01ACTIVEX-PERSISTENCE\x02\x03"))

	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func TestActiveXControlsExtract(t *testing.T) {
	pkg := buildDocxWithActiveX(t)
	doc, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	ctrls := doc.ActiveXControls()
	if len(ctrls) != 1 {
		t.Fatalf("ActiveXControls() = %d, want 1", len(ctrls))
	}
	c := ctrls[0]
	if c.Name != "/word/activeX/activeX1.xml" {
		t.Errorf("Name = %q", c.Name)
	}
	if c.ClassID != "{8BD21D40-EC42-11CE-9E0D-00AA006002F3}" {
		t.Errorf("ClassID = %q", c.ClassID)
	}
	if c.Persistence != "persistPropertyBag" {
		t.Errorf("Persistence = %q", c.Persistence)
	}
	if c.BinaryName != "/word/activeX/activeX1.bin" {
		t.Errorf("BinaryName = %q", c.BinaryName)
	}
	if !bytes.Contains(c.BinaryData, []byte("ACTIVEX-PERSISTENCE")) {
		t.Errorf("BinaryData not preserved: %q", c.BinaryData)
	}

	// The control parts must survive a save verbatim.
	out, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	doc2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if len(doc2.ActiveXControls()) != 1 {
		t.Errorf("activeX control lost on round trip")
	}
}

func TestActiveXControlsNoneOnPlainDoc(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("hello")
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	doc2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if ctrls := doc2.ActiveXControls(); len(ctrls) != 0 {
		t.Errorf("plain document returned %d ActiveX controls", len(ctrls))
	}
}
