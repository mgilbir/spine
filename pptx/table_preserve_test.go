package pptx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// styledTableXML is a bordered/styled a:tbl as PowerPoint authors them,
// carrying styling the domain model does not represent: cell margins
// (marL/marR/marT), per-edge borders, a cell fill, and a tableStyleId.
const styledTableXML = `<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="7" name="StyledTable"/><p:cNvGraphicFramePr><a:graphicFrameLocks noGrp="1"/></p:cNvGraphicFramePr><p:nvPr/></p:nvGraphicFramePr><p:xfrm><a:off x="0" y="0"/><a:ext cx="3657600" cy="914400"/></p:xfrm><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table"><a:tbl><a:tblPr firstRow="1" bandRow="1"><a:tableStyleId>{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}</a:tableStyleId></a:tblPr><a:tblGrid><a:gridCol w="1828800"/><a:gridCol w="1828800"/></a:tblGrid><a:tr h="457200"><a:tc><a:txBody><a:bodyPr/><a:p><a:r><a:t>A1</a:t></a:r></a:p></a:txBody><a:tcPr marL="228600" marT="45720"><a:lnL w="38100"><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></a:lnL><a:lnB w="12700"><a:solidFill><a:srgbClr val="00FF00"/></a:solidFill></a:lnB><a:solidFill><a:srgbClr val="DDEEFF"/></a:solidFill></a:tcPr></a:tc><a:tc><a:txBody><a:bodyPr/><a:p><a:r><a:t>B1</a:t></a:r></a:p></a:txBody><a:tcPr marR="91440"><a:lnT w="25400"><a:solidFill><a:srgbClr val="0000FF"/></a:solidFill></a:lnT></a:tcPr></a:tc></a:tr><a:tr h="457200"><a:tc><a:txBody><a:bodyPr/><a:p><a:r><a:t>A2</a:t></a:r></a:p></a:txBody><a:tcPr marL="228600"/></a:tc><a:tc><a:txBody><a:bodyPr/><a:p><a:r><a:t>B2</a:t></a:r></a:p></a:txBody><a:tcPr/></a:tc></a:tr></a:tbl></a:graphicData></a:graphic></p:graphicFrame>`

// styledTableStylingMarkers are the parsed styling bits that must survive any
// non-structural cell edit untouched.
var styledTableStylingMarkers = []string{
	`<a:tableStyleId>{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}</a:tableStyleId>`,
	`marL="228600" marT="45720"`,
	`<a:lnL w="38100">`,
	`<a:lnB w="12700">`,
	`<a:srgbClr val="DDEEFF"/>`,
	`marR="91440"`,
	`<a:lnT w="25400">`,
}

// styledTableDeck returns a saved deck whose first slide carries the styled
// table above.
func styledTableDeck(t *testing.T) []byte {
	t.Helper()
	return rewriteZipPart(t, savedDeck(t), "ppt/slides/slide1.xml", func(xml []byte) []byte {
		return bytes.Replace(xml, []byte("</p:spTree>"), []byte(styledTableXML+"</p:spTree>"), 1)
	})
}

func tableByName(t *testing.T, p *Presentation, name string) *Table {
	t.Helper()
	tbl, ok := p.Slides()[0].ShapeByName(name).(*Table)
	if !ok {
		t.Fatalf("shape %q is not a *Table", name)
	}
	return tbl
}

func assertStylingSurvives(t *testing.T, slideXML []byte, context string) {
	t.Helper()
	for _, marker := range styledTableStylingMarkers {
		if !bytes.Contains(slideXML, []byte(marker)) {
			t.Errorf("%s: styling %q wiped from slide XML", context, marker)
		}
	}
}

