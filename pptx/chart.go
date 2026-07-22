package pptx

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/common/dml"
	dmlchart "github.com/mgilbir/spine/common/dml/chart"
	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// ChartFrame is the shape a chart occupies on a slide: a p:graphicFrame whose
// graphicData carries a c:chart r:id pointing at a chart part. It is created by
// Slide.AddChart and behaves like any other shape in the sync path, so a chart
// coexists with the slide's existing shapes. The chart part, its embedded data
// workbook, and the wiring relationships are created eagerly by AddChart; this
// frame only records the slide-relative relationship id and geometry.
type ChartFrame struct {
	BaseShape

	// relID is the slide relationship id (RelTypeChart) whose target is the
	// chart part; it becomes the c:chart r:id in the graphicFrame.
	relID string
	// partName is the absolute chart part name (e.g. /ppt/charts/chart1.xml),
	// used by Charts() to load and re-parse the definition.
	partName string
}

// ShapeType reports that a ChartFrame is a chart.
func (cf *ChartFrame) ShapeType() ShapeType { return ShapeTypeChart }

// AddChart places a chart on the slide at (x, y) with the given width and
// height, all in EMUs. The chart's data has no host workbook in a
// presentation, so it is embedded: AddChart writes an .xlsx workbook (from
// c.EmbeddedWorkbook) whose Sheet1 ranges match the chart's c:f references, a
// chart part serialized from c, the chart→workbook package relationship, the
// slide→chart relationship, and the content-type overrides. The graphicFrame
// that references the chart is appended to the slide's shape tree on save,
// alongside any existing shapes.
//
// AddChart returns an error when the chart cannot be serialized (e.g. it has no
// series) or its embedded workbook cannot be built.
func (s *Slide) AddChart(c *chart.Chart, x, y, width, height int64) error {
	if c == nil {
		return fmt.Errorf("pptx: AddChart: chart is nil")
	}
	p := s.presentation
	if p == nil {
		return fmt.Errorf("pptx: AddChart: slide is not attached to a presentation")
	}
	if s.partName == "" {
		// A created slide has no part name until save; assign one now so the
		// chart's relationships and rels file are keyed correctly.
		s.partName = p.nextAvailableSlidePartName()
	}

	// The embedded workbook is the chart's data source; its sheet name is what
	// the c:f references are built against. Keep them aligned.
	c.SetDataRef(chartEmbedSheet)
	workbook, _, err := c.EmbeddedWorkbook()
	if err != nil {
		return fmt.Errorf("pptx: AddChart: build embedded workbook: %w", err)
	}
	chartXML, err := c.MarshalChartXML()
	if err != nil {
		return fmt.Errorf("pptx: AddChart: serialize chart: %w", err)
	}

	// Allocate unique part names across the whole package.
	chartPart := p.nextChartPartName()
	embedPart := p.nextEmbeddingPartName()

	// Chart part → embedded workbook (package relationship), referenced from the
	// chart XML via c:externalData r:id.
	embedRelID := "rId1"
	p.relationships[chartPart] = []*opc.Relationship{{
		ID:         embedRelID,
		Type:       opc.RelTypePackage,
		Target:     relativeTarget(chartPart, embedPart),
		TargetMode: opc.TargetModeInternal,
	}}
	chartXML = injectExternalData(chartXML, embedRelID)

	// Store the chart and workbook parts with their content-type overrides.
	p.otherParts[chartPart] = &coxml.RawPart{ContentType: opc.ContentTypeChart, Data: chartXML}
	p.otherParts[embedPart] = &coxml.RawPart{ContentType: opc.ContentTypeSpreadsheetPackage, Data: workbook}

	// Slide → chart relationship; its id is the graphicFrame's c:chart r:id.
	chartRelID := s.nextRelID()
	p.relationships[s.partName] = append(p.relationships[s.partName], &opc.Relationship{
		ID:         chartRelID,
		Type:       opc.RelTypeChart,
		Target:     relativeTarget(s.partName, chartPart),
		TargetMode: opc.TargetModeInternal,
	})

	cf := &ChartFrame{relID: chartRelID, partName: chartPart}
	cf.SetPosition(dml.EMU(x), dml.EMU(y))
	cf.SetSize(dml.EMU(width), dml.EMU(height))
	s.addShape(cf)
	return nil
}

// chartEmbedSheet is the worksheet name in the embedded workbook (and thus the
// sheet the chart's c:f references point at).
const chartEmbedSheet = "Sheet1"

// chartFrameToOxml converts a ChartFrame to the p:graphicFrame element that
// references its chart part by relationship id.
func chartFrameToOxml(cf *ChartFrame, id uint32) *oxml.GraphicFrame {
	name := cf.Name()
	if name == "" {
		name = "Chart " + strconv.FormatUint(uint64(id), 10)
	}
	x, y := cf.Position()
	w, h := cf.Size()
	return &oxml.GraphicFrame{
		NvGraphicFramePr: &oxml.NvGraphicFramePr{
			CNvPr: &dml.CNvPr{Id: id, Name: name},
			CNvGraphicFramePr: &oxml.CNvGraphicFramePr{
				GraphicFrameLocks: &oxml.GraphicFrameLocks{NoGrp: true},
			},
			NvPr: &oxml.NvPr{},
		},
		Xfrm: &dml.Xfrm{
			Off: &dml.OffXML{X: int64(x), Y: int64(y)},
			Ext: &dml.ExtXML{Cx: int64(w), Cy: int64(h)},
		},
		Graphic: &oxml.AGraphic{
			GraphicData: &oxml.AGraphicData{
				URI:      oxml.ChartGraphicDataURI,
				ChartRef: &dmlchart.RelId{Id: cf.relID},
			},
		},
	}
}

