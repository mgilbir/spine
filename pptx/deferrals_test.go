package pptx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// firstGroup returns the first GroupShape on a slide, failing when there is none.
func firstGroup(t *testing.T, s *Slide) *GroupShape {
	t.Helper()
	for _, sh := range s.Shapes() {
		if g, ok := sh.(*GroupShape); ok {
			return g
		}
	}
	t.Fatal("slide has no group shape")
	return nil
}

// --- Item 1: connectors inside groups ---

// A connector added to a domain-built group serializes as a p:cxnSp inside the
// p:grpSp and materializes back as a group child on reopen.
func TestGroupConnector_CreatedDeck(t *testing.T) {
	p := Create()
	s := p.AddSlide()

	g := NewGroupShape()
	g.SetName("ConnGroup")
	g.SetPosition(dml.Inches(1), dml.Inches(1))
	g.SetSize(dml.Inches(4), dml.Inches(2))
	c := g.AddConnector(ConnectorElbow)
	c.SetPoints(dml.Inches(1), dml.Inches(1), dml.Inches(3), dml.Inches(2))
	c.SetLineColor(dml.NewRGB(0x11, 0x22, 0x33).ToColor())
	if err := s.AddShape(g); err != nil {
		t.Fatal(err)
	}

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	grp := xml[strings.Index(xml, "<p:grpSp>"):strings.Index(xml, "</p:grpSp>")]
	if !strings.Contains(grp, "<p:cxnSp>") || !strings.Contains(grp, "bentConnector3") {
		t.Fatalf("connector not serialized inside group:\n%s", grp)
	}
	if !strings.Contains(grp, "112233") {
		t.Errorf("connector line color lost:\n%s", grp)
	}

	// Reopen: the connector materializes as a group child.
	g2 := firstGroup(t, openBytes(t, data).Slides()[0])
	conns := g2.Connectors()
	if len(conns) != 1 {
		t.Fatalf("want 1 group connector after reopen, got %d", len(conns))
	}
	if conns[0].Kind() != ConnectorElbow {
		t.Errorf("group connector kind = %v, want elbow", conns[0].Kind())
	}
}

// A connector added to a group inside a loaded deck binds to another group
// child; the binding resolves to that child's assigned cNvPr id on save.
func TestGroupConnector_BindsChildIDs(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	g := NewGroupShape()
	g.SetSize(dml.Inches(5), dml.Inches(5))
	a := NewAutoShape(PresetRect)
	a.SetBounds(dml.NewRect(dml.Inches(1), dml.Inches(1), dml.Inches(1), dml.Inches(1)))
	b := NewAutoShape(PresetEllipse)
	b.SetBounds(dml.NewRect(dml.Inches(3), dml.Inches(3), dml.Inches(1), dml.Inches(1)))
	g.AddChild(a)
	g.AddChild(b)
	c := g.AddConnector(ConnectorStraight)
	c.Connect(a, 3, b, 1)
	if err := s.AddShape(g); err != nil {
		t.Fatal(err)
	}

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if !strings.Contains(xml, `<a:stCxn id="`) || !strings.Contains(xml, `idx="3"`) {
		t.Errorf("grouped connector start binding not resolved:\n%s", xml)
	}
	if !strings.Contains(xml, `idx="1"`) {
		t.Errorf("grouped connector end binding not resolved:\n%s", xml)
	}
	assertUniqueShapeIDs(t, xml)
}

