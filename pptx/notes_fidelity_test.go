package pptx

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// replaceZipPart rewrites one entry of an OPC package, leaving every other
// entry byte-identical.
func replaceZipPart(t *testing.T, data []byte, name string, content []byte) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	found := false
	for _, f := range zr.File {
		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatalf("create %s: %v", f.Name, err)
		}
		if f.Name == name {
			found = true
			if _, err := w.Write(content); err != nil {
				t.Fatalf("write %s: %v", name, err)
			}
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		if _, err := io.Copy(w, rc); err != nil {
			t.Fatalf("copy %s: %v", f.Name, err)
		}
		_ = rc.Close()
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	if !found {
		t.Fatalf("part %q not present in package", name)
	}
	return buf.Bytes()
}

// notesWithAlternateContent is the shape PowerPoint writes for guarded content
// (ink, 2010+ effects) in a notes slide: mc: is declared on the part root and
// used by an mc:AlternateContent deep inside the shape tree, and mc:Ignorable
// lists it.
const notesWithAlternateContent = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
	`<p:notes xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"` +
	` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"` +
	` xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"` +
	` xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"` +
	` xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main"` +
	` mc:Ignorable="p14" showMasterSp="0">` +
	`<p:cSld><p:spTree>` +
	`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
	`<p:grpSpPr/>` +
	`<p:sp><p:nvSpPr><p:cNvPr id="2" name="Notes Placeholder"/><p:cNvSpPr/>` +
	`<p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr><p:spPr/>` +
	`<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>original</a:t></a:r></a:p></p:txBody></p:sp>` +
	`<mc:AlternateContent><mc:Choice Requires="p14"><p:sp><p:nvSpPr>` +
	`<p:cNvPr id="3" name="Ink"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr/></p:sp></mc:Choice>` +
	`<mc:Fallback><p:sp><p:nvSpPr><p:cNvPr id="3" name="Ink"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>` +
	`<p:spPr/></p:sp></mc:Fallback></mc:AlternateContent>` +
	`</p:spTree></p:cSld>` +
	`<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>` +
	`</p:notes>`

// C421: SetNotes regenerates the notes part. The regenerated root used to carry
// only the a/r/p declarations, so an mc:AlternateContent in the notes shape tree
// was re-emitted with the mc: prefix unbound anywhere in the part —
// namespace-invalid XML — and mc:Ignorable plus every extra root declaration was
// dropped.
func TestSetNotes_PreservesNotesRootDeclarations(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.SetNotes("original")
	base, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	const notesPart = "ppt/notesSlides/notesSlide1.xml"
	seeded := replaceZipPart(t, base, notesPart, []byte(notesWithAlternateContent))

	reopened := openBytes(t, seeded)
	slides := reopened.Slides()
	if len(slides) != 1 {
		t.Fatalf("reopened deck has %d slides, want 1", len(slides))
	}
	if got := slides[0].Notes(); got != "original" {
		t.Fatalf("seeded notes text = %q", got)
	}
	slides[0].SetNotes("updated")

	out, err := reopened.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	got := string(zipPart(t, out, notesPart))

	if strings.Contains(got, "mc:AlternateContent") &&
		!strings.Contains(got, `xmlns:mc="`+xmlb.NSMarkupCompatibility+`"`) {
		t.Errorf("regenerated notes part uses the mc: prefix with no declaration:\n%s", got)
	}
	if !strings.Contains(got, `mc:Ignorable="p14"`) {
		t.Errorf("mc:Ignorable dropped from the regenerated notes root:\n%s", got)
	}
	if !strings.Contains(got, `xmlns:p14="`+xmlb.NSPowerPoint2010+`"`) {
		t.Errorf("extra root declaration dropped from the regenerated notes root:\n%s", got)
	}
	if !strings.Contains(got, `showMasterSp="0"`) {
		t.Errorf("explicit showMasterSp=0 lost:\n%s", got)
	}
	if !strings.Contains(got, "updated") {
		t.Errorf("SetNotes did not take effect:\n%s", got)
	}

	// The result must still be parseable as XML with every prefix bound.
	if err := xmlb.UnmarshalWithSource([]byte(got), &struct{}{}); err != nil {
		t.Errorf("regenerated notes part is not namespace-valid: %v\n%s", err, got)
	}
}

// C381: clearing embedTrueTypeFonts / saveSubsetFonts must win over the value
// the source deck carried. Emitting only the true case left the modeled
// attribute list without the flag, so ReplayCapturedAttrs re-emitted the
// captured embedTrueTypeFonts="1" verbatim and SetEmbedTrueTypeFonts(false) was
// a silent no-op. saveSubsetFonts has no public setter yet; it is driven through
// the model here so both halves of the emission rule are covered.
func TestSetFontFlags_ClearsAValueTheSourceCarried(t *testing.T) {
	yes, no := true, false
	p := openBytes(t, savedDeck(t))
	p.SetEmbedTrueTypeFonts(true)
	p.presentation.SaveSubsetFonts = &yes
	enabled, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if pres := string(zipPart(t, enabled, "ppt/presentation.xml")); !strings.Contains(pres, `embedTrueTypeFonts="1"`) ||
		!strings.Contains(pres, `saveSubsetFonts="1"`) {
		t.Fatalf("setup: flags not enabled:\n%s", pres)
	}

	cleared := openBytes(t, enabled)
	if !cleared.EmbedTrueTypeFonts() {
		t.Fatal("setup: reopened deck does not report the flag")
	}
	cleared.SetEmbedTrueTypeFonts(false)
	cleared.presentation.SaveSubsetFonts = &no
	out, err := cleared.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	pres := string(zipPart(t, out, "ppt/presentation.xml"))
	if strings.Contains(pres, `embedTrueTypeFonts="1"`) {
		t.Errorf("SetEmbedTrueTypeFonts(false) was a no-op on a parsed deck:\n%s", pres)
	}
	if strings.Contains(pres, `saveSubsetFonts="1"`) {
		t.Errorf("SetSaveSubsetFonts(false) was a no-op on a parsed deck:\n%s", pres)
	}
	if openBytes(t, out).EmbedTrueTypeFonts() {
		t.Error("reopened deck still reports embedTrueTypeFonts")
	}
}
