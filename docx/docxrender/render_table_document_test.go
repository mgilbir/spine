package docxrender

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/mgilbir/forme/fonts/notosans"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/render"
)

// TestARealisticTableDocumentDrawsInStrictMode draws tables as Word writes
// them: Table Grid, and a banded Grid Table with theme colours, a merged cell and
// a header row, in strict mode.
func TestARealisticTableDocumentDrawsInStrictMode(t *testing.T) {
	letter := `<w:sectPr><w:pgSz w:w="12240" w:h="15840"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720" w:gutter="0"/></w:sectPr>`
	styles := wordTestWordStyles + `<w:style w:type="table" w:styleId="TableGrid"><w:name w:val="Table Grid"/><w:basedOn w:val="TableNormal"/><w:uiPriority w:val="39"/><w:pPr><w:spacing w:after="0" w:line="240" w:lineRule="auto"/></w:pPr><w:tblPr><w:tblBorders><w:top w:val="single" w:sz="4" w:space="0" w:color="auto"/><w:left w:val="single" w:sz="4" w:space="0" w:color="auto"/><w:bottom w:val="single" w:sz="4" w:space="0" w:color="auto"/><w:right w:val="single" w:sz="4" w:space="0" w:color="auto"/><w:insideH w:val="single" w:sz="4" w:space="0" w:color="auto"/><w:insideV w:val="single" w:sz="4" w:space="0" w:color="auto"/></w:tblBorders></w:tblPr></w:style>` +
		`<w:style w:type="table" w:styleId="GridTable4-Accent1"><w:name w:val="Grid Table 4 Accent 1"/><w:basedOn w:val="TableNormal"/><w:uiPriority w:val="49"/><w:pPr><w:spacing w:after="0" w:line="240" w:lineRule="auto"/></w:pPr><w:tblStylePr w:type="firstRow"><w:rPr><w:b/><w:bCs/><w:color w:val="FFFFFF" w:themeColor="background1"/></w:rPr><w:tblPr/><w:tcPr><w:tcBorders><w:top w:val="single" w:sz="4" w:space="0" w:color="4472C4" w:themeColor="accent1"/><w:left w:val="single" w:sz="4" w:space="0" w:color="4472C4" w:themeColor="accent1"/><w:bottom w:val="single" w:sz="4" w:space="0" w:color="4472C4" w:themeColor="accent1"/><w:right w:val="single" w:sz="4" w:space="0" w:color="4472C4" w:themeColor="accent1"/><w:insideH w:val="nil"/><w:insideV w:val="nil"/></w:tcBorders><w:shd w:val="clear" w:color="auto" w:fill="4472C4" w:themeFill="accent1"/></w:tcPr></w:tblStylePr><w:tblStylePr w:type="band1Horz"><w:tblPr/><w:tcPr><w:shd w:val="clear" w:color="auto" w:fill="D9E2F3" w:themeFill="accent1" w:themeFillTint="33"/></w:tcPr></w:tblStylePr><w:tblPr><w:tblStyleRowBandSize w:val="1"/><w:tblStyleColBandSize w:val="1"/><w:tblBorders><w:top w:val="single" w:sz="4" w:space="0" w:color="8EAADB" w:themeColor="accent1" w:themeTint="99"/><w:left w:val="single" w:sz="4" w:space="0" w:color="8EAADB"/><w:bottom w:val="single" w:sz="4" w:space="0" w:color="8EAADB"/><w:right w:val="single" w:sz="4" w:space="0" w:color="8EAADB"/><w:insideH w:val="single" w:sz="4" w:space="0" w:color="8EAADB"/><w:insideV w:val="single" w:sz="4" w:space="0" w:color="8EAADB"/></w:tblBorders></w:tblPr></w:style>`
	cell := func(w int, tcpr, text string) string {
		return `<w:tc><w:tcPr><w:tcW w:w="` + strconv.Itoa(w) + `" w:type="dxa"/>` + tcpr + `</w:tcPr><w:p><w:r><w:t>` + text + `</w:t></w:r></w:p></w:tc>`
	}
	row := func(hdr bool, cells ...string) string {
		pr := ""
		if hdr {
			pr = `<w:trPr><w:cantSplit/><w:tblHeader/></w:trPr>`
		}
		return `<w:tr>` + pr + strings.Join(cells, "") + `</w:tr>`
	}
	grid := `<w:tblGrid><w:gridCol w:w="3120"/><w:gridCol w:w="3120"/><w:gridCol w:w="3120"/></w:tblGrid>`
	t1 := `<w:tbl><w:tblPr><w:tblStyle w:val="TableGrid"/><w:tblW w:w="0" w:type="auto"/><w:tblLook w:val="04A0" w:firstRow="1" w:lastRow="0" w:firstColumn="1" w:lastColumn="0" w:noHBand="0" w:noVBand="1"/></w:tblPr>` + grid +
		row(true, cell(3120, "", "Name"), cell(3120, "", "Quantity"), cell(3120, "", "Price")) +
		row(false, cell(3120, "", "Apples"), cell(3120, "", "12"), cell(3120, `<w:shd w:val="clear" w:color="auto" w:fill="FFFF00"/>`, "1.20")) +
		row(false, cell(6240, `<w:gridSpan w:val="2"/>`, "Merged across two columns"), cell(3120, "", "3.40")) +
		row(false, cell(3120, `<w:vMerge w:val="restart"/><w:vAlign w:val="center"/>`, "Tall merged"), cell(3120, "", "x"), cell(3120, "", "y")) +
		row(false, cell(3120, `<w:vMerge/>`, ""), cell(3120, "", "x2"), cell(3120, "", "y2")) + `</w:tbl>`
	t2 := `<w:tbl><w:tblPr><w:tblStyle w:val="GridTable4-Accent1"/><w:tblW w:w="5000" w:type="pct"/><w:tblLook w:val="04A0" w:firstRow="1" w:lastRow="0" w:firstColumn="1" w:lastColumn="0" w:noHBand="0" w:noVBand="1"/></w:tblPr>` + grid +
		row(true, cell(3120, "", "Name"), cell(3120, "", "Quantity"), cell(3120, "", "Price")) +
		row(false, cell(3120, "", "Apples"), cell(3120, "", "12"), cell(3120, "", "1.20")) +
		row(false, cell(3120, "", "Pears"), cell(3120, "", "7"), cell(3120, "", "2.00")) +
		row(false, cell(3120, "", "Plums"), cell(3120, "", "9"), cell(3120, "", "2.50")) + `</w:tbl>`
	body := `<w:p><w:r><w:t>Table Grid:</w:t></w:r></w:p>` + t1 + `<w:p/><w:p><w:r><w:t>Grid Table 4:</w:t></w:r></w:p>` + t2 + `<w:p/>` + letter
	doc := wordTestDoc(t, body, wordTestParts{styles: styles, settings: wordTestWordSettings, theme: wordTestWordTheme})
	face, err := notosans.Face()
	if err != nil {
		t.Fatal(err)
	}
	opts := render.Options{Limits: render.Limits{MaxShapeWork: 1 << 30}, Fonts: func(_ context.Context, r render.FontRequest) (*shape.Face, error) { return face, nil }}
	pages, err := Prepare(context.Background(), doc, opts)
	if err != nil {
		t.Fatal(err)
	}
	if pages.Count() != 1 {
		t.Fatalf("%d pages", pages.Count())
	}
	// The header row of the Grid Table is accent 1 and the first band accent 1
	// at 20 percent (the theme tint); the highlighted cell is Word's yellow.
	fills := map[string]bool{}
	for _, r := range pages.rects(1) {
		fills[rgbOf(r.Color)] = true
	}
	for _, want := range []string{"4472c4", "dae3f3", "ffff00", "8eaadb"} {
		if !fills[want] {
			t.Errorf("no %s fill or border: %v", want, fills)
		}
	}
	if _, ok := pages.textOf(1, "columns"); !ok {
		t.Errorf("merged cell text not drawn: %v", pages.texts(1))
	}
}
