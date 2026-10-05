package docxrender

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mgilbir/spine/render"
)

// tableCase is a table feature that is not drawn exactly.
type tableCase struct {
	name string
	// props are the table properties; rows replaces the default one-cell row.
	props, rows string
	grid        []int
	warn        string
	approx      bool
}

func tableCases() []tableCase {
	cell := func(tcpr string) string {
		return wordTestRow("", wordTestCell(tcpr, "cell"))
	}
	return []tableCase{
		{name: "floating", props: `<w:tblpPr w:leftFromText="180" w:tblpX="100"/>`, warn: "floating tables", approx: true},
		{name: "cell spacing", props: `<w:tblCellSpacing w:w="30" w:type="dxa"/>`, warn: "cell spacing", approx: true},
		{name: "right to left", props: `<w:bidiVisual/>`, warn: "right-to-left tables", approx: true},
		{name: "autofit width differs from the grid", props: `<w:tblW w:w="6000" w:type="dxa"/>`, warn: "autofit table width", approx: true},
		{name: "fixed width differs from the grid", props: wordTestFixed + `<w:tblW w:w="6000" w:type="dxa"/>`, warn: "different from its grid", approx: true},
		{name: "unknown property", props: `<w:futureTableProp/>`, warn: "futureTableProp"},
		{name: "table shading pattern", props: `<w:shd w:val="diagStripe" w:color="FF0000" w:fill="FFFFFF"/>`, warn: "table shading pattern", approx: true},
		{name: "diagonal border", rows: cell(`<w:tcBorders><w:tl2br w:val="single" w:sz="4"/></w:tcBorders>`), warn: "diagonal cell borders"},
		{name: "wave border", rows: cell(`<w:tcBorders><w:top w:val="wave" w:sz="4"/></w:tcBorders>`), warn: "border style wave", approx: true},
		{name: "thick border", props: `<w:tblBorders><w:top w:val="thick" w:sz="4"/></w:tblBorders>`, warn: "border style thick", approx: true},
		{name: "cell shading pattern", rows: cell(`<w:shd w:val="pct25" w:color="FF0000" w:fill="FFFFFF"/>`), warn: "cell shading pattern", approx: true},
		{name: "cell text direction", rows: cell(`<w:textDirection w:val="btLr"/>`), warn: "cell text direction", approx: true},
		{name: "cell text fitting", rows: cell(`<w:tcFitText/>`), warn: "cell text fitting", approx: true},
		{name: "justified vertical alignment", rows: cell(`<w:vAlign w:val="both"/>`), warn: "justified vertical", approx: true},
		{name: "no wrap cell in an autofit table", rows: cell(`<w:noWrap/>`), warn: "must not wrap", approx: true},
		{name: "unknown cell property", rows: cell(`<w:futureCellProp/>`), warn: "futureCellProp"},
		{name: "unknown row property", rows: wordTestRow(`<w:futureRowProp/>`, wordTestCell("", "cell")), warn: "futureRowProp"},
		{name: "row alignment", rows: wordTestRow(`<w:jc w:val="center"/>`, wordTestCell("", "cell")), warn: "row alignment", approx: true},
		{name: "tracked row", rows: wordTestRow(`<w:ins w:id="1" w:author="a" w:date="2020-01-01T00:00:00Z"/>`, wordTestCell("", "cell")), warn: "tracked row", approx: true},
		{name: "page break in a cell", rows: wordTestRow("", wordTestCell("", wordTestPara("", `<w:r><w:t>cell</w:t><w:br w:type="page"/><w:t>more</w:t></w:r>`))), warn: "page breaks inside table cells", approx: true},
		{name: "page break before in a cell", rows: wordTestRow("", wordTestCell("", wordTestPara(`<w:pageBreakBefore/>`, wordTestRun("", "cell")))), warn: "page breaks inside table cells", approx: true},
		{name: "cell width differs from the grid", rows: cell(`<w:tcW w:w="3000" w:type="dxa"/>`), warn: "cell widths different", approx: true},
		{name: "hidden text in merged cell", rows: wordTestRow("", wordTestCell(`<w:vMerge w:val="restart"/>`, "cell")) + wordTestRow("", wordTestCell(`<w:vMerge/>`, "hidden")), warn: "vertically merged cells"},
		{name: "grid without widths", grid: []int{}, warn: "table grid without column widths", approx: true},
		{name: "unknown table element", rows: wordTestRow("", wordTestCell("", "cell")) + `<w:futureRowGroup/>`, warn: "futureRowGroup"},
		{name: "unknown row element", rows: `<w:tr><w:futureCell/>` + wordTestCell("", "cell") + `</w:tr>`, warn: "futureCell"},
		{name: "inside the cell: unknown block", rows: wordTestRow("", wordTestCell("", wordTestPara("", wordTestRun("", "cell"))+`<w:futureBlock/>`)), warn: "futureBlock"},
	}
}

