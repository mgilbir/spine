package pptx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// arrowedConnectorLn is an a:ln in the shape PowerPoint writes for a connector
// with arrowheads — the normal case for a connector — plus a round cap and a
// compound line. None of headEnd, tailEnd, cap or cmpd is reachable through the
// pptx API, so a line setter must not disturb them.
const arrowedConnectorLn = `<a:ln w="19050" cap="rnd" cmpd="sng">` +
	`<a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>` +
	`<a:prstDash val="dash"/>` +
	`<a:headEnd type="triangle" w="med" len="med"/>` +
	`<a:tailEnd type="stealth" w="lg" len="lg"/>` +
	`</a:ln>`

// deckWithArrowedConnector builds a deck with one connector and replaces its
// a:ln with arrowedConnectorLn.
func deckWithArrowedConnector(t *testing.T) []byte {
	t.Helper()
	p := Create()
	s := p.AddSlide()
	c := s.AddConnector(ConnectorStraight)
	c.SetLine(dml.Line{Width: 1, Color: dml.ColorBlack})

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	return rewriteZipPart(t, data, "ppt/slides/slide1.xml", func(xml []byte) []byte {
		start := bytes.Index(xml, []byte("<a:ln "))
		if start < 0 {
			start = bytes.Index(xml, []byte("<a:ln>"))
		}
		end := bytes.Index(xml, []byte("</a:ln>"))
		if start < 0 || end < 0 {
			t.Fatalf("no a:ln in the connector:\n%s", xml)
		}
		out := append([]byte{}, xml[:start]...)
		out = append(out, arrowedConnectorLn...)
		return append(out, xml[end+len("</a:ln>"):]...)
	})
}

// editConnector opens the deck, applies edit to the first connector, and
// returns the saved slide XML.
func editConnector(t *testing.T, deck []byte, edit func(*Connector)) string {
	t.Helper()
	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	var conn *Connector
	for _, sh := range p.Slides()[0].Shapes() {
		if c, ok := sh.(*Connector); ok {
			conn = c
			break
		}
	}
	if conn == nil {
		t.Fatal("no connector in the reopened deck")
	}
	edit(conn)

	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	return string(zipPart(t, saved, "ppt/slides/slide1.xml"))
}

// C417: SetLineWidth rebuilt a fresh a:ln from the three properties dml.Line
// models and assigned it over the parsed one, so a connector lost both its
// arrowheads and its cap on a width change.
func TestConnector_SetLineWidth_KeepsArrowheadsAndCap(t *testing.T) {
	out := editConnector(t, deckWithArrowedConnector(t), func(c *Connector) {
		c.SetLineWidth(3)
	})

	if !strings.Contains(out, `w="38100"`) {
		t.Fatalf("the width edit did not reach the XML:\n%s", out)
	}
	for _, want := range []string{
		`<a:headEnd type="triangle" w="med" len="med"/>`,
		`<a:tailEnd type="stealth" w="lg" len="lg"/>`,
		`cap="rnd"`,
		`cmpd="sng"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("SetLineWidth dropped unmodeled line property %s\n%s", want, out)
		}
	}
	// The properties the setter does not own keep their parsed values.
	if !strings.Contains(out, `<a:prstDash val="dash"/>`) {
		t.Errorf("SetLineWidth changed the dash:\n%s", out)
	}
	if !strings.Contains(out, `val="FF0000"`) {
		t.Errorf("SetLineWidth changed the colour:\n%s", out)
	}
}

// C417: the same for a colour change.
func TestConnector_SetLineColor_KeepsArrowheads(t *testing.T) {
	out := editConnector(t, deckWithArrowedConnector(t), func(c *Connector) {
		c.SetLineColor(dml.NewRGB(0, 255, 0).ToColor())
	})

	if !strings.Contains(out, `val="00FF00"`) {
		t.Fatalf("the colour edit did not reach the XML:\n%s", out)
	}
	if strings.Contains(out, `val="FF0000"`) {
		t.Errorf("the old colour survived alongside the new one:\n%s", out)
	}
	for _, want := range []string{`<a:headEnd type="triangle"`, `<a:tailEnd type="stealth"`, `cap="rnd"`} {
		if !strings.Contains(out, want) {
			t.Errorf("SetLineColor dropped unmodeled line property %s\n%s", want, out)
		}
	}
}

// C417: a dash change must replace the dash and nothing else — and setting the
// line back to solid must actually clear the parsed dash rather than silently
// doing nothing.
func TestConnector_SetLineDash_ReplacesOnlyTheDash(t *testing.T) {
	out := editConnector(t, deckWithArrowedConnector(t), func(c *Connector) {
		c.SetLineDash(dml.DashSolid)
	})

	if strings.Contains(out, `<a:prstDash val="dash"/>`) {
		t.Errorf("SetLineDash(DashSolid) left the parsed dash in place:\n%s", out)
	}
	if !strings.Contains(out, `<a:prstDash val="solid"/>`) {
		t.Errorf("SetLineDash(DashSolid) emitted no dash at all:\n%s", out)
	}
	for _, want := range []string{`<a:headEnd type="triangle"`, `cap="rnd"`, `val="FF0000"`} {
		if !strings.Contains(out, want) {
			t.Errorf("SetLineDash dropped unmodeled line property %s\n%s", want, out)
		}
	}
}

// C417: the same defect applies to AutoShape/TextBox lines, which go through
// the same applyShapeStyle path.
func TestAutoShape_SetLine_KeepsUnmodeledLineProperties(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	sh := NewAutoShape("rect")
	sh.SetLine(dml.Line{Width: 1, Color: dml.ColorBlack})
	if err := s.AddShape(sh); err != nil {
		t.Fatal(err)
	}

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	deck := rewriteZipPart(t, data, "ppt/slides/slide1.xml", func(xml []byte) []byte {
		start := bytes.Index(xml, []byte("<a:ln "))
		if start < 0 {
			start = bytes.Index(xml, []byte("<a:ln>"))
		}
		end := bytes.Index(xml, []byte("</a:ln>"))
		if start < 0 || end < 0 {
			t.Fatalf("no a:ln on the autoshape:\n%s", xml)
		}
		out := append([]byte{}, xml[:start]...)
		out = append(out, arrowedConnectorLn...)
		return append(out, xml[end+len("</a:ln>"):]...)
	})

	p2, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	for _, shape := range p2.Slides()[0].Shapes() {
		if as, ok := shape.(*AutoShape); ok {
			as.SetLine(dml.Line{Width: 4, Color: dml.ColorBlack})
		}
	}
	saved, err := p2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	out := string(zipPart(t, saved, "ppt/slides/slide1.xml"))

	if !strings.Contains(out, `w="50800"`) {
		t.Fatalf("the width edit did not reach the XML:\n%s", out)
	}
	if !strings.Contains(out, `cap="rnd"`) {
		t.Errorf("AutoShape.SetLineWidth dropped the parsed line cap:\n%s", out)
	}
}
