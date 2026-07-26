package docx

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/docx/internal/oxml"
	"github.com/mgilbir/spine/opc"
)

// chartPart is a chart added through the mutation API, together with the
// embedded workbook that carries its editable data. Both parts are written on
// each save lifecycle (saveNew and saveRoundTrip) via writeChartParts.
type chartPart struct {
	partName  string // /word/charts/chartN.xml
	data      []byte // serialized chart.xml (c:chartSpace)
	embedName string // /word/embeddings/Microsoft_Excel_WorksheetN.xlsx
	embedData []byte // embedded workbook bytes
	relID     string // owner-part relationship id referenced by c:chart r:id
	owner     string // owning part whose rels resolve relID (usually document.xml)
}

// chartDrawingNamespaces are the namespace declarations placed on the
// wp:inline element of a chart drawing: the wordprocessingDrawing wrapper, the
// DrawingML main namespace, the chart namespace, and the relationships
// namespace that carries the c:chart r:id.
const chartDrawingNamespaces = `xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" ` +
	`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
	`xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" ` +
	`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`

// chartGraphicDataURI is the a:graphicData uri that marks a drawing as a chart.
const chartGraphicDataURI = "http://schemas.openxmlformats.org/drawingml/2006/chart"

// embeddedWorkbookRelID is the chart part's relationship id (in the chart
// part's own .rels) targeting its embedded workbook. It is both the rel written
// by AddChart and the r:id of the c:externalData injected into the chart XML.
const embeddedWorkbookRelID = "rId1"

// AddChart appends a paragraph containing an inline chart to the document body
// and returns nothing but an error. It is the ergonomic primary: build a chart
// with the shared chart package (chart.NewColumn(), SetTitle, SetCategories,
// AddSeries, ...), then hand it here with the display size in EMUs (914400 per
// inch). The chart's data is embedded as an editable workbook so Office can
// open and edit it — docx charts have no host worksheet.
func (d *Document) AddChart(c *chart.Chart, widthEMU, heightEMU int64) error {
	p := d.AddParagraph()
	return p.AddChart(c, widthEMU, heightEMU)
}

// AddChart inserts an inline chart into a new run at the end of the paragraph,
// in the text flow like an inline image. The chart's data is written to an
// embedded workbook (word/embeddings/…xlsx) that the chart part references, so
// Office can edit the values.
func (p *Paragraph) AddChart(c *chart.Chart, widthEMU, heightEMU int64) error {
	if c == nil {
		return fmt.Errorf("docx: AddChart: nil chart")
	}
	doc := p.document
	if doc == nil {
		return fmt.Errorf("docx: AddChart: paragraph is not attached to a document")
	}

	// Build the embedded workbook and point the chart's c:f references at its
	// Sheet1 ranges. The layout's sheet is the reference base the chart.xml is
	// serialized against, so the two line up and Office edits the exact ranges.
	embedData, layout, err := c.EmbeddedWorkbook()
	if err != nil {
		return fmt.Errorf("docx: AddChart: %w", err)
	}
	c.SetDataRef(layout.Sheet)

	chartXML, err := c.MarshalChartXML()
	if err != nil {
		return fmt.Errorf("docx: AddChart: %w", err)
	}
	// Point the chart at its embedded workbook so Word's "Edit Data" can open it.
	// The chart part's relationship to the embedded workbook is rId1 (written
	// below); without this c:externalData the workbook is orphaned.
	chartXML = chart.InjectExternalData(chartXML, embeddedWorkbookRelID)

	num := doc.nextChartNumber()
	chartName := fmt.Sprintf("/word/charts/chart%d.xml", num)
	embedName := fmt.Sprintf("/word/embeddings/Microsoft_Excel_Worksheet%d.xlsx", num)

	owner := p.ownerPart()
	relID := fmt.Sprintf("rId%d", doc.nextRelID())

	cp := &chartPart{
		partName:  chartName,
		data:      chartXML,
		embedName: embedName,
		embedData: embedData,
		relID:     relID,
		owner:     owner,
	}
	doc.chartParts = append(doc.chartParts, cp)

	// Relationship from the owning part (document.xml) to the chart part.
	doc.addPartRelationship(owner, &opc.Relationship{
		ID:     relID,
		Type:   opc.RelTypeChart,
		Target: chartName[len("/word/"):],
	})

	// Relationship from the chart part to its embedded workbook (RelType
	// package). Targets are relative to /word/charts/, hence the "../".
	doc.addPartRelationship(chartName, &opc.Relationship{
		ID:     embeddedWorkbookRelID,
		Type:   opc.RelTypePackage,
		Target: "../embeddings/" + embedName[len("/word/embeddings/"):],
	})

	drawing := &oxml.CT_Drawing{RawContent: buildChartDrawingXML(int(num), relID, widthEMU, heightEMU)}
	p.AddRun().r.AppendDrawing(drawing)
	return nil
}

// ownerPart names the part that contains the paragraph's drawings: the
// header/footer part for header/footer paragraphs, otherwise the main document
// part. Chart relationships are registered against it so the c:chart r:id
// resolves from that part's rels.
func (p *Paragraph) ownerPart() string {
	if p.hfPart != "" {
		return p.hfPart
	}
	return p.document.mainPart()
}

