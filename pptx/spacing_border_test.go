package pptx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// C80: paragraph line spacing / space before / space after are serialized, and
// a spacing-only paragraph does not gain a spurious <a:buNone/>.
func TestParagraphToOxml_Spacing(t *testing.T) {
	p := &Paragraph{}
	p.SetLineSpacing(150000) // 150%
	p.SetSpaceBefore(dml.Points(12)) // 1200 hundredths of a point
	p.SetSpaceAfter(dml.Points(6))

	ap := paragraphToOxml(p)
	if ap.PPr == nil {
		t.Fatal("no paragraph properties emitted for spacing")
	}
	if ap.PPr.LnSpc == nil || ap.PPr.LnSpc.SpcPct == nil || ap.PPr.LnSpc.SpcPct.Val.Int32() != 150000 {
		t.Errorf("line spacing not serialized: %+v", ap.PPr.LnSpc)
	}
	if ap.PPr.SpcBef == nil || ap.PPr.SpcBef.SpcPts == nil || ap.PPr.SpcBef.SpcPts.Val != 1200 {
		t.Errorf("space-before not serialized: %+v", ap.PPr.SpcBef)
	}
	if ap.PPr.SpcAft == nil || ap.PPr.SpcAft.SpcPts == nil || ap.PPr.SpcAft.SpcPts.Val != 600 {
		t.Errorf("space-after not serialized: %+v", ap.PPr.SpcAft)
	}
	if ap.PPr.BuNone != nil {
		t.Error("spacing-only paragraph gained an unwanted buNone (would suppress inherited bullets)")
	}
}

// C80: run highlight is serialized.
func TestRunToOxml_Highlight(t *testing.T) {
	r := &Run{text: "hi"}
	r.SetHighlight(dml.ColorYellow)

	ar := runToOxml(r)
	if ar.RPr == nil || ar.RPr.Highlight == nil {
		t.Fatalf("run highlight not serialized: %+v", ar.RPr)
	}
	if ar.RPr.Highlight.SrgbClr == nil {
		t.Errorf("highlight color not set: %+v", ar.RPr.Highlight)
	}
}

// C81: table cell borders are serialized as cell edge lines.
func TestTableDataToOxml_CellBorders(t *testing.T) {
	tbl := NewTable(1, 1)
	tbl.Cell(0, 0).SetBorderLeft(&TableBorder{Width: 12700, Color: dml.ColorRed, Style: BorderStyleSingle})

	at := tableDataToOxml(tbl)
	tc := at.Tr[0].Tc[0]
	if tc.TcPr == nil || tc.TcPr.LnL == nil {
		t.Fatal("left border dropped")
	}
	if tc.TcPr.LnL.W == nil || *tc.TcPr.LnL.W != 12700 {
		t.Errorf("border width not serialized: %+v", tc.TcPr.LnL.W)
	}
	if tc.TcPr.LnL.SolidFill == nil {
		t.Error("border color not serialized")
	}

	// An explicit "none" border becomes a no-fill line.
	tbl.Cell(0, 0).SetBorderTop(&TableBorder{Style: BorderStyleNone})
	tc = tableDataToOxml(tbl).Tr[0].Tc[0]
	if tc.TcPr.LnT == nil || tc.TcPr.LnT.NoFill == nil {
		t.Errorf("none border not serialized as no-fill line: %+v", tc.TcPr.LnT)
	}
}

// Issue #356: space before and after are EMU in the API and hundredths of
// a point in a:spcPts.
func TestParagraphSpacingUnits(t *testing.T) {
	for _, tc := range []struct {
		emu  dml.EMU
		want int32
	}{
		{dml.Points(6), 600},
		{0, 0},
		{190, 1},  // 1.496 hundredths rounds down
		{191, 2},  // 1.504 rounds up
		{-127, 0}, // the schema has no negative spacing
		{dml.Points(2000), 158400},
	} {
		if got := spacingPoints(tc.emu); got != tc.want {
			t.Errorf("spacingPoints(%d) = %d, want %d", tc.emu, got, tc.want)
		}
	}
	if spacingEMU(600) != dml.Points(6) {
		t.Errorf("spacingEMU(600) = %d", spacingEMU(600))
	}

	// The issue's reproduction: 6pt is written as 600, and reads back.
	p := Create()
	s := p.AddSlide()
	para := s.AddTextBox().TextFrame().AddParagraph()
	para.SetSpaceBefore(dml.Points(6))
	para.SetSpaceAfter(dml.Points(3))
	para.AddRun().SetText("x")
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if !strings.Contains(slideXML, `<a:spcBef><a:spcPts val="600"/></a:spcBef>`) || !strings.Contains(slideXML, `<a:spcAft><a:spcPts val="300"/></a:spcAft>`) {
		t.Fatalf("spacing written as:\n%s", slideXML)
	}
	reopened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var got *Paragraph
	for _, shape := range reopened.Slides()[0].Shapes() {
		if tb, ok := shape.(*TextBox); ok {
			got = tb.TextFrame().Paragraphs()[0]
		}
	}
	if got == nil || got.SpaceBefore() != dml.Points(6) || got.SpaceAfter() != dml.Points(3) {
		t.Fatalf("read back: %+v", got)
	}
	// Edited in place on the reopened deck, the new spacing is converted too.
	got.SetSpaceBefore(dml.Points(9))
	data, err = reopened.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if slideXML = string(zipPart(t, data, "ppt/slides/slide1.xml")); !strings.Contains(slideXML, `<a:spcPts val="900"/>`) {
		t.Fatalf("edited spacing written as:\n%s", slideXML)
	}
}
