package pptx

import (
	"fmt"
	"strconv"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/dml/diagram"
	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// SmartArt is a SmartArt graphic (a "diagram") on a slide: a p:graphicFrame
// whose a:graphicData carries the diagram namespace URI and a dgm:relIds
// element referencing the four diagram parts — data (dgm:dataModel), layout,
// quickStyle, and colors. Reading exposes the diagram's text and hierarchy from
// the data part; the raw parts are preserved verbatim across a round trip, so
// an unmodified save reproduces the SmartArt byte-for-byte.
//
// Creating SmartArt from scratch is supported through Slide.AddSmartArt for the
// list, hierarchy, process, and cycle kinds: it generates all four definition parts, the
// content-type overrides, the slide relationships, and the graphicFrame so
// PowerPoint accepts and renders the diagram.
type SmartArt struct {
	// relIDs on the graphicFrame's dgm:relIds, in package-relationship form.
	dataRelID, layoutRelID, styleRelID, colorsRelID string
	// dataPartName is the resolved absolute part name of the data part
	// (dgm:dataModel), e.g. /ppt/diagrams/data1.xml, or "" when unresolved.
	dataPartName string
	// dataModel is the parsed data part, or nil when the part is missing or
	// unparseable.
	dataModel *diagram.DataModel
	// frame is the graphicFrame this diagram was created with (nil for a
	// SmartArt read from a file), used by SetBounds to reposition it.
	frame *SmartArtFrame
}

// SmartArtNode is one node in a SmartArt's text hierarchy: the node's text and
// its child nodes, in document order.
type SmartArtNode struct {
	// Text is the node's text, with paragraphs joined by "\n".
	Text string
	// Children are the node's child nodes.
	Children []*SmartArtNode
}

// Nodes returns the SmartArt's top-level text nodes and their descendants,
// derived from the data part's dgm:dataModel (its points and parent-of
// connections). It returns nil when the data part is missing or has no content
// points.
func (sa *SmartArt) Nodes() []*SmartArtNode {
	if sa == nil || sa.dataModel == nil {
		return nil
	}
	return convertNodes(sa.dataModel.TextTree())
}

// DataPartName reports the absolute part name of the diagram's data part
// (dgm:dataModel), or "" when it could not be resolved. It is exposed for
// callers that need to correlate a SmartArt with the underlying package part.
func (sa *SmartArt) DataPartName() string { return sa.dataPartName }

// convertNodes maps the diagram package's TextNode forest onto the public
// SmartArtNode type.
func convertNodes(in []*diagram.TextNode) []*SmartArtNode {
	if len(in) == 0 {
		return nil
	}
	out := make([]*SmartArtNode, 0, len(in))
	for _, n := range in {
		if n == nil {
			continue
		}
		out = append(out, &SmartArtNode{
			Text:     n.Text,
			Children: convertNodes(n.Children),
		})
	}
	return out
}

// SmartArt returns the SmartArt graphics on this slide, in shape order. Frames
// whose diagram data part is missing or unparseable are still returned, with no
// nodes, so callers can see that a diagram is present. Diagrams created via
// AddSmartArt but not yet synced into the shape tree are included too.
func (s *Slide) SmartArt() []*SmartArt {
	if s == nil {
		return nil
	}
	var out []*SmartArt
	seen := make(map[string]bool)
	if s.sx() != nil && s.sx().CSld != nil && s.sx().CSld.SpTree != nil {
		for _, gf := range s.sx().CSld.SpTree.GraphicFrame {
			relIDs := diagramRelIDsOf(gf)
			if relIDs == nil {
				continue
			}
			sa := &SmartArt{
				dataRelID:   relIDs.Dm,
				layoutRelID: relIDs.Lo,
				styleRelID:  relIDs.Qs,
				colorsRelID: relIDs.Cs,
			}
			if partName := s.relTargetPart(relIDs.Dm); partName != "" {
				sa.dataPartName = partName
				seen[partName] = true
				if s.presentation != nil {
					if part := s.presentation.otherParts[partName]; part != nil {
						if dm, err := diagram.ParseDataModel(part.Data); err == nil {
							sa.dataModel = dm
						}
					}
				}
			}
			out = append(out, sa)
		}
	}
	// Diagrams added this session whose frame has not yet been serialized into
	// the parsed tree (deduped against the tree scan by data part name).
	for _, sh := range s.shapeCache {
		sf, ok := sh.(*SmartArtFrame)
		if !ok || sf.sa == nil {
			continue
		}
		if sf.sa.dataPartName != "" && seen[sf.sa.dataPartName] {
			continue
		}
		out = append(out, sf.sa)
	}
	return out
}

// diagramRelIDsOf returns the dgm:relIds of a graphic frame whose graphicData
// is a diagram, or nil for any other graphic frame.
func diagramRelIDsOf(gf *oxml.GraphicFrame) *diagram.RelIds {
	if gf == nil || gf.Graphic == nil || gf.Graphic.GraphicData == nil {
		return nil
	}
	gd := gf.Graphic.GraphicData
	if gd.URI != oxml.DiagramGraphicDataURI || gd.DiagramRelIds == nil {
		return nil
	}
	return gd.DiagramRelIds
}

// SmartArt returns every SmartArt graphic in the presentation, in slide order.
func (p *Presentation) SmartArt() []*SmartArt {
	var out []*SmartArt
	for _, s := range p.slides {
		if s != nil {
			out = append(out, s.SmartArt()...)
		}
	}
	return out
}

// SmartArtKind selects the diagram layout family that Slide.AddSmartArt
// generates. Each kind ships a complete, schema-valid layout/quickStyle/colors
// definition so PowerPoint accepts and renders the diagram.
type SmartArtKind int

const (
	// SmartArtList is a top-to-bottom list: each top-level node becomes a
	// rounded rectangle stacked vertically.
	SmartArtList SmartArtKind = iota
	// SmartArtHierarchy is a top-down hierarchy / organization chart: the node
	// tree fans out with each node's children in a row beneath it.
	SmartArtHierarchy
	// SmartArtProcess is a left-to-right process: each top-level node becomes a
	// rounded rectangle laid out horizontally, reading as a sequence of steps.
	SmartArtProcess
	// SmartArtCycle is a radial cycle: each top-level node becomes an ellipse
	// arranged evenly around a circle.
	SmartArtCycle
)

func (k SmartArtKind) diagramKind() diagram.Kind {
	switch k {
	case SmartArtHierarchy:
		return diagram.KindHierarchy
	case SmartArtProcess:
		return diagram.KindProcess
	case SmartArtCycle:
		return diagram.KindCycle
	default:
		return diagram.KindList
	}
}

// Default placement for a created diagram: it fills the slide's main content
// region (the geometry PowerPoint uses when inserting SmartArt into a standard
// 16:9 content layout). Callers that need a different rectangle set it on the
// returned SmartArt via SetBounds before saving.
const (
	smartArtDefaultX  = 838200
	smartArtDefaultY  = 365125
	smartArtDefaultCx = 7772400
	smartArtDefaultCy = 4351338
)

// SmartArtFrame is the shape a created SmartArt occupies on a slide: a
// p:graphicFrame whose a:graphicData carries the diagram URI and a dgm:relIds
// referencing the four generated diagram parts. It behaves like any other shape
// in the sync path, so a created diagram coexists with the slide's existing
// shapes. AddSmartArt creates the parts and wiring eagerly; the frame only
// records the slide-relative relationship ids and geometry.
type SmartArtFrame struct {
	BaseShape

	dataRelID, layoutRelID, styleRelID, colorsRelID string
	// sa is the read view of the diagram this frame hosts, so Slide.SmartArt
	// reports a created diagram before it has been synced into the shape tree.
	sa *SmartArt
}

// ShapeType reports that a SmartArtFrame is a diagram.
func (sf *SmartArtFrame) ShapeType() ShapeType { return ShapeTypeDiagram }

// SetBounds positions and sizes the diagram frame, in EMUs. It is exposed on the
// returned SmartArt (see SmartArt.SetBounds) so callers can override the default
// placement before saving.
func (sf *SmartArtFrame) setBounds(x, y, w, h int64) {
	sf.SetPosition(dml.EMU(x), dml.EMU(y))
	sf.SetSize(dml.EMU(w), dml.EMU(h))
}

// AddSmartArt places a SmartArt diagram of the given kind on the slide, built
// from the supplied node outline. A flat list passes top-level nodes with no
// children; a hierarchy nests them (a node's Children become its subordinates).
//
// AddSmartArt generates everything Office needs to accept and render the
// diagram: the data part (dgm:dataModel) carrying the node text and parent-of
// connections, a layout definition (dgm:layoutDef) with the kind's algorithm, a
// quick-style (dgm:styleDef) and color transform (dgm:colorsDef), the four
// content-type overrides, the slide→part relationships, and the graphicFrame
// whose dgm:relIds ties them together. The diagram fills a default content
// region; call SetBounds on the returned SmartArt to move or resize it.
//
// It returns the created diagram as a SmartArt (the same read view returned by
// Slide.SmartArt), so Nodes reports the outline back immediately. It returns nil
// only when the slide is not attached to a presentation.
func (s *Slide) AddSmartArt(kind SmartArtKind, nodes ...*SmartArtNode) *SmartArt {
	p := s.presentation
	if p == nil {
		return nil
	}
	if s.partName == "" {
		// A created slide has no part name until save; assign one now so the
		// diagram's relationships and rels file are keyed correctly.
		s.partName = p.nextAvailableSlidePartName()
	}

	parts := diagram.Build(kind.diagramKind(), toBuildNodes(nodes))

	// Allocate a shared index so the four parts share the ...N.xml suffix Office
	// uses (data1.xml, layout1.xml, quickStyle1.xml, colors1.xml).
	idx := p.nextDiagramIndex()
	dataPart := fmt.Sprintf("/ppt/diagrams/data%d.xml", idx)
	layoutPart := fmt.Sprintf("/ppt/diagrams/layout%d.xml", idx)
	stylePart := fmt.Sprintf("/ppt/diagrams/quickStyle%d.xml", idx)
	colorsPart := fmt.Sprintf("/ppt/diagrams/colors%d.xml", idx)

	p.otherParts[dataPart] = &coxml.RawPart{ContentType: opc.ContentTypeDiagramData, Data: parts.Data}
	p.otherParts[layoutPart] = &coxml.RawPart{ContentType: opc.ContentTypeDiagramLayout, Data: parts.Layout}
	p.otherParts[stylePart] = &coxml.RawPart{ContentType: opc.ContentTypeDiagramStyle, Data: parts.QuickStyle}
	p.otherParts[colorsPart] = &coxml.RawPart{ContentType: opc.ContentTypeDiagramColors, Data: parts.Colors}

	// One slide relationship per part; the ids become the graphicFrame's
	// dgm:relIds (dm/lo/qs/cs).
	addRel := func(relType, target string) string {
		id := s.nextRelID()
		p.relationships[s.partName] = append(p.relationships[s.partName], &opc.Relationship{
			ID:         id,
			Type:       relType,
			Target:     relativeTarget(s.partName, target),
			TargetMode: opc.TargetModeInternal,
		})
		return id
	}
	dmID := addRel(opc.RelTypeDiagramData, dataPart)
	loID := addRel(opc.RelTypeDiagramLayout, layoutPart)
	qsID := addRel(opc.RelTypeDiagramQuickStyle, stylePart)
	csID := addRel(opc.RelTypeDiagramColors, colorsPart)

	sa := &SmartArt{
		dataRelID:    dmID,
		layoutRelID:  loID,
		styleRelID:   qsID,
		colorsRelID:  csID,
		dataPartName: dataPart,
	}
	if dm, err := diagram.ParseDataModel(parts.Data); err == nil {
		sa.dataModel = dm
	}

	sf := &SmartArtFrame{
		dataRelID:   dmID,
		layoutRelID: loID,
		styleRelID:  qsID,
		colorsRelID: csID,
		sa:          sa,
	}
	sf.setBounds(smartArtDefaultX, smartArtDefaultY, smartArtDefaultCx, smartArtDefaultCy)
	sa.frame = sf
	s.addShape(sf)
	return sa
}

// SetBounds positions and sizes a created diagram's frame, in EMUs. It has no
// effect on a SmartArt read from a file (whose frame is preserved verbatim).
func (sa *SmartArt) SetBounds(x, y, width, height int64) {
	if sa == nil || sa.frame == nil {
		return
	}
	sa.frame.setBounds(x, y, width, height)
}

// toBuildNodes converts the public node outline into the diagram package's
// build input, dropping nil nodes.
func toBuildNodes(in []*SmartArtNode) []diagram.BuildNode {
	if len(in) == 0 {
		return nil
	}
	out := make([]diagram.BuildNode, 0, len(in))
	for _, n := range in {
		if n == nil {
			continue
		}
		out = append(out, diagram.BuildNode{Text: n.Text, Children: toBuildNodes(n.Children)})
	}
	return out
}

// nextDiagramIndex returns the smallest N>=1 for which none of the four
// ppt/diagrams/{data,layout,quickStyle,colors}N.xml part names is taken, so a
// created diagram claims a consistent set of names.
func (p *Presentation) nextDiagramIndex() int {
	for i := 1; ; i++ {
		if !p.partNameTaken(fmt.Sprintf("/ppt/diagrams/data%d.xml", i)) &&
			!p.partNameTaken(fmt.Sprintf("/ppt/diagrams/layout%d.xml", i)) &&
			!p.partNameTaken(fmt.Sprintf("/ppt/diagrams/quickStyle%d.xml", i)) &&
			!p.partNameTaken(fmt.Sprintf("/ppt/diagrams/colors%d.xml", i)) {
			return i
		}
	}
}

// smartArtFrameToOxml converts a SmartArtFrame to the p:graphicFrame element
// that references its four diagram parts by relationship id (dgm:relIds).
func smartArtFrameToOxml(sf *SmartArtFrame, id uint32) *oxml.GraphicFrame {
	name := sf.Name()
	if name == "" {
		name = "Diagram " + strconv.FormatUint(uint64(id), 10)
	}
	x, y := sf.Position()
	w, h := sf.Size()
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
				URI: oxml.DiagramGraphicDataURI,
				DiagramRelIds: &diagram.RelIds{
					Dm: sf.dataRelID,
					Lo: sf.layoutRelID,
					Qs: sf.styleRelID,
					Cs: sf.colorsRelID,
				},
			},
		},
	}
}