func (c tableCase) body() string {
	grid := c.grid
	if grid == nil {
		grid = []int{1500}
	}
	rows := c.rows
	if rows == "" {
		rows = wordTestRow("", wordTestCell("", "cell"))
	}
	table := wordTestTable(c.props, grid, rows)
	if len(grid) == 0 {
		table = strings.Replace(table, "<w:tblGrid></w:tblGrid>", "<w:tblGrid/>", 1)
	}
	return wordTestPara("", wordTestRun("", "before")) + table + wordTestPara("", wordTestRun("", "after")) + wordTestPage
}

func TestTableStrictModeRefusesWhatItCannotDrawExactly(t *testing.T) {
	for _, c := range tableCases() {
		t.Run(c.name, func(t *testing.T) {
			o := newWordTestOpts(t, false)
			p, err := Prepare(context.Background(), wordTestDoc(t, c.body(), wordTestParts{styles: wordTestStyles}), o.Options)
			if p != nil || !errors.Is(err, render.ErrUnsupported) {
				t.Fatalf("got %v, %v", p, err)
			}
		})
	}
}

func TestTableBestEffortReportsAndDrawsTheRest(t *testing.T) {
	for _, c := range tableCases() {
		t.Run(c.name, func(t *testing.T) {
			o := newWordTestOpts(t, true)
			p, err := Prepare(context.Background(), wordTestDoc(t, c.body(), wordTestParts{styles: wordTestStyles}), o.Options)
			if err != nil {
				t.Fatal(err)
			}
			var found error
			for _, w := range o.warnings {
				if strings.Contains(w.Error(), c.warn) {
					found = w
				}
			}
			if found == nil {
				t.Fatalf("no warning containing %q in %v", c.warn, o.warnings)
			}
			if got := errors.Is(found, render.ErrApproximated); got != c.approx {
				t.Errorf("approximated = %v, want %v: %v", got, c.approx, found)
			}
			if !c.approx && !errors.Is(found, render.ErrUnsupported) {
				t.Errorf("left-out warning does not wrap ErrUnsupported: %v", found)
			}
			var all strings.Builder
			for pg := 1; pg <= p.Count(); pg++ {
				for _, tx := range p.texts(pg) {
					all.WriteString(tx.text)
				}
			}
			if !strings.HasPrefix(all.String(), "before") || !strings.HasSuffix(all.String(), "after") {
				t.Errorf("surrounding paragraphs drawn as %q", all.String())
			}
		})
	}
}

