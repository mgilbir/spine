package chart

import (
	"archive/zip"
	"bytes"
	"fmt"
	"sort"
	"strconv"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// EmbeddedWorkbook builds a minimal .xlsx workbook (as bytes) holding the
// chart's data laid out to match its c:f references, and returns it together
// with the DataLayout describing the cell ranges. docx and pptx charts embed
// this workbook so Office can edit the data (a document has no host worksheet);
// xlsx charts reference the host sheet directly and do not need it.
//
// The layout places categories (or scatter X) in column A and each series in a
// subsequent column, with the series name in row 1 and its values in rows
// 2..N+1 — the same convention MarshalChartXML builds its references from, so
// the returned layout's references equal those in the emitted chart.xml.
//
// The workbook is a fixed, minimal SpreadsheetML package (one worksheet with
// inline strings, no styles or shared strings) written here directly. That
// keeps chart self-contained: an embedded workbook is the chart's own data in
// the chart's own layout, so producing it needs no dependency on the xlsx
// package.
func (c *Chart) EmbeddedWorkbook() ([]byte, DataLayout, error) {
	dl := c.layout()
	data, err := c.buildEmbeddedWorkbook(dl.Sheet)
	if err != nil {
		return nil, dl, err
	}
	return data, dl, nil
}

// embedCell is one populated cell in the embedded worksheet: either a string
// (a category label or series name) or a number (a data value).
type embedCell struct {
	col    int
	text   string
	number float64
	isText bool
}

// embeddedRows returns the populated cells grouped by 1-based row, in the fixed
// layout the chart's c:f references point at: categories (or scatter X) in
// column A rows 2.., and each series' name in row 1 with its values below, in
// columns B, C, ...
func (c *Chart) embeddedRows() map[int][]embedCell {
	rows := map[int][]embedCell{}
	add := func(row int, cell embedCell) { rows[row] = append(rows[row], cell) }

	if c.kind == KindScatter {
		if len(c.series) > 0 {
			for i, x := range c.series[0].XValues {
				add(i+2, embedCell{col: 1, number: x})
			}
		}
	} else {
		for i, label := range c.categories {
			add(i+2, embedCell{col: 1, text: label, isText: true})
		}
	}
	for si, s := range c.series {
		col := si + 2 // B, C, ...
		add(1, embedCell{col: col, text: s.Name, isText: true})
		for i, v := range s.Values {
			add(i+2, embedCell{col: col, number: v})
		}
	}
	return rows
}

// buildEmbeddedWorkbook serializes the chart's data into a minimal .xlsx
// package with a single worksheet named sheetName.
func (c *Chart) buildEmbeddedWorkbook(sheetName string) ([]byte, error) {
	sheetXML := c.marshalEmbeddedSheet()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	parts := []struct {
		name string
		data []byte
	}{
		{"[Content_Types].xml", []byte(embedContentTypes)},
		{"_rels/.rels", []byte(embedRootRels)},
		{"xl/workbook.xml", embedWorkbookXML(sheetName)},
		{"xl/_rels/workbook.xml.rels", []byte(embedWorkbookRels)},
		{"xl/worksheets/sheet1.xml", sheetXML},
	}
	for _, p := range parts {
		w, err := zw.Create(p.name)
		if err != nil {
			return nil, fmt.Errorf("chart: embedded workbook %s: %w", p.name, err)
		}
		if _, err := w.Write(p.data); err != nil {
			return nil, fmt.Errorf("chart: embedded workbook %s: %w", p.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("chart: embedded workbook: %w", err)
	}
	return buf.Bytes(), nil
}

// marshalEmbeddedSheet renders the worksheet part carrying the chart's data.
func (c *Chart) marshalEmbeddedSheet() []byte {
	rows := c.embeddedRows()
	rowNums := make([]int, 0, len(rows))
	maxRow, maxCol := 0, 0
	for r, cells := range rows {
		rowNums = append(rowNums, r)
		if r > maxRow {
			maxRow = r
		}
		for _, cell := range cells {
			if cell.col > maxCol {
				maxCol = cell.col
			}
		}
	}
	sort.Ints(rowNums)

	b := xmlb.NewSpreadsheetMLBuilder()
	b.WriteHeader()
	b.StartElementWithNS(xmlb.NSSpreadsheetML, "worksheet", []xmlb.NSDecl{{Prefix: "", URI: xmlb.NSSpreadsheetML}})
	if maxRow > 0 && maxCol > 0 {
		b.EmptyElement(xmlb.NSSpreadsheetML, "dimension",
			xmlb.StrAttr("ref", "A1:"+colName(maxCol)+strconv.Itoa(maxRow)))
	}
	b.StartElement(xmlb.NSSpreadsheetML, "sheetData")
	for _, r := range rowNums {
		cells := rows[r]
		sort.Slice(cells, func(i, j int) bool { return cells[i].col < cells[j].col })
		b.StartElement(xmlb.NSSpreadsheetML, "row", xmlb.IntAttr("r", int64(r)))
		for _, cell := range cells {
			ref := colName(cell.col) + strconv.Itoa(r)
			if cell.isText {
				b.StartElement(xmlb.NSSpreadsheetML, "c",
					xmlb.StrAttr("r", ref), xmlb.StrAttr("t", "inlineStr"))
				b.StartElement(xmlb.NSSpreadsheetML, "is")
				var attrs []xmlb.Attr
				if needsSpacePreserve(cell.text) {
					attrs = append(attrs, xmlb.Attr{Name: "xml:space", Value: "preserve"})
				}
				b.WriteElement(xmlb.NSSpreadsheetML, "t", cell.text, attrs...)
				b.EndElement(xmlb.NSSpreadsheetML, "is")
				b.EndElement(xmlb.NSSpreadsheetML, "c")
			} else {
				b.StartElement(xmlb.NSSpreadsheetML, "c", xmlb.StrAttr("r", ref))
				b.WriteElement(xmlb.NSSpreadsheetML, "v", strconv.FormatFloat(cell.number, 'f', -1, 64))
				b.EndElement(xmlb.NSSpreadsheetML, "c")
			}
		}
		b.EndElement(xmlb.NSSpreadsheetML, "row")
	}
	b.EndElement(xmlb.NSSpreadsheetML, "sheetData")
	b.EndElement(xmlb.NSSpreadsheetML, "worksheet")
	_ = b.Finish()
	return b.Bytes()
}

// needsSpacePreserve reports whether s would lose leading/trailing whitespace
// without xml:space="preserve" on its <t> element.
func needsSpacePreserve(s string) bool {
	if s == "" {
		return false
	}
	isWS := func(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }
	return isWS(s[0]) || isWS(s[len(s)-1])
}

// embedWorkbookXML builds xl/workbook.xml declaring one worksheet named
// sheetName (the reference base the chart's c:f formulas resolve against).
func embedWorkbookXML(sheetName string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<sheets><sheet name="` + xmlb.EscapeAttrValue(sheetName) + `" sheetId="1" r:id="rId1"/></sheets>` +
		`</workbook>`)
}

// Fixed boilerplate parts for the minimal embedded package.
const (
	embedContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
		`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
		`</Types>`

	embedRootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>` +
		`</Relationships>`

	embedWorkbookRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>` +
		`</Relationships>`
)
