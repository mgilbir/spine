package pptx

import (
	"bytes"
	"testing"
)

// unreferencedSlideDeckWithPresRel builds a package that carries slide2.xml
// without listing it in sldIdLst, while presentation.xml.rels still holds a
// slide-type relationship pointing at it. Valid producers emit this: the part is
// reachable, just not shown.
func unreferencedSlideDeckWithPresRel(t *testing.T) []byte {
	t.Helper()
	base := savedDeck(t)
	slide1XML := zipPart(t, base, "ppt/slides/slide1.xml")
	unrefXML := bytes.Replace(slide1XML, []byte(">content<"), []byte(">unreferenced<"), 1)

	deck := addZipParts(t, base, map[string][]byte{
		"ppt/slides/slide2.xml":            unrefXML,
		"ppt/slides/_rels/slide2.xml.rels": []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/></Relationships>`),
	})
	deck = rewriteZipPart(t, deck, "[Content_Types].xml", func(ct []byte) []byte {
		add := `<Override PartName="/ppt/slides/slide2.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`
		return bytes.Replace(ct, []byte("</Types>"), []byte(add+"</Types>"), 1)
	})
	// A slide-type presentation relationship to the unlisted part. Its id is
	// chosen well above the deck's own ids so it cannot be confused with one the
	// allocator would hand out.
	return rewriteZipPart(t, deck, "ppt/_rels/presentation.xml.rels", func(rels []byte) []byte {
		add := `<Relationship Id="rId900" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide2.xml"/>`
		return bytes.Replace(rels, []byte("</Relationships>"), []byte(add+"</Relationships>"), 1)
	})
}

// C512: a slide-type presentation relationship whose target is a preserved
// unreferenced slide (the C118 case) must survive the save. Dropping it left the
// part in the package but rel-orphaned, and the resulting change to the rel set
// forced presentation.xml.rels to be regenerated, so a zero-modification save
// stopped being byte-identical.
func TestUnreferencedSlideRel_SurvivesSaveAndKeepsByteIdentity(t *testing.T) {
	deck := unreferencedSlideDeckWithPresRel(t)

	p := openBytes(t, deck)
	if got := p.SlideCount(); got != 1 {
		t.Fatalf("SlideCount = %d, want 1", got)
	}
	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	rels := zipPart(t, out, "ppt/_rels/presentation.xml.rels")
	if !bytes.Contains(rels, []byte(`Id="rId900"`)) {
		t.Errorf("slide rel to the preserved unreferenced slide was dropped:\n%s", rels)
	}
	if _, ok := zipPartIfExists(t, out, "ppt/slides/slide2.xml"); !ok {
		t.Error("unreferenced slide part vanished on save")
	}
	// Zero-modification save keeps presentation.xml.rels byte-identical.
	if want := zipPart(t, deck, "ppt/_rels/presentation.xml.rels"); !bytes.Equal(rels, want) {
		t.Errorf("presentation.xml.rels was regenerated on a zero-modification save:\ngot  %s\nwant %s", rels, want)
	}
}

// The rel must be dropped, not blindly kept, when its target really is gone: a
// slide-type rel to a part the package does not carry would otherwise stay
// dangling.
func TestDanglingSlideRel_StillDropped(t *testing.T) {
	deck := rewriteZipPart(t, savedDeck(t), "ppt/_rels/presentation.xml.rels", func(rels []byte) []byte {
		add := `<Relationship Id="rId901" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide99.xml"/>`
		return bytes.Replace(rels, []byte("</Relationships>"), []byte(add+"</Relationships>"), 1)
	})
	out, err := openBytes(t, deck).SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if rels := zipPart(t, out, "ppt/_rels/presentation.xml.rels"); bytes.Contains(rels, []byte(`Id="rId901"`)) {
		t.Errorf("slide rel to a part the package does not carry was kept:\n%s", rels)
	}
}