func TestTableFullyExactTableIsDrawnInStrictMode(t *testing.T) {
	style := `<w:style w:type="table" w:styleId="Fancy"><w:name w:val="Fancy"/><w:basedOn w:val="TableNormal"/><w:pPr><w:spacing w:after="0"/></w:pPr><w:tblPr><w:tblBorders><w:top w:val="single" w:sz="8" w:color="auto"/><w:insideH w:val="double" w:sz="4"/></w:tblBorders><w:tblStyleRowBandSize w:val="2"/></w:tblPr>` +
		`<w:tblStylePr w:type="firstRow"><w:rPr><w:b/></w:rPr></w:tblStylePr><w:tblStylePr w:type="lastCol"><w:tcPr><w:shd w:val="solid" w:color="DDDDDD"/></w:tcPr></w:tblStylePr><w:tblStylePr w:type="band2Horz"><w:tcPr><w:shd w:val="clear" w:fill="EEEEEE"/></w:tcPr></w:tblStylePr></w:style>`
	inner := wordTestTable(wordTestFixed, []int{600}, wordTestRow("", wordTestCell("", "n")))
	rows := []string{
		wordTestRow(`<w:tblHeader/><w:cantSplit/><w:trHeight w:val="300" w:hRule="atLeast"/>`,
			wordTestCell(`<w:gridSpan w:val="2"/><w:tcW w:w="3000" w:type="dxa"/><w:shd w:val="clear" w:fill="FF0000"/>`, "head")),
		wordTestRow("", wordTestCell(`<w:vMerge w:val="restart"/><w:vAlign w:val="center"/><w:tcBorders><w:left w:val="dotted" w:sz="4"/></w:tcBorders>`, "m"), wordTestCell(`<w:tcMar><w:left w:w="200" w:type="dxa"/></w:tcMar>`, inner+wordTestPara("", wordTestRun("", "end")))),
		wordTestRow("", wordTestCell(`<w:vMerge/>`, wordTestPara("")), wordTestCell("", "z")),
	}
	body := wordTestBody(wordTestTable(`<w:tblStyle w:val="Fancy"/><w:tblW w:w="3000" w:type="dxa"/><w:tblInd w:w="100" w:type="dxa"/><w:tblLook w:firstRow="1" w:lastColumn="1" w:noVBand="1"/>`, []int{1500, 1500}, rows...))
	o := newWordTestOpts(t, false)
	if _, err := Prepare(context.Background(), wordTestDoc(t, body, wordTestParts{styles: wordTestStyles + wordTestNormalTable + style}), o.Options); err != nil {
		t.Fatalf("strict: %v", err)
	}
}

func TestTableAutofitWithItsGridIsExact(t *testing.T) {
	// Word's default is autofit, and it stores the widths it computed.
	body := wordTestBody(wordTestTable(`<w:tblW w:w="0" w:type="auto"/>`, []int{1500, 1500},
		wordTestRow("", wordTestCell(`<w:tcW w:w="1500" w:type="dxa"/>`, "a"), wordTestCell(`<w:tcW w:w="1500" w:type="dxa"/>`, "b"))))
	p, _ := wordTestRender(t, body)
	wordTestAt(t, p, 1, "b", 120, base(20))
}

func TestTableAutofitScalesToItsPreferredWidth(t *testing.T) {
	// A percentage width that disagrees with the grid scales the columns.
	body := wordTestBody(wordTestTable(`<w:tblW w:w="5000" w:type="pct"/>`, []int{1500, 1500},
		wordTestRow("", wordTestCell("", "a"), wordTestCell("", "b"))))
	o := newWordTestOpts(t, true)
	p := wordTestPages(t, wordTestDoc(t, body, wordTestParts{styles: wordTestStyles}), o)
	wordTestAt(t, p, 1, "b", 20+130, base(20))
	if len(o.warnings) != 1 {
		t.Errorf("%v", o.warnings)
	}
}

func TestTableSectionPropertiesInACellEndNothing(t *testing.T) {
	body := wordTestBody(wordTestTable(wordTestFixed, []int{1500},
		wordTestRow("", wordTestCell("", wordTestPara(`<w:sectPr><w:type w:val="nextPage"/></w:sectPr>`, wordTestRun("", "a"))))),
		wordTestPara("", wordTestRun("", "b")))
	p, _ := wordTestRender(t, body)
	if p.Count() != 1 {
		t.Errorf("%d pages", p.Count())
	}
}

