package pptx

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/mgilbir/spine/opc"
)

// inkPartXML is a minimal InkML content part (a single one-point trace). The
// bytes are carried verbatim; the geometry is never parsed.
const inkPartXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<inkml:ink xmlns:inkml="http://www.w3.org/2003/InkML">` +
	`<inkml:definitions><inkml:brush xml:id="br0">` +
	`<inkml:brushProperty name="color" value="#FF0000"/></inkml:brush></inkml:definitions>` +
	`<inkml:trace contextRef="#ctx0" brushRef="#br0">0 0 10</inkml:trace>` +
	`</inkml:ink>`

// contentPartFragment is an mc:AlternateContent carrying a p14:contentPart (the
// ink reference) with a picture fallback, as PowerPoint writes ink into a slide.
const contentPartFragment = `<mc:AlternateContent xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006">` +
	`<mc:Choice xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main" Requires="p14">` +
	`<p:contentPart p14:bwMode="auto" r:id="rId2">` +
	`<p14:nvContentPartPr><p14:cNvPr id="8" name="Ink 7"/><p14:cNvContentPartPr/><p14:nvPr/></p14:nvContentPartPr>` +
	`<p14:xfrm><a:off x="1561526" y="971040"/><a:ext cx="3210480" cy="1010160"/></p14:xfrm>` +
	`</p:contentPart></mc:Choice>` +
	`<mc:Fallback><p:sp><p:nvSpPr><p:cNvPr id="8" name="Ink 7"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>` +
	`<p:spPr/></p:sp></mc:Fallback></mc:AlternateContent>`

// model3DFragment is a graphicFrame holding an am3d:model3D that embeds the glb
// model part via r:embed.
const model3DFragment = `<p:graphicFrame><p:nvGraphicFramePr>` +
	`<p:cNvPr id="9" name="3D Model 8"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr>` +
	`<p:xfrm><a:off x="0" y="0"/><a:ext cx="1000000" cy="1000000"/></p:xfrm>` +
	`<a:graphic><a:graphicData uri="http://schemas.microsoft.com/office/drawing/2017/model3d">` +
	`<am3d:model3D xmlns:am3d="http://schemas.microsoft.com/office/drawing/2017/model3d" r:embed="rId3"/>` +
	`</a:graphicData></a:graphic></p:graphicFrame>`

// buildPPTXWithInkAndModel rebuilds test.pptx with an ink (InkML) part and a 3D
// model (glb) part referenced from slide1, plus the matching content types,
// relationships, and shape-tree references.
func buildPPTXWithInkAndModel(t *testing.T, glbData []byte) []byte {
	t.Helper()
	src, err := os.ReadFile("testdata/test.pptx")
	if err != nil {
		t.Fatalf("read test.pptx: %v", err)
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
				`<Default Extension="glb" ContentType="`+opc.ContentTypeModel3D+`"/>`+
					`<Override PartName="/ppt/ink/ink1.xml" ContentType="`+opc.ContentTypeInk+`"/></Types>`, 1)
		case "ppt/slides/_rels/slide1.xml.rels":
			s = strings.Replace(s, "</Relationships>",
				`<Relationship Id="rId2" Type="`+opc.RelTypeCustomXML+`" Target="../ink/ink1.xml"/>`+
					`<Relationship Id="rId3" Type="http://schemas.microsoft.com/office/2017/06/relationships/model3d" Target="../media/model1.glb"/></Relationships>`, 1)
		case "ppt/slides/slide1.xml":
			s = strings.Replace(s, "</p:spTree>", contentPartFragment+model3DFragment+"</p:spTree>", 1)
		}
		write(f.Name, []byte(s))
	}
	write("ppt/ink/ink1.xml", []byte(inkPartXML))
	write("ppt/media/model1.glb", glbData)

	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func TestSlideInkAnnotationsExtract(t *testing.T) {
	pkg := buildPPTXWithInkAndModel(t, []byte("glTF\x02\x00\x00\x00 fake glb"))
	pres, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	slides := pres.Slides()
	if len(slides) == 0 {
		t.Fatal("no slides")
	}
	inks := slides[0].InkAnnotations()
	if len(inks) != 1 {
		t.Fatalf("InkAnnotations = %d, want 1", len(inks))
	}
	got := inks[0]
	if got.PartName != "/ppt/ink/ink1.xml" {
		t.Errorf("PartName = %q", got.PartName)
	}
	if got.ContentType != opc.ContentTypeInk {
		t.Errorf("ContentType = %q", got.ContentType)
	}
	if got.RelID != "rId2" {
		t.Errorf("RelID = %q", got.RelID)
	}
	if string(got.Data) != inkPartXML {
		t.Errorf("Data mismatch: got %q", got.Data)
	}
	// Presentation-level aggregate reports the same ink.
	if all := pres.InkAnnotations(); len(all) != 1 {
		t.Errorf("Presentation.InkAnnotations = %d, want 1", len(all))
	}
}

