package docx

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/mgilbir/spine/common/validate"
	"github.com/mgilbir/spine/docx/internal/oxml"
	"github.com/mgilbir/spine/opc"
)

// Validation codes specific to WordprocessingML (see common/validate for the
// shared OPC-level codes).
const (
	codeNumberingMissing = "numbering-missing" // numPr references a numId absent from numbering.xml
	codeStyleMissing     = "style-missing"     // pStyle/rStyle references an undefined style
	codeCommentMissing   = "comment-missing"   // comment marker references an absent comment
)

// Validate walks the in-memory document model and reports structural problems
// without saving or re-parsing. Save and SaveTo run it first and refuse to
// write when any error-severity finding is present; use SaveToUnvalidated to
// bypass the gate.
//
// The checks are sound (no false positives on Word-accepted packages).
func (d *Document) Validate() validate.Report {
	c := validate.New()
	d.validateNumbering(c)
	d.validateHeaderFooterRefs(c)
	d.validateEmbedRefs(c)
	d.validateStyleRefs(c)
	d.validateCommentRefs(c)
	if d.reader != nil {
		d.validatePackage(c)
	}
	return c.Report()
}

// numIDSet returns the set of w:num numId values defined in numbering.xml
// (parsed definitions plus any added this session). The second result reports
// whether a numbering part is present at all.
func (d *Document) numIDSet() (map[string]bool, bool) {
	if d.numbering == nil {
		return nil, false
	}
	set := make(map[string]bool)
	for _, id := range d.numbering.ParsedNumIDs {
		set[id] = true
	}
	for _, n := range d.numbering.Num {
		if n != nil {
			set[n.NumId] = true
		}
	}
	return set, true
}

// validateNumbering reports paragraphs whose numPr references a numId with no
// matching w:num in numbering.xml (the C26 class). A zero numId means "no
// numbering" and is skipped.
//
// The finding is warning severity, not error: Word tolerates a dangling numPr
// (it renders the paragraph without list formatting), and real Office-accepted
// documents in the wild reference a numId while shipping an empty or partial
// numbering part. Blocking a save on that would reject a file Word accepts.
func (d *Document) validateNumbering(c *validate.Collector) {
	ids, present := d.numIDSet()
	for _, para := range d.allParagraphs() {
		if para.PPr == nil || para.PPr.NumPr == nil || para.PPr.NumPr.NumId == nil {
			continue
		}
		val := para.PPr.NumPr.NumId.Val
		if val == 0 {
			continue
		}
		if present && ids[strconv.Itoa(val)] {
			continue
		}
		detail := fmt.Sprintf("paragraph numPr references numId %d with no matching w:num in numbering.xml", val)
		if !present {
			detail = fmt.Sprintf("paragraph numPr references numId %d but the document has no numbering part", val)
		}
		c.Warnf(codeNumberingMissing, d.mainPart(), detail)
	}
}

// validateHeaderFooterRefs reports section header/footer references whose r:id
// has no matching relationship on the document part (dangling-rel).
func (d *Document) validateHeaderFooterRefs(c *validate.Collector) {
	if d.document == nil || d.document.Body == nil {
		return
	}
	rels := relIDSetBool(d.relationships[d.mainPart()])
	check := func(sect *oxml.CT_SectPr) {
		if sect == nil {
			return
		}
		for _, ref := range sect.HeaderReference {
			if ref != nil && ref.RID != "" && !rels[ref.RID] {
				c.Errorf(validate.CodeDanglingRel, d.mainPart(),
					fmt.Sprintf("headerReference references relationship %q with no matching relationship", ref.RID))
			}
		}
		for _, ref := range sect.FooterReference {
			if ref != nil && ref.RID != "" && !rels[ref.RID] {
				c.Errorf(validate.CodeDanglingRel, d.mainPart(),
					fmt.Sprintf("footerReference references relationship %q with no matching relationship", ref.RID))
			}
		}
	}
	check(d.document.Body.SectPr)
	for _, p := range d.document.Body.P {
		if p != nil && p.PPr != nil {
			check(p.PPr.SectPr)
		}
	}
}