func TestTableGridBeforeAndAfter(t *testing.T) {
	body := wordTestBody(wordTestTable(wordTestFixed, []int{1500, 1500, 1500},
		wordTestRow(`<w:gridBefore w:val="1"/><w:gridAfter w:val="1"/>`, wordTestCell("", "mid")),
		wordTestRow("", wordTestCell("", "a"), wordTestCell("", "b"), wordTestCell("", "c"))))
	p, _ := wordTestRender(t, body)
	wordTestAt(t, p, 1, "mid", 120, base(20))
	wordTestAt(t, p, 1, "c", 220, base(32))
}

func TestTableLegacyHorizontalMerge(t *testing.T) {
	body := wordTestBody(wordTestTable(wordTestFixed, []int{1500, 1500, 1500},
		wordTestRow("", wordTestCell(`<w:hMerge w:val="restart"/>`, "wide"), wordTestCell(`<w:hMerge w:val="continue"/>`, wordTestPara("")), wordTestCell("", "r")),
		wordTestRow("", wordTestCell("", "a"), wordTestCell("", "b"), wordTestCell("", "c"))))
	p, _ := wordTestRender(t, body)
	wordTestAt(t, p, 1, "r", 220, base(20))
	wordTestAt(t, p, 1, "b", 120, base(32))
}

func TestTableHiddenRowsAreNotDrawn(t *testing.T) {
	body := wordTestBody(wordTestTable(wordTestFixed, []int{1500},
		wordTestRow("", wordTestCell("", "a")),
		wordTestRow(`<w:hidden/>`, wordTestCell("", "b")),
		wordTestRow("", wordTestCell("", "c"))))
	p, _ := wordTestRender(t, body)
	if _, ok := p.textOf(1, "b"); ok {
		t.Errorf("hidden row drawn")
	}
	wordTestAt(t, p, 1, "c", 20, base(32))
}

func TestTableRowExceptionAndCellMargins(t *testing.T) {
	body := wordTestBody(wordTestTable(wordTestFixed+`<w:tblCellMar><w:left w:w="150" w:type="dxa"/></w:tblCellMar>`, []int{1500, 1500},
		wordTestRow("", wordTestCell("", "a"), wordTestCell(`<w:tcMar><w:left w:w="300" w:type="dxa"/></w:tcMar>`, "b")),
		`<w:tr><w:tblPrEx><w:tblCellMar><w:left w:w="0" w:type="dxa"/></w:tblCellMar></w:tblPrEx>`+wordTestCell("", "c")+wordTestCell("", "d")+`</w:tr>`))
	p, _ := wordTestRender(t, body, wordTestParts{styles: wordTestStyles, settings: wordTestCompat15})
	wordTestAt(t, p, 1, "a", 30, base(20))
	wordTestAt(t, p, 1, "b", 120+20, base(20))
	wordTestAt(t, p, 1, "c", 20, base(32))
	wordTestAt(t, p, 1, "d", 120, base(32))
}

func TestTableStyleConditionalLayersAndBasedOn(t *testing.T) {
	styles := wordTestStyles + wordTestNormalTable +
		`<w:style w:type="table" w:styleId="Base"><w:name w:val="Base"/><w:basedOn w:val="TableNormal"/><w:tblStylePr w:type="lastRow"><w:tcPr><w:shd w:val="clear" w:fill="AA0000"/></w:tcPr></w:tblStylePr><w:tblStylePr w:type="firstCol"><w:tcPr><w:shd w:val="clear" w:fill="00AA00"/></w:tcPr></w:tblStylePr></w:style>` +
		`<w:style w:type="table" w:styleId="Derived"><w:name w:val="Derived"/><w:basedOn w:val="Base"/><w:tblStylePr w:type="firstCol"><w:tcPr><w:shd w:val="clear" w:fill="0000AA"/></w:tcPr></w:tblStylePr><w:tblStylePr w:type="nwCell"><w:tcPr><w:shd w:val="clear" w:fill="AAAA00"/></w:tcPr></w:tblStylePr></w:style>`
	// Bits of w:val: 0x40 last row, 0x80 first column, 0x20 first row, 0x200 and 0x400 no bands.
	look := `<w:tblLook w:val="06E0"/>`
	body := wordTestBody(wordTestTable(wordTestFixed+`<w:tblStyle w:val="Derived"/>`+look, []int{1500, 1500},
		wordTestRow("", wordTestCell("", "a"), wordTestCell("", "b")),
		wordTestRow("", wordTestCell("", "c"), wordTestCell("", "d")),
		wordTestRow("", wordTestCell("", "e"), wordTestCell("", "f"))))
	p, _ := wordTestRender(t, body, wordTestParts{styles: styles, settings: wordTestCompat15})
	// The corner cell takes the corner layer, the first column the derived
	// style's first column, and the last row, applied after it, the inherited
	// last row shading even in the first column.
	for _, want := range []struct {
		x, y  float64
		color string
	}{{20, 20, "aaaa00"}, {20, 32, "0000aa"}, {20, 44, "aa0000"}, {120, 44, "aa0000"}} {
		if !hasRect(p, 1, want.x, want.y, 100, 12, want.color) {
			t.Errorf("no %s fill at (%v, %v)", want.color, want.x, want.y)
		}
	}
	for _, r := range p.rects(1) {
		if near(r.Rect.X.Px(), 120) && near(r.Rect.Y.Px(), 20) && rgbOf(r.Color) != "ffffff" {
			t.Errorf("cell b is filled: %v", rgbOf(r.Color))
		}
	}
}