// C174: a single SetText on one cell of a loaded table must not regenerate the
// a:tbl — borders, margins, fills, and the table style of every cell survive.
func TestLoadedTableCellEditPreservesStyling(t *testing.T) {
	p := openBytes(t, styledTableDeck(t))
	tableByName(t, p, "StyledTable").Cell(0, 0).SetText("edited")

	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := zipPart(t, out, "ppt/slides/slide1.xml")
	assertStylingSurvives(t, slideXML, "after cell edit")
	if !bytes.Contains(slideXML, []byte("<a:t>edited</a:t>")) {
		t.Error("edited cell text missing")
	}
	for _, keep := range []string{"<a:t>B1</a:t>", "<a:t>A2</a:t>", "<a:t>B2</a:t>"} {
		if !bytes.Contains(slideXML, []byte(keep)) {
			t.Errorf("untouched cell text %q lost", keep)
		}
	}
}

// C174: edit -> save -> edit -> save keeps styling across cycles, both on the
// same Presentation and through a reopen.
func TestLoadedTableCellEditMultiCycle(t *testing.T) {
	p := openBytes(t, styledTableDeck(t))
	tbl := tableByName(t, p, "StyledTable")

	tbl.Cell(0, 0).SetText("first")
	out1, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	tbl.Cell(1, 1).SetText("second")
	out2, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := zipPart(t, out2, "ppt/slides/slide1.xml")
	assertStylingSurvives(t, slideXML, "same-session second cycle")
	for _, want := range []string{"<a:t>first</a:t>", "<a:t>second</a:t>"} {
		if !bytes.Contains(slideXML, []byte(want)) {
			t.Errorf("edit %q missing after second save", want)
		}
	}

	// Reopen the first save and edit again.
	p2 := openBytes(t, out1)
	tableByName(t, p2, "StyledTable").Cell(1, 0).SetText("third")
	out3, err := p2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML = zipPart(t, out3, "ppt/slides/slide1.xml")
	assertStylingSurvives(t, slideXML, "reopen cycle")
	for _, want := range []string{"<a:t>first</a:t>", "<a:t>third</a:t>"} {
		if !bytes.Contains(slideXML, []byte(want)) {
			t.Errorf("edit %q missing after reopen cycle", want)
		}
	}
}

// C174: explicitly restyling one cell applies that edit without wiping the
// unmodeled properties of that cell or its neighbors.
func TestLoadedTableCellPropEditKeepsUnmodeledProps(t *testing.T) {
	p := openBytes(t, styledTableDeck(t))
	tableByName(t, p, "StyledTable").Cell(0, 1).SetFill(dml.NewRGB(0xAA, 0xBB, 0xCC).ToColor())

	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := zipPart(t, out, "ppt/slides/slide1.xml")
	assertStylingSurvives(t, slideXML, "after cell restyle")
	if !bytes.Contains(slideXML, []byte(`<a:srgbClr val="AABBCC"/>`)) {
		t.Error("explicit cell fill missing")
	}
}

// C174: a structural change (AddRow) still produces a valid table, and the
// surviving cells keep their parsed styling and text.
func TestLoadedTableStructuralChangePreservesSurvivingCells(t *testing.T) {
	p := openBytes(t, styledTableDeck(t))
	tbl := tableByName(t, p, "StyledTable")
	row := tbl.AddRow()
	row.Cell(0).SetText("A3")
	row.Cell(1).SetText("B3")

	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := zipPart(t, out, "ppt/slides/slide1.xml")
	if got := strings.Count(string(slideXML), "<a:tr "); got != 3 {
		t.Errorf("row count = %d, want 3", got)
	}
	assertStylingSurvives(t, slideXML, "after AddRow")
	for _, want := range []string{"<a:t>A1</a:t>", "<a:t>B1</a:t>", "<a:t>A3</a:t>", "<a:t>B3</a:t>"} {
		if !bytes.Contains(slideXML, []byte(want)) {
			t.Errorf("cell text %q missing after AddRow", want)
		}
	}

	// The result must reopen cleanly and keep the new grid shape.
	p2 := openBytes(t, out)
	if got := tableByName(t, p2, "StyledTable").RowCount(); got != 3 {
		t.Errorf("reopened row count = %d, want 3", got)
	}
}
