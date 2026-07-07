package pptx

import (
	"archive/zip"
	"bytes"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// grpSpFragment is a minimal group shape with a nested member shape; ids 9
// and 10 sit above anything the generated deck uses.
const grpSpFragment = `<p:grpSp><p:nvGrpSpPr><p:cNvPr id="9" name="Group 9"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/><p:sp><p:nvSpPr><p:cNvPr id="10" name="Grouped 10"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr/></p:sp></p:grpSp>`

// deckWithGroupShape builds a deck via the API and splices a group shape into
// the slide XML — group shapes cannot be created through the API, but decks
// in the wild contain them.
func deckWithGroupShape(t *testing.T) []byte {
	t.Helper()
	p := Create()
	slide := p.AddSlide()
	box := slide.AddTextBox()
	box.TextFrame().SetText("prototype")
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	return rewriteZipPart(t, data, "ppt/slides/slide1.xml", func(xml []byte) []byte {
		if !bytes.Contains(xml, []byte("</p:spTree>")) {
			t.Fatal("slide1.xml has no spTree close tag")
		}
		return bytes.Replace(xml, []byte("</p:spTree>"), []byte(grpSpFragment+"</p:spTree>"), 1)
	})
}

func rewriteZipPart(t *testing.T, data []byte, name string, rewrite func([]byte) []byte) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for _, file := range reader.File {
		src, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(src)
		_ = src.Close()
		if err != nil {
			t.Fatal(err)
		}
		if file.Name == name {
			content = rewrite(content)
		}
		dst, err := writer.Create(file.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := dst.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func zipPart(t *testing.T, data []byte, name string) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		src, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = src.Close() }()
		content, err := io.ReadAll(src)
		if err != nil {
			t.Fatal(err)
		}
		return content
	}
	t.Fatalf("part %s not found", name)
	return nil
}

// TestAddShapeToLoadedSlidePreservesUnmodeledContent is the append-only sync
// proof: cloning a prototype shape onto a loaded slide must not drop content
// the domain model cannot represent (the group shape), must not disturb the
// parsed shapes, and must assign the clone an id above everything present.
func TestAddShapeToLoadedSlidePreservesUnmodeledContent(t *testing.T) {
	deck := deckWithGroupShape(t)

	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	slide := p.Slides()[0]

	var prototype *TextBox
	for _, shape := range slide.Shapes() {
		if tb, ok := shape.(*TextBox); ok {
			prototype = tb
			break
		}
	}
	if prototype == nil {
		t.Fatal("prototype text box not materialized")
	}

	clone := CloneShape(prototype)
	if clone == nil {
		t.Fatal("CloneShape returned nil")
	}
	clone.(*TextBox).TextFrame().SetText("cloned")
	clone.SetPosition(dml.EMU(914400), dml.EMU(914400))
	slide.AddShape(clone)

	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := string(zipPart(t, saved, "ppt/slides/slide1.xml"))

	if !strings.Contains(slideXML, "grpSp") {
		t.Fatal("group shape was dropped by the save")
	}
	if !strings.Contains(slideXML, "prototype") || !strings.Contains(slideXML, "cloned") {
		t.Fatal("expected both the prototype and the clone in the slide XML")
	}

	// Every cNvPr id on the slide must be unique, and the clone's id must sit
	// above the pre-existing maximum (10, inside the group).
	ids := regexp.MustCompile(`cNvPr id="(\d+)"`).FindAllStringSubmatch(slideXML, -1)
	seen := map[string]bool{}
	maxSeen := 0
	for _, m := range ids {
		if seen[m[1]] {
			t.Fatalf("duplicate shape id %s in slide XML", m[1])
		}
		seen[m[1]] = true
		if n := len(m[1]); n > 0 {
			var v int
			for _, r := range m[1] {
				v = v*10 + int(r-'0')
			}
			if v > maxSeen {
				maxSeen = v
			}
		}
	}
	if maxSeen < 11 {
		t.Fatalf("expected the clone to get id 11, max id seen %d", maxSeen)
	}

	// The deck must still round-trip: the group survives a reload too.
	reloaded, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(reloaded.Slides()[0].Shapes()); got != 3 {
		t.Fatalf("expected 3 shapes after reload (prototype, group, clone), got %d", got)
	}
}

// TestRemoveShapeOnLoadedSlideStillRebuilds pins the fallback: structural
// edits the append path cannot express keep the previous full-rebuild
// behavior.
func TestRemoveShapeOnLoadedSlideStillRebuilds(t *testing.T) {
	deck := deckWithGroupShape(t)
	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	slide := p.Slides()[0]
	for _, shape := range slide.Shapes() {
		if tb, ok := shape.(*TextBox); ok {
			slide.RemoveShape(tb)
			break
		}
	}
	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := string(zipPart(t, saved, "ppt/slides/slide1.xml"))
	if strings.Contains(slideXML, "prototype") {
		t.Fatal("removed shape still present")
	}
}

// TestCreatedDeckAddAfterSaveStillSyncs pins created-deck semantics: domain
// mutations after a save still reach the XML via the full rebuild.
func TestCreatedDeckAddAfterSaveStillSyncs(t *testing.T) {
	p := Create()
	slide := p.AddSlide()
	first := slide.AddTextBox()
	first.TextFrame().SetText("first")
	if _, err := p.SaveBytes(); err != nil {
		t.Fatal(err)
	}

	first.TextFrame().SetText("first-edited")
	second := slide.AddTextBox()
	second.TextFrame().SetText("second")
	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := string(zipPart(t, saved, "ppt/slides/slide1.xml"))
	if !strings.Contains(slideXML, "first-edited") || !strings.Contains(slideXML, "second") {
		t.Fatal("created-deck rebuild no longer syncs domain mutations")
	}
}