func TestTableColumnBanding(t *testing.T) {
	styles := wordTestStyles + wordTestNormalTable +
		`<w:style w:type="table" w:styleId="Cols"><w:name w:val="Cols"/><w:basedOn w:val="TableNormal"/><w:tblPr><w:tblStyleColBandSize w:val="1"/></w:tblPr><w:tblStylePr w:type="band1Vert"><w:tcPr><w:shd w:val="clear" w:fill="C0C0C0"/></w:tcPr></w:tblStylePr></w:style>`
	body := wordTestBody(wordTestTable(wordTestFixed+`<w:tblStyle w:val="Cols"/><w:tblLook w:noHBand="1" w:noVBand="0" w:firstColumn="0" w:firstRow="0"/>`, []int{1500, 1500, 1500},
		wordTestRow("", wordTestCell("", "a"), wordTestCell("", "b"), wordTestCell("", "c"))))
	p, _ := wordTestRender(t, body, wordTestParts{styles: styles, settings: wordTestCompat15})
	if !hasRect(p, 1, 20, 20, 100, 12, "c0c0c0") || !hasRect(p, 1, 220, 20, 100, 12, "c0c0c0") {
		t.Errorf("odd columns not shaded: %v", p.rects(1))
	}
	if hasRect(p, 1, 120, 20, 100, 12, "c0c0c0") {
		t.Errorf("even column shaded")
	}
}

func TestTableUnknownStyleFallsBackToDefault(t *testing.T) {
	styles := wordTestStyles + wordTestNormalTable
	body := wordTestBody(wordTestTable(wordTestFixed+`<w:tblStyle w:val="NoSuchStyle"/>`, []int{1500},
		wordTestRow("", wordTestCell("", "a"))))
	p, _ := wordTestRender(t, body, wordTestParts{styles: styles, settings: wordTestCompat15})
	// The default table style's margin still applies.
	wordTestAt(t, p, 1, "a", 20+7.2, base(20))
}

func TestTableStyleCycleIsInvalid(t *testing.T) {
	styles := wordTestStyles + `<w:style w:type="table" w:styleId="A"><w:name w:val="A"/><w:basedOn w:val="B"/></w:style><w:style w:type="table" w:styleId="B"><w:name w:val="B"/><w:basedOn w:val="A"/></w:style>`
	body := wordTestBody(wordTestTable(`<w:tblStyle w:val="A"/>`, []int{1500}, wordTestRow("", wordTestCell("", "a"))))
	_, err := Prepare(context.Background(), wordTestDoc(t, body, wordTestParts{styles: styles}), newWordTestOpts(t, true).Options)
	if !errors.Is(err, render.ErrInvalid) {
		t.Errorf("got %v", err)
	}
}

