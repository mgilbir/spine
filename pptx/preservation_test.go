package pptx

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

// savedDeck builds a one-slide deck via the API and returns its bytes.
func savedDeck(t *testing.T) []byte {
	t.Helper()
	p := Create()
	slide := p.AddSlide()
	box := slide.AddTextBox()
	box.TextFrame().SetText("content")
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// openBytes opens a deck from raw bytes.
func openBytes(t *testing.T, data []byte) *Presentation {
	t.Helper()
	p, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// C170(a): p14/p15 extension booleans are xsd:boolean — val="true" is
// schema-valid and must neither fail parsing (which previously made the slide
// silently vanish) nor be rewritten to a different lexical form on save.
func TestBooleanExtensionValTrueRoundTrips(t *testing.T) {
	ext := `<p:extLst><p:ext uri="{2FDB2607-1784-4EEB-B798-7EB5836EED8A}"><p14:showMediaCtrls xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main" val="true"/></p:ext></p:extLst>`
	data := rewriteZipPart(t, savedDeck(t), "ppt/slides/slide1.xml", func(xml []byte) []byte {
		return bytes.Replace(xml, []byte("</p:sld>"), []byte(ext+"</p:sld>"), 1)
	})

	p := openBytes(t, data)
	if got := len(p.Slides()); got != 1 {
		t.Fatalf("Slides() = %d, want 1: slide with val=\"true\" extension was dropped", got)
	}

	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := zipPart(t, out, "ppt/slides/slide1.xml")
	if !bytes.Contains(slideXML, []byte(`<p14:showMediaCtrls xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main" val="true"/>`)) {
		t.Errorf("saved slide1.xml does not preserve val=\"true\" lexical form:\n%s", slideXML)
	}
	pres := zipPart(t, out, "ppt/presentation.xml")
	if !bytes.Contains(pres, []byte("<p:sldId")) {
		t.Error("saved presentation.xml lost its sldIdLst entries")
	}
}

// C170(a): values outside the xsd:boolean lexical space are rejected, and
// (per C170(b)) the rejection surfaces from Open instead of dropping the slide.
func TestBooleanExtensionInvalidValSurfacesError(t *testing.T) {
	ext := `<p:extLst><p:ext uri="{2FDB2607-1784-4EEB-B798-7EB5836EED8A}"><p14:showMediaCtrls xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main" val="banana"/></p:ext></p:extLst>`
	data := rewriteZipPart(t, savedDeck(t), "ppt/slides/slide1.xml", func(xml []byte) []byte {
		return bytes.Replace(xml, []byte("</p:sld>"), []byte(ext+"</p:sld>"), 1)
	})

	_, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err == nil {
		t.Fatal("Open succeeded on a slide with val=\"banana\"; want a parse error")
	}
	if !strings.Contains(err.Error(), "slide1.xml") {
		t.Errorf("error does not name the failing part: %v", err)
	}
}

// C170(b): a referenced slide that fails to parse must surface as an error
// from Open rather than silently vanishing from the deck.
func TestOpenSurfacesSlideParseError(t *testing.T) {
	data := rewriteZipPart(t, savedDeck(t), "ppt/slides/slide1.xml", func([]byte) []byte {
		return []byte(`<?xml version="1.0"?><p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld>`)
	})

	_, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err == nil {
		t.Fatal("Open succeeded on a deck with a malformed referenced slide; want an error")
	}
	if !strings.Contains(err.Error(), "slide1.xml") {
		t.Errorf("error does not name the failing part: %v", err)
	}
}

// C172: graphicData with an unknown URI (embedded OLE object) must round-trip
// byte-faithfully through open+save instead of being emptied.
func TestUnknownGraphicDataRoundTrips(t *testing.T) {
	const oleData = `<a:graphicData uri="http://schemas.openxmlformats.org/presentationml/2006/ole"><p:oleObj spid="_x0000_s1026" name="Worksheet" r:id="rId7" imgW="1000" imgH="1000" progId="Excel.Sheet.12"><p:embed/></p:oleObj></a:graphicData>`
	gf := `<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="5" name="Object 1"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr><p:xfrm><a:off x="0" y="0"/><a:ext cx="914400" cy="914400"/></p:xfrm><a:graphic>` + oleData + `</a:graphic></p:graphicFrame>`
	data := rewriteZipPart(t, savedDeck(t), "ppt/slides/slide1.xml", func(xml []byte) []byte {
		return bytes.Replace(xml, []byte("</p:spTree>"), []byte(gf+"</p:spTree>"), 1)
	})

	// Zero-modification save.
	p := openBytes(t, data)
	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := zipPart(t, out, "ppt/slides/slide1.xml")
	if !bytes.Contains(slideXML, []byte(oleData)) {
		t.Errorf("saved slide1.xml does not preserve the OLE graphicData verbatim:\n%s", slideXML)
	}
}

// C195: p14:media children (trim/fade/bookmarks) must survive a save instead
// of being reduced to a bare r:embed.
func TestP14MediaTrimRoundTrips(t *testing.T) {
	const media = `<p14:media xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main" r:embed="rId9"><p14:trim st="1000" end="5000"/></p14:media>`
	ext := `<p:extLst><p:ext uri="{DAA4B4D4-6D71-4841-9C94-3DE7FCFB9230}">` + media + `</p:ext></p:extLst>`
	data := rewriteZipPart(t, savedDeck(t), "ppt/slides/slide1.xml", func(xml []byte) []byte {
		return bytes.Replace(xml, []byte("<p:nvPr/>"), []byte("<p:nvPr>"+ext+"</p:nvPr>"), 1)
	})

	p := openBytes(t, data)
	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := zipPart(t, out, "ppt/slides/slide1.xml")
	if !bytes.Contains(slideXML, []byte(media)) {
		t.Errorf("saved slide1.xml does not preserve p14:media children verbatim:\n%s", slideXML)
	}
}

// C196: the full shape-tree rebuild must assign ids above surviving shapes
// (connectors are kept by the rebuild) — cNvPr ids are slide-wide unique.
func TestRebuildAssignsUniqueIDsAboveConnector(t *testing.T) {
	cxn := `<p:cxnSp><p:nvCxnSpPr><p:cNvPr id="2" name="Connector 1"/><p:cNvCxnSpPr/><p:nvPr/></p:nvCxnSpPr><p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="914400" cy="914400"/></a:xfrm><a:prstGeom prst="line"><a:avLst/></a:prstGeom></p:spPr></p:cxnSp>`
	// A slide whose only content is a connector: nothing materializes, so
	// AddTextBox triggers the full rebuild path.
	data := rewriteZipPart(t, savedDeck(t), "ppt/slides/slide1.xml", func(xml []byte) []byte {
		spRE := regexp.MustCompile(`(?s)<p:sp>.*</p:sp>`)
		xml = spRE.ReplaceAll(xml, nil)
		return bytes.Replace(xml, []byte("</p:spTree>"), []byte(cxn+"</p:spTree>"), 1)
	})

	p := openBytes(t, data)
	slide := p.Slides()[0]
	box := slide.AddTextBox()
	box.TextFrame().SetText("added")
	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	slideXML := zipPart(t, out, "ppt/slides/slide1.xml")
	if !bytes.Contains(slideXML, []byte("p:cxnSp")) {
		t.Fatal("connector did not survive the rebuild")
	}
	idRE := regexp.MustCompile(`<p:cNvPr id="(\d+)"`)
	seen := make(map[string]bool)
	for _, m := range idRE.FindAllSubmatch(slideXML, -1) {
		id := string(m[1])
		if seen[id] {
			t.Errorf("duplicate cNvPr id %s in rebuilt slide:\n%s", id, slideXML)
		}
		seen[id] = true
	}
	if len(seen) < 3 {
		t.Errorf("expected at least 3 cNvPr ids (tree, connector, textbox), got %d", len(seen))
	}
}

// C222: an opened deck whose presentation.xml has no defaultTextStyle must not
// gain a fabricated one on save; created decks keep getting the default.
func TestNoFabricatedDefaultTextStyleOnOpenedDeck(t *testing.T) {
	base := savedDeck(t)
	if !bytes.Contains(zipPart(t, base, "ppt/presentation.xml"), []byte("<p:defaultTextStyle>")) {
		t.Fatal("created deck is expected to carry a defaultTextStyle")
	}

	dtsRE := regexp.MustCompile(`(?s)<p:defaultTextStyle>.*</p:defaultTextStyle>`)
	data := rewriteZipPart(t, base, "ppt/presentation.xml", func(xml []byte) []byte {
		out := dtsRE.ReplaceAll(xml, nil)
		if bytes.Equal(out, xml) {
			t.Fatal("failed to strip defaultTextStyle from fixture")
		}
		return out
	})

	p := openBytes(t, data)
	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(zipPart(t, out, "ppt/presentation.xml"), []byte("defaultTextStyle")) {
		t.Error("unmodified save fabricated a defaultTextStyle into a deck that had none")
	}
}