// buildChartDrawingXML builds the w:drawing inner content for an inline chart:
// wp:inline → a:graphic → a:graphicData(uri=chart) → c:chart r:id.
func buildChartDrawingXML(drawingID int, relID string, widthEMU, heightEMU int64) []byte {
	xml := fmt.Sprintf(
		`<wp:inline distT="0" distB="0" distL="0" distR="0" `+chartDrawingNamespaces+`>`+
			`<wp:extent cx="%d" cy="%d"/>`+
			`<wp:effectExtent l="0" t="0" r="0" b="0"/>`+
			`<wp:docPr id="%d" name="Chart %d"/>`+
			`<wp:cNvGraphicFramePr/>`+
			`<a:graphic>`+
			`<a:graphicData uri="%s">`+
			`<c:chart r:id="%s"/>`+
			`</a:graphicData>`+
			`</a:graphic>`+
			`</wp:inline>`,
		widthEMU, heightEMU,
		drawingID, drawingID,
		chartGraphicDataURI, relID,
	)
	return []byte(xml)
}

// nextChartNumber returns the smallest positive N for which no
// /word/charts/chartN.xml part exists, scanning the parts preserved from an
// opened package as well as charts added in this session, so names stay
// collision-free across open→add→save cycles.
func (d *Document) nextChartNumber() int {
	used := make(map[int]bool)
	mark := func(name string) {
		const prefix = "/word/charts/chart"
		name = strings.ToLower(name)
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".xml") {
			return
		}
		n, err := strconv.Atoi(name[len(prefix) : len(name)-len(".xml")])
		if err == nil && n > 0 {
			used[n] = true
		}
	}
	for name := range d.preservedParts {
		mark(name)
	}
	for name := range d.otherParts {
		mark(name)
	}
	for _, cp := range d.chartParts {
		mark(cp.partName)
	}
	for n := 1; ; n++ {
		if !used[n] {
			return n
		}
	}
}

// writeChartParts writes every chart added through the mutation API: the chart
// part, its embedded workbook, and the chart part's own rels (linking it to the
// embedded workbook). Called from writeAddedParts on both save lifecycles, so a
// chart added to a document opened from a file is written into the package too.
func (d *Document) writeChartParts(writer *opc.Writer) error {
	for _, cp := range d.chartParts {
		if err := writer.WritePart(cp.partName, opc.ContentTypeChart, cp.data); err != nil {
			return err
		}
		if err := writer.WritePart(cp.embedName, opc.ContentTypeSpreadsheetPackage, cp.embedData); err != nil {
			return err
		}
		if rels := d.relationships[cp.partName]; len(rels) > 0 {
			if err := writer.WritePartRelationships(cp.partName, rels); err != nil {
				return err
			}
		}
	}
	return nil
}

// --- Document.Charts ---

// Charts returns every chart in the document, in document order, parsed into
// chart.Chart definitions. Charts are found by scanning the drawings in every
// body paragraph (including runs nested in hyperlinks and tracked changes) for
// a c:chart reference, resolving its relationship to the chart part, and parsing
// that part. Charts whose part cannot be resolved or parsed are skipped.
func (d *Document) Charts() []*chart.Chart {
	var out []*chart.Chart
	if d.doc() == nil || d.doc().Body == nil {
		return nil
	}
	for _, p := range d.doc().Body.AllParagraphs() {
		para := &Paragraph{document: d, p: p}
		for _, cr := range oxmlParagraphRuns(p) {
			for _, dr := range cr.Drawing {
				if dr == nil {
					continue
				}
				relID := scanChartRelID(dr.RawContent)
				if relID == "" {
					continue
				}
				data := d.resolveChartXML(para.ownerPart(), relID)
				if len(data) == 0 {
					continue
				}
				if c, err := chart.Parse(data); err == nil && c != nil {
					out = append(out, c)
				}
			}
		}
	}
	return out
}

// scanChartRelID returns the r:id of the c:chart element in a drawing's raw
// content, or "" if the drawing carries no chart reference.
func scanChartRelID(raw []byte) string {
	i := bytes.Index(raw, []byte("<c:chart"))
	if i < 0 {
		return ""
	}
	seg := raw[i:]
	if end := bytes.IndexByte(seg, '>'); end >= 0 {
		seg = seg[:end]
	}
	return attrValue(seg, []byte(` r:id="`))
}

// resolveChartXML resolves a chart relationship id in the owning part to the
// chart part's bytes, searching the charts added in this session, the preserved
// package parts, otherParts, and the reader.
func (d *Document) resolveChartXML(owner, relID string) []byte {
	var target string
	for _, rel := range d.relationships[owner] {
		if rel != nil && rel.ID == relID {
			target = opc.ResolvePartName(owner, rel.Target)
			break
		}
	}
	if target == "" {
		return nil
	}
	for _, cp := range d.chartParts {
		if cp.partName == target {
			return cp.data
		}
	}
	if part, ok := d.preservedParts[target]; ok {
		return part.Data
	}
	if part, ok := d.otherParts[target]; ok {
		return part.Data
	}
	if d.reader != nil {
		if f := d.reader.GetFile(target); f != nil {
			if data, err := f.ReadAll(); err == nil {
				return data
			}
		}
	}
	return nil
}