// validateEmbedRefs reports drawing image references (a:blip r:embed / r:link)
// and hyperlink r:id references whose relationship is absent from the owning
// part — the C25/C180 dangling-image class. Each part's references are resolved
// against that part's own relationships. Findings are warning severity: Word
// tolerates a dangling image/hyperlink reference (a broken-image placeholder or
// a non-link), so it must not block a save.
func (d *Document) validateEmbedRefs(c *validate.Collector) {
	if d.document != nil && d.document.Body != nil {
		rels := relIDSetBool(d.relationships[d.mainPart()])
		for _, p := range d.document.Body.P {
			d.checkParagraphEmbeds(c, p, d.mainPart(), rels)
		}
	}
	for name, hp := range d.headers {
		if hp != nil && hp.hdr != nil {
			rels := relIDSetBool(d.relationships[name])
			for _, p := range hp.hdr.P {
				d.checkParagraphEmbeds(c, p, name, rels)
			}
		}
	}
	for name, fp := range d.footers {
		if fp != nil && fp.ftr != nil {
			rels := relIDSetBool(d.relationships[name])
			for _, p := range fp.ftr.P {
				d.checkParagraphEmbeds(c, p, name, rels)
			}
		}
	}
}

// checkParagraphEmbeds checks the drawing embeds and hyperlink r:ids in one
// paragraph (and its directly nested run containers) against part's rels.
func (d *Document) checkParagraphEmbeds(c *validate.Collector, p *oxml.CT_P, part string, rels map[string]bool) {
	if p == nil {
		return
	}
	for _, r := range collectParagraphRuns(p) {
		for _, dr := range r.Drawing {
			if dr == nil {
				continue
			}
			for _, id := range scanEmbedIDs(dr.RawContent) {
				if id != "" && !rels[id] {
					c.Warnf(validate.CodeDanglingRel, part,
						fmt.Sprintf("drawing references image relationship %q with no matching relationship", id))
				}
			}
		}
	}
	for _, h := range p.Hyperlink {
		if h != nil && h.RID != "" && !rels[h.RID] {
			c.Warnf(validate.CodeDanglingRel, part,
				fmt.Sprintf("hyperlink references relationship %q with no matching relationship", h.RID))
		}
	}
}

// collectParagraphRuns returns the runs directly in a paragraph and in its
// hyperlink / tracked-change children. Runs nested more deeply (e.g. inside
// SDTs) are not reached; missing one only loses coverage, never soundness.
func collectParagraphRuns(p *oxml.CT_P) []*oxml.CT_R {
	if p == nil {
		return nil
	}
	runs := append([]*oxml.CT_R(nil), p.R...)
	for _, h := range p.Hyperlink {
		if h != nil {
			runs = append(runs, h.R...)
		}
	}
	for _, ins := range p.Ins {
		if ins != nil {
			runs = append(runs, ins.R...)
		}
	}
	for _, del := range p.Del {
		if del != nil {
			runs = append(runs, del.R...)
		}
	}
	return runs
}

// scanEmbedIDs extracts the relationship ids from r:embed / r:link attributes in
// raw drawing bytes (a:blip and asvg:svgBlip both use r:embed).
func scanEmbedIDs(raw []byte) []string {
	var ids []string
	for _, marker := range [][]byte{[]byte(`r:embed="`), []byte(`r:link="`)} {
		s := raw
		for {
			i := bytes.Index(s, marker)
			if i < 0 {
				break
			}
			s = s[i+len(marker):]
			j := bytes.IndexByte(s, '"')
			if j < 0 {
				break
			}
			ids = append(ids, string(s[:j]))
			s = s[j+1:]
		}
	}
	return ids
}

// validateStyleRefs reports paragraph/run style references to styleIds that are
// not defined in styles.xml. It is warning severity (Word tolerates dangling
// style references, resolving them to defaults, and latent/built-in styles are
// not always materialized in the styles part).
func (d *Document) validateStyleRefs(c *validate.Collector) {
	styleIDs := d.definedStyleIDs()
	for _, para := range d.allParagraphs() {
		if para.PPr != nil && para.PPr.PStyle != nil && para.PPr.PStyle.Val != "" {
			if _, ok := styleIDs[para.PPr.PStyle.Val]; !ok {
				c.Warnf(codeStyleMissing, d.mainPart(),
					fmt.Sprintf("paragraph references style %q not defined in styles.xml", para.PPr.PStyle.Val))
			}
		}
		for _, r := range collectParagraphRuns(para) {
			if r.RPr != nil && r.RPr.RStyle != nil && r.RPr.RStyle.Val != "" {
				if _, ok := styleIDs[r.RPr.RStyle.Val]; !ok {
					c.Warnf(codeStyleMissing, d.mainPart(),
						fmt.Sprintf("run references style %q not defined in styles.xml", r.RPr.RStyle.Val))
				}
			}
		}
	}
}