// injectExternalData inserts a c:externalData element (referencing the embedded
// workbook relationship) into a serialized chart part. The shared chart package
// is data-source-agnostic and does not emit it, so the format integration adds
// it: without it, PowerPoint cannot open the workbook to edit the chart data.
// The element is placed immediately before </c:chartSpace>, which is the schema
// position for externalData given the chart package emits no spPr/txPr/
// printSettings/userShapes after c:chart.
func injectExternalData(chartXML []byte, relID string) []byte {
	const closeTag = "</c:chartSpace>"
	idx := bytes.LastIndex(chartXML, []byte(closeTag))
	if idx < 0 {
		return chartXML
	}
	ext := `<c:externalData r:id="` + relID + `"><c:autoUpdate val="0"/></c:externalData>`
	out := make([]byte, 0, len(chartXML)+len(ext))
	out = append(out, chartXML[:idx]...)
	out = append(out, ext...)
	out = append(out, chartXML[idx:]...)
	return out
}

// nextChartPartName returns an unused /ppt/charts/chartN.xml part name.
func (p *Presentation) nextChartPartName() string {
	return p.nextIndexedPartName("/ppt/charts/chart", ".xml")
}

// nextEmbeddingPartName returns an unused embedded-workbook part name. The first
// is Microsoft_Excel_Worksheet.xlsx and subsequent ones carry an index, mirroring
// PowerPoint's own naming.
func (p *Presentation) nextEmbeddingPartName() string {
	base := "/ppt/embeddings/Microsoft_Excel_Worksheet"
	if !p.partNameTaken(base + ".xlsx") {
		return base + ".xlsx"
	}
	for i := 1; ; i++ {
		name := base + strconv.Itoa(i) + ".xlsx"
		if !p.partNameTaken(name) {
			return name
		}
	}
}

// nextIndexedPartName returns prefix+N+suffix for the smallest N>=1 not already
// present in the package.
func (p *Presentation) nextIndexedPartName(prefix, suffix string) string {
	for i := 1; ; i++ {
		name := prefix + strconv.Itoa(i) + suffix
		if !p.partNameTaken(name) {
			return name
		}
	}
}

// partNameTaken reports whether a part name is already used by the source
// package or by a part this session added.
func (p *Presentation) partNameTaken(name string) bool {
	if _, ok := p.otherParts[name]; ok {
		return true
	}
	if p.reader != nil && p.reader.GetFile(name) != nil {
		return true
	}
	return false
}

// Charts returns the chart definitions referenced by this slide's graphic
// frames, parsed from their chart parts. Frames whose chart part is missing or
// unparseable are skipped.
func (s *Slide) Charts() []*chart.Chart {
	if s.presentation == nil {
		return nil
	}
	seen := make(map[string]bool)
	var out []*chart.Chart
	add := func(partName string) {
		if partName == "" || seen[partName] {
			return
		}
		seen[partName] = true
		part := s.presentation.otherParts[partName]
		if part == nil {
			return
		}
		c, err := chart.Parse(part.Data)
		if err != nil {
			return
		}
		out = append(out, c)
	}

	// Graphic frames already in the (parsed or synced) shape tree.
	if s.sx() != nil && s.sx().CSld != nil && s.sx().CSld.SpTree != nil {
		for _, gf := range s.sx().CSld.SpTree.GraphicFrame {
			if relID := chartRelIDOf(gf); relID != "" {
				add(s.relTargetPart(relID))
			}
		}
	}
	// Chart frames added via AddChart but not yet synced into the tree (deduped
	// against the tree scan by part name).
	for _, sh := range s.shapeCache {
		if cf, ok := sh.(*ChartFrame); ok {
			add(cf.partName)
		}
	}
	return out
}

// chartRelIDOf returns the c:chart r:id of a graphic frame whose graphicData is
// a chart, or "" for any other graphic frame.
func chartRelIDOf(gf *oxml.GraphicFrame) string {
	if gf == nil || gf.Graphic == nil || gf.Graphic.GraphicData == nil {
		return ""
	}
	gd := gf.Graphic.GraphicData
	if gd.URI != oxml.ChartGraphicDataURI || gd.ChartRef == nil {
		return ""
	}
	return gd.ChartRef.Id
}

// Charts returns every chart definition in the presentation, in slide order.
func (p *Presentation) Charts() []*chart.Chart {
	var out []*chart.Chart
	for _, s := range p.slides {
		if s != nil {
			out = append(out, s.Charts()...)
		}
	}
	return out
}

// chartRelHasTarget reports whether a chart relationship id on a slide resolves
// to a part the package carries; used by validation.
func (p *Presentation) chartRelHasTarget(s *Slide, relID string) bool {
	target := s.relTargetPart(relID)
	if target == "" {
		return false
	}
	if _, ok := p.otherParts[target]; ok {
		return true
	}
	if p.reader != nil && p.reader.GetFile(target) != nil {
		return true
	}
	return false
}