// A connector added to a group that was loaded from a file is appended to the
// parsed p:grpSp without disturbing the existing children.
func TestGroupConnector_LoadedGroupAppend(t *testing.T) {
	grp := `<p:grpSp><p:nvGrpSpPr><p:cNvPr id="10" name="G"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
		`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="914400" cy="914400"/>` +
		`<a:chOff x="0" y="0"/><a:chExt cx="914400" cy="914400"/></a:xfrm></p:grpSpPr>` +
		`<p:sp><p:nvSpPr><p:cNvPr id="11" name="inner"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>` +
		`<p:spPr/><p:txBody><a:bodyPr/><a:p><a:r><a:t>grouped</a:t></a:r></a:p></p:txBody></p:sp></p:grpSp>`
	data := rewriteZipPart(t, savedDeck(t), "ppt/slides/slide1.xml", func(xml []byte) []byte {
		return bytes.Replace(xml, []byte("</p:spTree>"), []byte(grp+"</p:spTree>"), 1)
	})

	p := openBytes(t, data)
	g := firstGroup(t, p.Slides()[0])
	c := g.AddConnector(ConnectorCurved)
	c.SetPoints(0, 0, dml.Inches(1), dml.Inches(1))

	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, out, "ppt/slides/slide1.xml"))
	inner := xml[strings.Index(xml, "<p:grpSp>"):strings.Index(xml, "</p:grpSp>")]
	if !strings.Contains(inner, "curvedConnector3") {
		t.Errorf("connector not appended to loaded group:\n%s", inner)
	}
	if !strings.Contains(inner, "grouped") {
		t.Errorf("existing group child lost:\n%s", inner)
	}
	if n := len(firstGroup(t, openBytes(t, out).Slides()[0]).Connectors()); n != 1 {
		t.Errorf("want 1 group connector after reopen, got %d", n)
	}
}

// A group containing a connector, materialized from a loaded deck but left
// untouched, round-trips byte-for-byte.
func TestGroupConnector_UntouchedByteIdentical(t *testing.T) {
	grp := `<p:grpSp><p:nvGrpSpPr><p:cNvPr id="20" name="G"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
		`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="914400" cy="914400"/>` +
		`<a:chOff x="0" y="0"/><a:chExt cx="914400" cy="914400"/></a:xfrm></p:grpSpPr>` +
		`<p:cxnSp><p:nvCxnSpPr><p:cNvPr id="21" name="Connector 1"/><p:cNvCxnSpPr/><p:nvPr/></p:nvCxnSpPr>` +
		`<p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="914400" cy="914400"/></a:xfrm>` +
		`<a:prstGeom prst="straightConnector1"><a:avLst/></a:prstGeom></p:spPr></p:cxnSp></p:grpSp>`
	data := rewriteZipPart(t, savedDeck(t), "ppt/slides/slide1.xml", func(xml []byte) []byte {
		return bytes.Replace(xml, []byte("</p:spTree>"), []byte(grp+"</p:spTree>"), 1)
	})

	before := zipPart(t, data, "ppt/slides/slide1.xml")
	p := openBytes(t, data)
	// The connector materializes as a group child.
	if n := len(firstGroup(t, p.Slides()[0]).Connectors()); n != 1 {
		t.Fatalf("want 1 group connector, got %d", n)
	}
	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	after := zipPart(t, out, "ppt/slides/slide1.xml")
	if !bytes.Equal(before, after) {
		t.Errorf("untouched group connector not byte-identical:\nbefore: %s\nafter:  %s", before, after)
	}
}

// --- Item 2: adding a brand-new master text-style level ---

// Adding a level absent from the source list style inserts it at its schema
// position: before a higher existing level and before a trailing a:extLst.
func TestMasterTextStyle_AddNewLevelOrder(t *testing.T) {
	// A bodyStyle with lvl1pPr and lvl3pPr (lvl2 missing) plus an a:extLst.
	body := `<p:bodyStyle>` +
		`<a:lvl1pPr><a:defRPr sz="1800"/></a:lvl1pPr>` +
		`<a:lvl3pPr><a:defRPr sz="1400"/></a:lvl3pPr>` +
		`<a:extLst><a:ext uri="{X}"/></a:extLst>` +
		`</p:bodyStyle>`
	data := rewriteZipPart(t, savedDeck(t), "ppt/slideMasters/slideMaster1.xml", func(xml []byte) []byte {
		start := bytes.Index(xml, []byte("<p:bodyStyle>"))
		end := bytes.Index(xml, []byte("</p:bodyStyle>")) + len("</p:bodyStyle>")
		return append(append(append([]byte{}, xml[:start]...), []byte(body)...), xml[end:]...)
	})

	p := openBytes(t, data)
	ts := p.SlideMasters()[0].BodyStyle()
	ts.SetLevelFontSize(1, 16) // add lvl2pPr (index 1)

	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, out, "ppt/slideMasters/slideMaster1.xml"))

	l1 := strings.Index(xml, "<a:lvl1pPr")
	l2 := strings.Index(xml, "<a:lvl2pPr")
	l3 := strings.Index(xml, "<a:lvl3pPr")
	ext := strings.Index(xml, "<a:extLst")
	if l2 < 0 {
		t.Fatalf("new lvl2pPr not written:\n%s", xml)
	}
	if l1 >= l2 || l2 >= l3 || l3 >= ext {
		t.Errorf("levels/extLst out of schema order: lvl1=%d lvl2=%d lvl3=%d extLst=%d\n%s", l1, l2, l3, ext, xml)
	}
	if !strings.Contains(xml, `<a:defRPr sz="1600"/>`) {
		t.Errorf("new level font size not written:\n%s", xml)
	}
}

