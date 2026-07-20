package pptx

import (
	"github.com/mgilbir/spine/common/dml/diagram"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// SmartArt is a SmartArt graphic (a "diagram") on a slide: a p:graphicFrame
// whose a:graphicData carries the diagram namespace URI and a dgm:relIds
// element referencing the four diagram parts — data (dgm:dataModel), layout,
// quickStyle, and colors. Reading exposes the diagram's text and hierarchy from
// the data part; the raw parts are preserved verbatim across a round trip, so
// an unmodified save reproduces the SmartArt byte-for-byte.
//
// Creating SmartArt from scratch is not yet supported: a valid diagram also
// requires the layout/quickStyle/colors definition parts and a dsp drawing
// fallback, which PowerPoint rejects if malformed. See the package
// documentation for the current status.
type SmartArt struct {
	// relIDs on the graphicFrame's dgm:relIds, in package-relationship form.
	dataRelID, layoutRelID, styleRelID, colorsRelID string
	// dataPartName is the resolved absolute part name of the data part
	// (dgm:dataModel), e.g. /ppt/diagrams/data1.xml, or "" when unresolved.
	dataPartName string
	// dataModel is the parsed data part, or nil when the part is missing or
	// unparseable.
	dataModel *diagram.DataModel
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
// nodes, so callers can see that a diagram is present.
func (s *Slide) SmartArt() []*SmartArt {
	if s == nil || s.slideXML == nil || s.slideXML.CSld == nil || s.slideXML.CSld.SpTree == nil {
		return nil
	}
	var out []*SmartArt
	for _, gf := range s.slideXML.CSld.SpTree.GraphicFrame {
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