func TestSlideModel3DExtract(t *testing.T) {
	glb := []byte("glTF\x02\x00\x00\x00 fake glb payload")
	pkg := buildPPTXWithInkAndModel(t, glb)
	pres, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	models := pres.Slides()[0].Model3D()
	if len(models) != 1 {
		t.Fatalf("Model3D = %d, want 1", len(models))
	}
	got := models[0]
	if got.PartName != "/ppt/media/model1.glb" {
		t.Errorf("PartName = %q", got.PartName)
	}
	if got.ContentType != opc.ContentTypeModel3D {
		t.Errorf("ContentType = %q", got.ContentType)
	}
	if got.RelID != "rId3" {
		t.Errorf("RelID = %q", got.RelID)
	}
	if !bytes.Equal(got.Data, glb) {
		t.Errorf("Data mismatch: got %q", got.Data)
	}
	if all := pres.Model3D(); len(all) != 1 {
		t.Errorf("Presentation.Model3D = %d, want 1", len(all))
	}
}

// TestInkAndModel3DRoundTrip confirms the ink and 3D model parts survive a
// save/reopen unchanged and that the referencing shape-tree elements are
// preserved verbatim in the re-serialized slide.
func TestInkAndModel3DRoundTrip(t *testing.T) {
	glb := []byte("glTF\x02\x00\x00\x00 fake glb payload for round trip")
	pkg := buildPPTXWithInkAndModel(t, glb)
	pres, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	out, err := pres.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	// The re-serialized slide still carries the contentPart and model3D refs.
	zr, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen zip: %v", err)
	}
	slideXML := readZipEntry(t, zr, "ppt/slides/slide1.xml")
	if !strings.Contains(slideXML, `<p:contentPart p14:bwMode="auto" r:id="rId2">`) {
		t.Errorf("contentPart (ink) not preserved in slide:\n%s", slideXML)
	}
	if !strings.Contains(slideXML, `r:embed="rId3"`) {
		t.Errorf("model3D reference not preserved in slide:\n%s", slideXML)
	}

	// The ink and model part bytes are byte-identical after the round trip.
	if got := readZipEntry(t, zr, "ppt/ink/ink1.xml"); got != inkPartXML {
		t.Errorf("ink part bytes changed:\n%s", got)
	}
	if got := readZipEntryBytes(t, zr, "ppt/media/model1.glb"); !bytes.Equal(got, glb) {
		t.Errorf("model part bytes changed: got %q", got)
	}

	// Extraction on the reopened deck yields the same handles.
	repres, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if inks := repres.Slides()[0].InkAnnotations(); len(inks) != 1 || string(inks[0].Data) != inkPartXML {
		t.Errorf("ink not re-extractable after round trip: %+v", inks)
	}
	if ms := repres.Slides()[0].Model3D(); len(ms) != 1 || !bytes.Equal(ms[0].Data, glb) {
		t.Errorf("model not re-extractable after round trip: %+v", ms)
	}
}

func TestInkAndModel3DNoneOnPlainDeck(t *testing.T) {
	pres, err := Open("testdata/test.pptx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for _, s := range pres.Slides() {
		if got := s.InkAnnotations(); len(got) != 0 {
			t.Errorf("plain slide returned %d ink annotations", len(got))
		}
		if got := s.Model3D(); len(got) != 0 {
			t.Errorf("plain slide returned %d 3D models", len(got))
		}
	}
}

func readZipEntry(t *testing.T, zr *zip.Reader, name string) string {
	t.Helper()
	return string(readZipEntryBytes(t, zr, name))
}

func readZipEntryBytes(t *testing.T, zr *zip.Reader, name string) []byte {
	t.Helper()
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open %s: %v", name, err)
			}
			defer func() { _ = rc.Close() }()
			var b bytes.Buffer
			if _, err := b.ReadFrom(rc); err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			return b.Bytes()
		}
	}
	t.Fatalf("entry %s not found", name)
	return nil
}