// --- Item 3: image (blip) backgrounds on layout, master, and slide ---

func TestBackgroundImage_LayoutMasterSlide(t *testing.T) {
	png := minimalTransparentPNG

	t.Run("layout", func(t *testing.T) {
		p := Create()
		p.AddSlide()
		layout := p.SlideMasters()[0].Layouts()[0]
		if err := layout.SetBackgroundImage(png, "image/png"); err != nil {
			t.Fatal(err)
		}
		data, err := p.SaveBytes()
		if err != nil {
			t.Fatal(err)
		}
		xml := string(zipPart(t, data, "ppt/slideLayouts/slideLayout1.xml"))
		if !strings.Contains(xml, "<a:blipFill>") || !strings.Contains(xml, "<a:blip") {
			t.Fatalf("layout image background not emitted:\n%s", xml)
		}
		rels := string(zipPart(t, data, "ppt/slideLayouts/_rels/slideLayout1.xml.rels"))
		if !strings.Contains(rels, "/relationships/image") {
			t.Errorf("layout image relationship missing:\n%s", rels)
		}
		if !p.SlideMasters()[0].Layouts()[0].HasBackground() {
			t.Error("layout HasBackground() false after SetBackgroundImage")
		}
		// media part present
		if _, ok := zipPartIfExists(t, data, "ppt/media/image1.png"); !ok {
			t.Error("layout background media part not written")
		}
	})

	t.Run("master", func(t *testing.T) {
		p := Create()
		p.AddSlide()
		if err := p.SlideMasters()[0].SetBackgroundImage(png, "image/png"); err != nil {
			t.Fatal(err)
		}
		data, err := p.SaveBytes()
		if err != nil {
			t.Fatal(err)
		}
		xml := string(zipPart(t, data, "ppt/slideMasters/slideMaster1.xml"))
		if !strings.Contains(xml, "<a:blipFill>") {
			t.Fatalf("master image background not emitted:\n%s", xml)
		}
	})

	t.Run("slide", func(t *testing.T) {
		p := Create()
		s := p.AddSlide()
		if err := s.SetBackgroundImage(png, "image/png"); err != nil {
			t.Fatal(err)
		}
		data, err := p.SaveBytes()
		if err != nil {
			t.Fatal(err)
		}
		xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
		if !strings.Contains(xml, "<a:blipFill>") {
			t.Fatalf("slide image background not emitted:\n%s", xml)
		}
		if !openBytes(t, data).Slides()[0].HasBackground() {
			t.Error("slide HasBackground() false after reopen")
		}
	})
}

func TestBackgroundImage_Errors(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	if err := s.SetBackgroundImage(nil, "image/png"); err == nil {
		t.Error("expected error for empty data")
	}
	if err := s.SetBackgroundImage([]byte{1, 2, 3}, ""); err == nil {
		t.Error("expected error for empty content type")
	}
}

// --- Item 4: authoring a transition start sound ---

// minimalWAV returns a tiny but structurally valid RIFF/WAVE header so the
// content-type sniffer recognizes it as audio/wav.
func minimalWAV() []byte {
	return []byte("RIFF\x24\x00\x00\x00WAVEfmt \x10\x00\x00\x00" +
		"\x01\x00\x01\x00\x40\x1f\x00\x00\x40\x1f\x00\x00\x01\x00\x08\x00data\x00\x00\x00\x00")
}

func TestTransitionStartSound_Author(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.SetTransition(Transition{
		Type: TransitionFade,
		Sound: &TransitionSound{
			StartSoundName: "chime",
			StartSoundLoop: true,
			StartSoundData: minimalWAV(),
		},
	})

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if !strings.Contains(xml, "<p:sndAc>") || !strings.Contains(xml, "<p:stSnd") {
		t.Fatalf("start sound action not emitted:\n%s", xml)
	}
	if !strings.Contains(xml, `name="chime"`) || !strings.Contains(xml, `loop="1"`) {
		t.Errorf("start sound name/loop not emitted:\n%s", xml)
	}
	rels := string(zipPart(t, data, "ppt/slides/_rels/slide1.xml.rels"))
	if !strings.Contains(rels, "/relationships/audio") {
		t.Errorf("audio relationship missing:\n%s", rels)
	}
	if _, ok := zipPartIfExists(t, data, "ppt/media/media1.wav"); !ok {
		t.Error("audio media part not written")
	}

	// Reopen: the start sound reads back.
	tr := openBytes(t, data).Slides()[0].Transition()
	if tr == nil || tr.Sound == nil || tr.Sound.StartSoundName != "chime" || !tr.Sound.StartSoundLoop {
		t.Errorf("start sound did not round-trip: %+v", tr)
	}
}

