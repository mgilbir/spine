package pptx

import (
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// C80: paragraph line spacing / space before / space after are serialized, and
// a spacing-only paragraph does not gain a spurious <a:buNone/>.
func TestParagraphToOxml_Spacing(t *testing.T) {
	p := &Paragraph{}
	p.SetLineSpacing(150000) // 150%
	p.SetSpaceBefore(1200)
	p.SetSpaceAfter(600)

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