// definedStyleIDs returns the set of styleIds defined in styles.xml. When there
// is no styles part it returns nil, and validateStyleRefs' lookups all miss —
// but since the finding is a warning, that only produces advisory output.
func (d *Document) definedStyleIDs() map[string]struct{} {
	set := make(map[string]struct{})
	if d.styles == nil {
		return set
	}
	for _, s := range d.styles.Style {
		if s != nil && s.StyleId != "" {
			set[s.StyleId] = struct{}{}
		}
	}
	return set
}

// allParagraphs returns the underlying CT_P values reachable from the body,
// including SDT-nested ones (via the public walk), plus header/footer
// paragraphs.
func (d *Document) allParagraphs() []*oxml.CT_P {
	var out []*oxml.CT_P
	if d.document != nil && d.document.Body != nil {
		out = append(out, d.document.Body.Paragraphs()...)
	}
	for _, hp := range d.headers {
		if hp != nil && hp.hdr != nil {
			out = append(out, hp.hdr.P...)
		}
	}
	for _, fp := range d.footers {
		if fp != nil && fp.ftr != nil {
			out = append(out, fp.ftr.P...)
		}
	}
	return out
}

// validateCommentRefs reports document comment markers (commentRangeStart and
// the run-level commentReference) whose w:id has no matching w:comment in
// comments.xml — the dangling-comment class. Findings are warning severity:
// Word tolerates an orphaned marker (it renders no comment), so it must not
// block a save.
func (d *Document) validateCommentRefs(c *validate.Collector) {
	ids := make(map[string]bool)
	if d.comments != nil {
		for _, cm := range d.comments.Comment {
			if cm != nil {
				ids[cm.Id] = true
			}
		}
	}
	for _, para := range d.allParagraphs() {
		if para == nil {
			continue
		}
		for _, rs := range para.CommentRangeStart {
			if rs != nil && rs.Id != "" && !ids[rs.Id] {
				c.Warnf(codeCommentMissing, d.mainPart(),
					fmt.Sprintf("commentRangeStart references comment id %q with no matching comment in comments.xml", rs.Id))
			}
		}
		for _, r := range collectParagraphRuns(para) {
			for _, ref := range r.CommentReference {
				if ref != nil && ref.Id != "" && !ids[ref.Id] {
					c.Warnf(codeCommentMissing, d.mainPart(),
						fmt.Sprintf("commentReference references comment id %q with no matching comment in comments.xml", ref.Id))
				}
			}
		}
	}
}

// validatePackage runs the shared OPC-level checks against the source package.
func (d *Document) validatePackage(c *validate.Collector) {
	parts := d.knownPartNames()
	ct := func(name string) string {
		if d.reader != nil && d.reader.ContentTypes != nil {
			return d.reader.ContentTypes.GetContentType(name)
		}
		return ""
	}
	validate.CheckDuplicateParts(c, parts)
	validate.CheckContentTypes(c, parts, ct)
	validate.CheckRelationshipTargets(c, d.relationships, d.partExists)
}

func (d *Document) knownPartNames() []string {
	seen := make(map[string]bool)
	var out []string
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	if d.reader != nil {
		for _, f := range d.reader.Files {
			add(f.Name)
		}
	}
	for name := range d.preservedParts {
		add(name)
	}
	for name := range d.headers {
		add(name)
	}
	for name := range d.footers {
		add(name)
	}
	return out
}

// partExists reports whether a part name is present in the source package or a
// model collection. Deliberately over-inclusive so relationship-target checks
// never yield a false positive.
func (d *Document) partExists(name string) bool {
	if d.reader != nil && d.reader.GetFile(name) != nil {
		return true
	}
	if _, ok := d.preservedParts[name]; ok {
		return true
	}
	if _, ok := d.otherParts[name]; ok {
		return true
	}
	if _, ok := d.headers[name]; ok {
		return true
	}
	if _, ok := d.footers[name]; ok {
		return true
	}
	switch name {
	case d.mainPart(), "/docProps/core.xml", "/docProps/app.xml":
		return true
	}
	return false
}

// relIDSetBool returns the set of relationship ids in rels.
func relIDSetBool(rels []*opc.Relationship) map[string]bool {
	set := make(map[string]bool, len(rels))
	for _, r := range rels {
		if r != nil {
			set[r.ID] = true
		}
	}
	return set
}