// A repeated SetTransition with the same sound reuses the embedded part rather
// than embedding it twice.
func TestTransitionStartSound_NoDoubleEmbed(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	snd := &TransitionSound{StartSoundName: "beep", StartSoundData: minimalWAV()}
	s.SetTransition(Transition{Type: TransitionFade, Sound: snd})
	s.SetTransition(Transition{Type: TransitionFade, Sound: snd})

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := zipPartIfExists(t, data, "ppt/media/media2.wav"); ok {
		t.Error("start sound embedded twice")
	}
}

// --- Item 5: embedding fonts from raw bytes ---

func TestEmbedFont_CreatedDeck(t *testing.T) {
	p := Create()
	p.AddSlide()
	regular := []byte("REGULAR-FONT-BYTES")
	bold := []byte("BOLD-FONT-BYTES")
	if err := p.EmbedFont("Courier New", regular, bold, nil, nil); err != nil {
		t.Fatal(err)
	}
	if !p.EmbedTrueTypeFonts() {
		t.Error("embedTrueTypeFonts not set by EmbedFont")
	}

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := zipPartIfExists(t, data, "ppt/fonts/font1.fntdata"); !ok || !bytes.Equal(got, regular) {
		t.Errorf("regular font part missing or wrong bytes (ok=%v)", ok)
	}
	if got, ok := zipPartIfExists(t, data, "ppt/fonts/font2.fntdata"); !ok || !bytes.Equal(got, bold) {
		t.Errorf("bold font part missing or wrong bytes (ok=%v)", ok)
	}
	rels := string(zipPart(t, data, "ppt/_rels/presentation.xml.rels"))
	if strings.Count(rels, "/relationships/font") != 2 {
		t.Errorf("expected two font relationships:\n%s", rels)
	}
	pres := string(zipPart(t, data, "ppt/presentation.xml"))
	if !strings.Contains(pres, "<p:embeddedFontLst>") || !strings.Contains(pres, `typeface="Courier New"`) {
		t.Errorf("embeddedFontLst not written:\n%s", pres)
	}

	// Reopen: the font entry round-trips with resolvable rel ids.
	reopened := openBytes(t, data)
	fonts := reopened.EmbeddedFonts()
	if len(fonts) != 1 || fonts[0].Typeface != "Courier New" || fonts[0].Regular == "" || fonts[0].Bold == "" {
		t.Fatalf("reopened EmbeddedFonts = %+v", fonts)
	}
}

func TestEmbedFont_OpenedDeckAndReplace(t *testing.T) {
	p := openBytes(t, savedDeck(t))
	if err := p.EmbedFont("Arial", []byte("A-REG"), nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	// Replacing the same typeface must not add a second embeddedFont entry.
	if err := p.EmbedFont("Arial", []byte("A-REG-2"), []byte("A-BOLD"), nil, nil); err != nil {
		t.Fatal(err)
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	pres := string(zipPart(t, data, "ppt/presentation.xml"))
	if got := strings.Count(pres, "<p:embeddedFont>"); got != 1 {
		t.Errorf("expected 1 embeddedFont entry after replace, got %d:\n%s", got, pres)
	}
	reopened := openBytes(t, data)
	if fonts := reopened.EmbeddedFonts(); len(fonts) != 1 || fonts[0].Bold == "" {
		t.Errorf("reopened EmbeddedFonts = %+v", fonts)
	}
}

func TestEmbedFont_Errors(t *testing.T) {
	p := Create()
	if err := p.EmbedFont("", []byte("x"), nil, nil, nil); err == nil {
		t.Error("expected error for empty typeface")
	}
	if err := p.EmbedFont("Arial", nil, nil, nil, nil); err == nil {
		t.Error("expected error for missing regular data")
	}
}
