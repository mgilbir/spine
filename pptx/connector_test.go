package pptx

import (
	"bytes"
	"regexp"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// A connector added to a created deck serializes as a p:cxnSp with the routing
// preset geometry, an xfrm, and a styled line.
func TestAddConnectorFreePoints(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	c := s.AddConnector(ConnectorElbow)
	c.SetPoints(dml.Inches(1), dml.Inches(1), dml.Inches(3), dml.Inches(2))
	c.SetLineWidth(2)
	c.SetLineColor(dml.NewRGB(0xFF, 0x00, 0x00).ToColor())
	c.SetLineDash(dml.DashDash)

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := zipPart(t, data, "ppt/slides/slide1.xml")

	for _, want := range []string{
		"<p:cxnSp>", "<p:nvCxnSpPr>", "<p:cNvCxnSpPr", "bentConnector3",
		`w="25400"`, "FF0000", `val="dash"`,
	} {
		if !bytes.Contains(xml, []byte(want)) {
			t.Errorf("saved slide missing %q:\n%s", want, xml)
		}
	}
	if bytes.Contains(xml, []byte("stCxn")) || bytes.Contains(xml, []byte("endCxn")) {
		t.Errorf("free-point connector should have no stCxn/endCxn:\n%s", xml)
	}
}

// Connect binds both endpoints to shapes; the bindings resolve to the shapes'
// assigned cNvPr ids at save time.
func TestAddConnectorBindsShapeIDs(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	a := NewAutoShape(PresetRect)
	a.SetBounds(dml.NewRect(dml.Inches(1), dml.Inches(1), dml.Inches(1), dml.Inches(1)))
	b := NewAutoShape(PresetEllipse)
	b.SetBounds(dml.NewRect(dml.Inches(4), dml.Inches(3), dml.Inches(1), dml.Inches(1)))
	if err := s.AddShape(a); err != nil {
		t.Fatal(err)
	}
	if err := s.AddShape(b); err != nil {
		t.Fatal(err)
	}
	c := s.AddConnector(ConnectorStraight)
	c.Connect(a, 3, b, 1)

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := zipPart(t, data, "ppt/slides/slide1.xml")

	// a and b get ids 2 and 3 (tree is 1); the connector binds to them.
	stRE := regexp.MustCompile(`<a:stCxn id="(\d+)" idx="3"/>`)
	endRE := regexp.MustCompile(`<a:endCxn id="(\d+)" idx="1"/>`)
	sm := stRE.FindSubmatch(xml)
	em := endRE.FindSubmatch(xml)
	if sm == nil || em == nil {
		t.Fatalf("connector bindings missing:\n%s", xml)
	}
	if got := string(sm[1]); got != "2" {
		t.Errorf("stCxn id = %s, want 2", got)
	}
	if got := string(em[1]); got != "3" {
		t.Errorf("endCxn id = %s, want 3", got)
	}
}

// Connectors materialized from a loaded deck round-trip byte-identically when
// untouched.
func TestConnectorPreservedByteIdentical(t *testing.T) {
	cxn := `<p:cxnSp><p:nvCxnSpPr><p:cNvPr id="7" name="Straight Arrow Connector 6"/>` +
		`<p:cNvCxnSpPr><a:stCxn id="2" idx="3"/><a:endCxn id="3" idx="1"/></p:cNvCxnSpPr><p:nvPr/></p:nvCxnSpPr>` +
		`<p:spPr><a:xfrm flipV="1"><a:off x="914400" y="914400"/><a:ext cx="1828800" cy="914400"/></a:xfrm>` +
		`<a:prstGeom prst="straightConnector1"><a:avLst/></a:prstGeom>` +
		`<a:ln w="12700"><a:solidFill><a:srgbClr val="00B0F0"/></a:solidFill></a:ln></p:spPr></p:cxnSp>`
	data := rewriteZipPart(t, savedDeck(t), "ppt/slides/slide1.xml", func(xml []byte) []byte {
		return bytes.Replace(xml, []byte("</p:spTree>"), []byte(cxn+"</p:spTree>"), 1)
	})

	before := zipPart(t, data, "ppt/slides/slide1.xml")
	p := openBytes(t, data)

	// The connector materializes and reads back correctly.
	conns := p.Slides()[0].Connectors()
	if len(conns) != 1 {
		t.Fatalf("want 1 connector, got %d", len(conns))
	}
	c := conns[0]
	if c.Kind() != ConnectorStraight {
		t.Errorf("kind = %v, want straight", c.Kind())
	}
	if id, idx, ok := c.StartConnection(); !ok || id != 2 || idx != 3 {
		t.Errorf("StartConnection = (%d,%d,%v), want (2,3,true)", id, idx, ok)
	}
	if id, idx, ok := c.EndConnection(); !ok || id != 3 || idx != 1 {
		t.Errorf("EndConnection = (%d,%d,%v), want (3,1,true)", id, idx, ok)
	}
	if col := c.LineColor(); col == nil || col.RGB.String() != "00B0F0" {
		t.Errorf("LineColor = %v, want 00B0F0", col)
	}

	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	after := zipPart(t, out, "ppt/slides/slide1.xml")
	if !bytes.Equal(before, after) {
		t.Errorf("unmodified connector not byte-identical:\nbefore: %s\nafter:  %s", before, after)
	}
}

// Editing a materialized connector flushes into its parsed node in place,
// leaving other slide content untouched.
func TestConnectorEditInPlace(t *testing.T) {
	cxn := `<p:cxnSp><p:nvCxnSpPr><p:cNvPr id="7" name="Connector 6"/><p:cNvCxnSpPr/><p:nvPr/></p:nvCxnSpPr>` +
		`<p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="914400" cy="914400"/></a:xfrm>` +
		`<a:prstGeom prst="straightConnector1"><a:avLst/></a:prstGeom></p:spPr></p:cxnSp>`
	data := rewriteZipPart(t, savedDeck(t), "ppt/slides/slide1.xml", func(xml []byte) []byte {
		return bytes.Replace(xml, []byte("</p:spTree>"), []byte(cxn+"</p:spTree>"), 1)
	})

	p := openBytes(t, data)
	c := p.Slides()[0].Connectors()[0]
	c.SetLineColor(dml.NewRGB(0x00, 0xFF, 0x00).ToColor())

	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := zipPart(t, out, "ppt/slides/slide1.xml")
	if !bytes.Contains(xml, []byte("00FF00")) {
		t.Errorf("edited line color not written:\n%s", xml)
	}
	// The original text box is untouched.
	if !bytes.Contains(xml, []byte("content")) {
		t.Errorf("sibling shape lost after connector edit:\n%s", xml)
	}
}

// A connector added to a loaded deck is appended without disturbing the
// existing shapes.
func TestAddConnectorToLoadedDeck(t *testing.T) {
	p := openBytes(t, savedDeck(t))
	s := p.Slides()[0]
	c := s.AddConnector(ConnectorCurved)
	c.SetPoints(0, 0, dml.Inches(2), dml.Inches(2))

	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := zipPart(t, out, "ppt/slides/slide1.xml")
	if !bytes.Contains(xml, []byte("curvedConnector3")) {
		t.Errorf("connector not appended:\n%s", xml)
	}
	if !bytes.Contains(xml, []byte("content")) {
		t.Errorf("existing shape lost:\n%s", xml)
	}
	// Reopen: the connector is materialized.
	if n := len(openBytes(t, out).Slides()[0].Connectors()); n != 1 {
		t.Errorf("want 1 connector after reopen, got %d", n)
	}
}
