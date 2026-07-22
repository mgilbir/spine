package pptx

import (
	"bytes"
	"encoding/xml"
	"strconv"
)

// ZoomKind identifies the type of a PowerPoint Zoom object embedded in a
// slide's shape tree.
type ZoomKind string

const (
	// ZoomKindSlide is a Slide Zoom: a thumbnail that jumps to a single slide.
	ZoomKindSlide ZoomKind = "slide"
	// ZoomKindSection is a Section Zoom: a thumbnail that jumps to a section.
	ZoomKindSection ZoomKind = "section"
	// ZoomKindSummary is a Summary Zoom: a landing object linking several
	// sections.
	ZoomKindSummary ZoomKind = "summary"
)

// Zoom graphicData URIs (MS-PPTX §2.8). A Zoom object is a p:graphicFrame whose
// a:graphicData carries one of these URIs and wraps the matching zoom element
// (pslz:sldZm, psez:sectionZm, psuz:summaryZm).
const (
	zoomSlideURI   = "http://schemas.microsoft.com/office/powerpoint/2016/slidezoom"
	zoomSectionURI = "http://schemas.microsoft.com/office/powerpoint/2016/sectionzoom"
	zoomSummaryURI = "http://schemas.microsoft.com/office/powerpoint/2016/summaryzoom"
)

// ZoomLink describes one Zoom object (a p:graphicFrame carrying a slide,
// section, or summary zoom) found on a slide. It is a read-only view: the zoom
// frame round-trips verbatim, but creating a zoom is not supported because a
// zoom requires a rendered thumbnail image of its target that this library
// cannot produce.
type ZoomLink struct {
	// Kind is the zoom type: slide, section, or summary.
	Kind ZoomKind
	// SourceSlideIndex is the 0-based index of the slide that hosts the zoom.
	SourceSlideIndex int
	// ShapeID is the cNvPr id of the zoom's graphic frame.
	ShapeID uint32
	// ShapeName is the cNvPr name of the zoom's graphic frame.
	ShapeName string
	// TargetSlideID is the sldId a Slide Zoom links to — the numeric id of a
	// p:sldId entry in the presentation's sldIdLst. It is zero for section and
	// summary zooms.
	TargetSlideID uint32
	// TargetSectionIDs are the section GUIDs the zoom links to: one for a
	// Section Zoom, one per zoomed section for a Summary Zoom, and none for a
	// Slide Zoom.
	TargetSectionIDs []string
}

// ZoomLinks reports the Zoom objects (Slide, Section, and Summary zooms) on the
// slide, in shape-tree order. It reads through the normal slide accessor and
// never marks the slide modified, so an unmodified deck that carries zooms still
// round-trips byte-for-byte after calling it; the zoom frames are preserved
// verbatim. Creating a zoom is not supported: a zoom embeds a rendered
// thumbnail image of its target that this library cannot generate.
func (s *Slide) ZoomLinks() []ZoomLink {
	m := s.sx()
	if m == nil || m.CSld == nil || m.CSld.SpTree == nil {
		return nil
	}
	var links []ZoomLink
	for _, gf := range m.CSld.SpTree.GraphicFrame {
		if gf == nil || gf.Graphic == nil || gf.Graphic.GraphicData == nil {
			continue
		}
		kind, ok := zoomKindForURI(gf.Graphic.GraphicData.URI)
		if !ok {
			continue
		}
		link := ZoomLink{Kind: kind, SourceSlideIndex: s.index}
		if gf.NvGraphicFramePr != nil && gf.NvGraphicFramePr.CNvPr != nil {
			link.ShapeID = gf.NvGraphicFramePr.CNvPr.Id
			link.ShapeName = gf.NvGraphicFramePr.CNvPr.Name
		}
		link.TargetSlideID, link.TargetSectionIDs = parseZoomTargets(gf.Graphic.GraphicData.RawContent)
		links = append(links, link)
	}
	return links
}

// ZoomLinks reports every Zoom object across all of the deck's slides, in slide
// order then shape-tree order. Each link carries the index of the slide that
// hosts it (ZoomLink.SourceSlideIndex).
func (p *Presentation) ZoomLinks() []ZoomLink {
	var links []ZoomLink
	for _, s := range p.slides {
		links = append(links, s.ZoomLinks()...)
	}
	return links
}

// zoomKindForURI maps a graphicData URI to its zoom kind, reporting false when
// the URI is not a zoom URI.
func zoomKindForURI(uri string) (ZoomKind, bool) {
	switch uri {
	case zoomSlideURI:
		return ZoomKindSlide, true
	case zoomSectionURI:
		return ZoomKindSection, true
	case zoomSummaryURI:
		return ZoomKindSummary, true
	default:
		return "", false
	}
}

// parseZoomTargets extracts the target slide id (Slide Zoom) and section GUIDs
// (Section and Summary zooms) from the raw inner XML of a zoom graphicData.
// Elements and attributes are matched by local name so the result does not
// depend on which namespace prefix the producer used.
func parseZoomTargets(raw []byte) (uint32, []string) {
	if len(raw) == 0 {
		return 0, nil
	}
	dec := xml.NewDecoder(bytes.NewReader(raw))
	var slideID uint32
	var sectionIDs []string
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch se.Name.Local {
		case "sldZmObj":
			if v := zoomAttr(se, "sldId"); v != "" {
				if n, err := strconv.ParseUint(v, 10, 32); err == nil {
					slideID = uint32(n)
				}
			}
		case "sectionZmObj", "summaryZmObj":
			if v := zoomAttr(se, "sectionId"); v != "" {
				sectionIDs = append(sectionIDs, v)
			}
		}
	}
	return slideID, sectionIDs
}

// zoomAttr returns the value of the attribute with the given local name on se,
// or "" when absent.
func zoomAttr(se xml.StartElement, local string) string {
	for _, a := range se.Attr {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}
