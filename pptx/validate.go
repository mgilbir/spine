package pptx

import (
	"encoding/xml"
	"fmt"

	"github.com/mgilbir/spine/common/validate"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

const presMainPartName = "/ppt/presentation.xml"

// Validation codes specific to PresentationML (see common/validate for the
// shared OPC-level codes).
const (
	codeShapeIDDup      = "shape-id-dup"      // duplicate cNvPr id within one slide
	codeCommentNoAuthor = "comment-no-author" // comment authorId with no matching author
	codeHyperlinkNoRel  = "hyperlink-no-rel"  // hlinkClick r:id with no matching slide rel
	codeChartNoRel      = "chart-no-rel"      // chart graphicFrame r:id with no target part
)

// Validate walks the in-memory presentation model and reports structural
// problems without saving or re-parsing. Save and SaveTo run it first and
// refuse to write when any error-severity finding is present; use
// SaveToUnvalidated to bypass the gate.
//
// The checks are sound (no false positives on Office-accepted packages): each
// mirrors what the save path actually writes and errs toward silence when a
// fact cannot be established from the model alone.
func (p *Presentation) Validate() validate.Report {
	c := validate.New()
	p.validateShapeIDs(c)
	p.validateIDListReferences(c)
	p.validateCommentAuthors(c)
	p.validateHyperlinks(c)
	p.validateCharts(c)
	if p.reader != nil {
		// Package-level checks compare against the parts the source package
		// carries. For a freshly created deck the writer synthesizes content
		// types and relationships by construction, so there is nothing to
		// cross-check yet.
		p.validatePackage(c)
	}
	return c.Report()
}

// validateShapeIDs reports duplicate drawing-element ids (cNvPr id) within a
// single slide's shape tree. ST_DrawingElementId must be unique per slide;
// duplicates make PowerPoint drop or merge shapes.
func (p *Presentation) validateShapeIDs(c *validate.Collector) {
	for _, slide := range p.slides {
		// Read sxModel directly (never sx()): a slide that was never accessed is
		// unmodified and already passed the parse-then-discard validation at
		// open, so it cannot have acquired a duplicate id — skip it rather than
		// force a parse of every slide just to validate.
		if slide == nil || slide.sxModel == nil || slide.sxModel.CSld == nil {
			continue
		}
		seen := make(map[uint32]bool)
		reported := make(map[uint32]bool)
		for _, id := range collectTreeShapeIDs(slide.sxModel.CSld.SpTree) {
			if id == 0 {
				continue
			}
			if seen[id] {
				if !reported[id] {
					c.Errorf(codeShapeIDDup, slide.partName,
						fmt.Sprintf("drawing element id %d appears more than once on the slide", id))
					reported[id] = true
				}
				continue
			}
			seen[id] = true
		}
	}
}

// collectTreeShapeIDs returns every cNvPr id in a shape tree, recursing into
// groups, in document order.
func collectTreeShapeIDs(st *oxml.ShapeTree) []uint32 {
	if st == nil {
		return nil
	}
	var ids []uint32
	for _, sp := range st.Sp {
		if sp != nil && sp.NvSpPr != nil && sp.NvSpPr.CNvPr != nil {
			ids = append(ids, sp.NvSpPr.CNvPr.Id)
		}
	}
	for _, pic := range st.Pic {
		if pic != nil && pic.NvPicPr != nil && pic.NvPicPr.CNvPr != nil {
			ids = append(ids, pic.NvPicPr.CNvPr.Id)
		}
	}
	for _, gf := range st.GraphicFrame {
		if gf != nil && gf.NvGraphicFramePr != nil && gf.NvGraphicFramePr.CNvPr != nil {
			ids = append(ids, gf.NvGraphicFramePr.CNvPr.Id)
		}
	}
	for _, cxn := range st.CxnSp {
		if cxn != nil && cxn.NvCxnSpPr != nil && cxn.NvCxnSpPr.CNvPr != nil {
			ids = append(ids, cxn.NvCxnSpPr.CNvPr.Id)
		}
	}
	for _, g := range st.GrpSp {
		ids = append(ids, collectGroupShapeIDs(g)...)
	}
	return ids
}

func collectGroupShapeIDs(g *oxml.GroupShape) []uint32 {
	if g == nil {
		return nil
	}
	var ids []uint32
	if g.NvGrpSpPr != nil && g.NvGrpSpPr.CNvPr != nil {
		ids = append(ids, g.NvGrpSpPr.CNvPr.Id)
	}
	for _, sp := range g.Shapes {
		if sp != nil && sp.NvSpPr != nil && sp.NvSpPr.CNvPr != nil {
			ids = append(ids, sp.NvSpPr.CNvPr.Id)
		}
	}
	for _, pic := range g.Pictures {
		if pic != nil && pic.NvPicPr != nil && pic.NvPicPr.CNvPr != nil {
			ids = append(ids, pic.NvPicPr.CNvPr.Id)
		}
	}
	for _, gf := range g.GraphicFrames {
		if gf != nil && gf.NvGraphicFramePr != nil && gf.NvGraphicFramePr.CNvPr != nil {
			ids = append(ids, gf.NvGraphicFramePr.CNvPr.Id)
		}
	}
	for _, cxn := range g.ConnectionShapes {
		if cxn != nil && cxn.NvCxnSpPr != nil && cxn.NvCxnSpPr.CNvPr != nil {
			ids = append(ids, cxn.NvCxnSpPr.CNvPr.Id)
		}
	}
	for _, sub := range g.GroupShapes {
		ids = append(ids, collectGroupShapeIDs(sub)...)
	}
	return ids
}

// validateIDListReferences reports sldLayoutId entries whose r:id has no
// backing relationship — the dangling-relationship class that caused the worst
// corruption.
//
// The sldIdLst and sldMasterIdLst are intentionally NOT checked here: the save
// path rebuilds both from p.slides / p.slideMasters (r:id := each item's relID)
// and synthesizes a matching relationship for each, so they are consistent by
// construction and the in-memory p.presentation.SlideIDs is stale between a
// mutation and the next save. A master's sldLayoutIdLst, by contrast, is
// preserved verbatim on an unmodified master, so a dangling layout reference in
// the source (or one left by a mutation that dropped a rel) is a real defect
// that survives the save.
func (p *Presentation) validateIDListReferences(c *validate.Collector) {
	for _, m := range p.slideMasters {
		if m == nil || m.masterXML == nil || m.masterXML.SlideLayoutIDs == nil {
			continue
		}
		// When the layout set was modified the save rebuilds the id list from
		// m.layouts (r:id := layout.relID) and registers a rel for each, so the
		// list is consistent by construction. Only the preserved (unmodified)
		// list can carry a stale reference.
		if m.layoutsModified {
			continue
		}
		// The written relationship set is the master's preserved rels plus one
		// per current layout (the save synthesizes a rel for each layout.relID,
		// and created decks hold their layout rels only in m.layouts, never in
		// p.relationships).
		masterIDs := relIDSet(p.relationships[m.partName])
		for _, l := range m.layouts {
			if l != nil {
				masterIDs[l.relID] = true
			}
		}
		for _, lid := range m.masterXML.SlideLayoutIDs.SlideLayoutID {
			if lid.RID != "" && !masterIDs[lid.RID] {
				c.Errorf(validate.CodeDanglingRel, m.partName,
					fmt.Sprintf("sldLayoutId id=%d references relationship %q with no matching relationship", lid.ID, lid.RID))
			}
		}
	}
}

// validateCommentAuthors warns when a comment references an author id that has
// no matching entry in the author list. A missing author leaves the comment
// showing as authored by "Unknown" in PowerPoint; it is a warning, not an
// error, since the file still opens.
func (p *Presentation) validateCommentAuthors(c *validate.Collector) {
	legacy := p.legacyAuthorNames()
	modern := p.modernAuthorNames()
	for _, s := range p.slides {
		if s == nil {
			continue
		}
		for _, rel := range p.relationships[s.partName] {
			if rel == nil || rel.TargetMode == opc.TargetModeExternal {
				continue
			}
			target := opc.ResolvePartName(s.partName, rel.Target)
			switch rel.Type {
			case opc.RelTypeComments:
				var lst oxml.CommentList
				if xmlUnmarshal(p.rawPartData(target), &lst) {
					for _, cm := range lst.Cm {
						if cm == nil {
							continue
						}
						if _, ok := legacy[cm.AuthorId]; !ok {
							c.Warnf(codeCommentNoAuthor, target,
								fmt.Sprintf("comment idx=%d references author id %d with no matching commentAuthor", cm.Idx, cm.AuthorId))
						}
					}
				}
			case opc.RelTypeModernComments:
				part, err := oxml.ParseModernCommentPart(p.rawPartData(target))
				if err != nil || part.Comment == nil {
					continue
				}
				ids := []string{part.Comment.AuthorID}
				for _, r := range part.Comment.Replies {
					ids = append(ids, r.AuthorID)
				}
				for _, id := range ids {
					if _, ok := modern[id]; !ok {
						c.Warnf(codeCommentNoAuthor, target,
							fmt.Sprintf("comment references author %q with no matching author in authors.xml", id))
					}
				}
			}
		}
	}
}

// validateHyperlinks warns when a materialized hyperlink references a slide
// relationship id that has no matching relationship — the dangling-r:id class
// that leaves the link dead in PowerPoint. Hyperlinks created via the API but
// not yet saved carry no relID and are skipped (their rel is allocated on save).
func (p *Presentation) validateHyperlinks(c *validate.Collector) {
	for _, s := range p.slides {
		// Read sxModel directly (never sx()/Hyperlinks() on an unparsed slide):
		// a slide that was never accessed is unmodified, so it cannot have
		// acquired a hyperlink whose relationship went missing — skip it rather
		// than force a parse of every slide just to validate.
		if s == nil || s.sxModel == nil {
			continue
		}
		ids := relIDSet(p.relationships[s.partName])
		for _, h := range s.Hyperlinks() {
			if h.relID != "" && !ids[h.relID] {
				c.Warnf(codeHyperlinkNoRel, s.partName,
					fmt.Sprintf("hyperlink references relationship %q with no matching relationship", h.relID))
			}
		}
	}
}

// validateCharts warns when a slide graphic frame declares a chart
// (a:graphicData uri=.../chart) whose c:chart r:id has no backing slide
// relationship, or whose relationship targets a part the package does not
// carry. Such a frame renders empty in PowerPoint. The check is cheap: it walks
// the parsed shape tree already in memory.
func (p *Presentation) validateCharts(c *validate.Collector) {
	for _, s := range p.slides {
		// Read sxModel directly (never sx()): a never-accessed slide is
		// unmodified, so it carries no newly broken chart reference to warn
		// about — skip it rather than force a parse of every slide.
		if s == nil || s.sxModel == nil || s.sxModel.CSld == nil || s.sxModel.CSld.SpTree == nil {
			continue
		}
		for _, gf := range s.sxModel.CSld.SpTree.GraphicFrame {
			relID := chartRelIDOf(gf)
			if relID == "" {
				continue
			}
			if !p.chartRelHasTarget(s, relID) {
				c.Warnf(codeChartNoRel, s.partName,
					fmt.Sprintf("chart graphic frame references relationship %q with no target part", relID))
			}
		}
	}
}

// xmlUnmarshal reports whether data decodes into v without error (nil data
// yields false).
func xmlUnmarshal(data []byte, v any) bool {
	if data == nil {
		return false
	}
	return xml.Unmarshal(data, v) == nil
}

// validatePackage runs the shared OPC-level checks against the source package's
// parts, unioned with parts this session added.
func (p *Presentation) validatePackage(c *validate.Collector) {
	parts := p.knownPartNames()
	ct := func(name string) string {
		if p.reader != nil && p.reader.ContentTypes != nil {
			return p.reader.ContentTypes.GetContentType(name)
		}
		return ""
	}
	exists := func(name string) bool { return p.partExists(name) }

	validate.CheckDuplicateParts(c, parts)
	validate.CheckContentTypes(c, parts, ct)
	validate.CheckRelationshipTargets(c, p.relationships, exists)
}

// knownPartNames returns the part names present in the source package plus the
// parts this session added.
func (p *Presentation) knownPartNames() []string {
	seen := make(map[string]bool)
	var out []string
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	if p.reader != nil {
		for _, f := range p.reader.Files {
			add(f.Name)
		}
	}
	for _, s := range p.slides {
		if s != nil {
			add(s.partName)
		}
	}
	for _, m := range p.slideMasters {
		if m != nil {
			add(m.partName)
		}
	}
	for _, l := range p.slideLayouts {
		if l != nil {
			add(l.partName)
		}
	}
	for name := range p.otherParts {
		add(name)
	}
	for name := range p.themeData {
		add(name)
	}
	return out
}

// partExists reports whether a part name is present in the source package or in
// a model collection. It is deliberately over-inclusive (a removed part still
// counted as present) so relationship-target checks never yield a false
// positive.
func (p *Presentation) partExists(name string) bool {
	if p.reader != nil && p.reader.GetFile(name) != nil {
		return true
	}
	for _, s := range p.slides {
		if s != nil && s.partName == name {
			return true
		}
	}
	for _, m := range p.slideMasters {
		if m != nil && m.partName == name {
			return true
		}
	}
	for _, l := range p.slideLayouts {
		if l != nil && l.partName == name {
			return true
		}
	}
	if _, ok := p.otherParts[name]; ok {
		return true
	}
	if _, ok := p.themeData[name]; ok {
		return true
	}
	if _, ok := p.printerSettings[name]; ok {
		return true
	}
	switch name {
	case presMainPartName, "/ppt/presProps.xml", "/ppt/viewProps.xml",
		"/ppt/tableStyles.xml", "/docProps/core.xml", "/docProps/app.xml",
		"/docProps/thumbnail.jpeg":
		return true
	}
	return false
}

// relIDSet returns the set of relationship ids in rels.
func relIDSet(rels []*opc.Relationship) map[string]bool {
	set := make(map[string]bool, len(rels))
	for _, r := range rels {
		if r != nil {
			set[r.ID] = true
		}
	}
	return set
}