func TestTableLimits(t *testing.T) {
	t.Run("nesting", func(t *testing.T) {
		inner := wordTestTable(wordTestFixed, []int{600}, wordTestRow("", wordTestCell("", "x")))
		for range wordMaxTableDepth + 2 {
			inner = wordTestTable(wordTestFixed, []int{600}, wordTestRow("", wordTestCell("", inner+wordTestPara(""))))
		}
		o := newWordTestOpts(t, true)
		o.MaxLayoutNodes = 1 << 20
		_, err := Prepare(context.Background(), wordTestDoc(t, wordTestBody(inner)), o.Options)
		if !errors.Is(err, render.ErrLimit) {
			t.Errorf("got %v", err)
		}
	})
	t.Run("grid span", func(t *testing.T) {
		body := wordTestBody(wordTestTable("", []int{600}, wordTestRow("", wordTestCell(`<w:gridSpan w:val="100000"/>`, "x"))))
		_, err := Prepare(context.Background(), wordTestDoc(t, body), newWordTestOpts(t, true).Options)
		if !errors.Is(err, render.ErrInvalid) {
			t.Errorf("got %v", err)
		}
	})
	t.Run("columns", func(t *testing.T) {
		var cells []string
		grid := make([]int, wordMaxGridCols+1)
		for range grid {
			cells = append(cells, wordTestCell("", "x"))
		}
		body := wordTestBody(wordTestTable("", grid, wordTestRow("", cells...)))
		_, err := Prepare(context.Background(), wordTestDoc(t, body), newWordTestOpts(t, true).Options)
		if !errors.Is(err, render.ErrLimit) {
			t.Errorf("got %v", err)
		}
	})
	t.Run("nodes", func(t *testing.T) {
		var rows []string
		for range 2000 {
			rows = append(rows, wordTestRow("", wordTestCell("", "x")))
		}
		o := newWordTestOpts(t, true)
		o.MaxLayoutNodes = 1000
		_, err := Prepare(context.Background(), wordTestDoc(t, wordTestBody(wordTestTable("", []int{600}, rows...))), o.Options)
		if !errors.Is(err, render.ErrLimit) {
			t.Errorf("got %v", err)
		}
	})
}

func TestTableMarkupIsEscaped(t *testing.T) {
	// Nothing but numbers and escaped text from the document reaches the markup.
	body := wordTestBody(wordTestTable(`<w:tblStyle w:val="x&quot;&gt;&lt;script"/>`, []int{1500},
		wordTestRow("", wordTestCell(`<w:tcBorders><w:top w:val="single" w:color="red;x" w:sz="9999999999"/></w:tcBorders>`, "x"))))
	_, err := Prepare(context.Background(), wordTestDoc(t, body, wordTestParts{styles: wordTestStyles}), newWordTestOpts(t, true).Options)
	if !errors.Is(err, render.ErrInvalid) {
		t.Fatalf("got %v", err)
	}
	body = wordTestBody(wordTestTable(`<w:tblStyle w:val="x&quot;&gt;&lt;script"/>`, []int{1500},
		wordTestRow("", wordTestCell("", wordTestPara("", wordTestRun("", "&lt;b&gt;&amp;"))))))
	p, _ := wordTestRender(t, body)
	if _, ok := p.textOf(1, "<b>&"); !ok {
		t.Errorf("text not drawn literally: %v", p.texts(1))
	}
}

func TestTableManyPagesOfCellsStayWithinBudget(t *testing.T) {
	var rows []string
	for i := range 300 {
		rows = append(rows, wordTestRow("", wordTestCell("", fmt.Sprint("r", i)), wordTestCell("", "x")))
	}
	o := newWordTestOpts(t, false)
	o.MaxLayoutNodes = 1 << 20
	if _, err := Prepare(context.Background(), wordTestDoc(t, wordTestBody(wordTestTable(wordTestFixed, []int{1500, 1500}, rows...)), wordTestParts{styles: wordTestStyles}), o.Options); err != nil {
		t.Fatal(err)
	}
}
