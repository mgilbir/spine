package docx

import (
	"regexp"

	"github.com/mgilbir/spine/docx/internal/oxml"
	"github.com/mgilbir/spine/opc"
)

// Relationship reclamation for replace-style edits.
//
// Paragraph.SetText, Paragraph.Clear and Run.Clear all delete content; the
// deleted content may have been the only thing referencing a part-scoped
// relationship (a hyperlink's External rel, an image's r:embed). Deleting the
// node cleaned the node and left the inbound edge, so a template-fill workflow
// that calls SetText on every paragraph accreted one dead hyperlink
// relationship per link and, for images added in the same session, a dead media
// part too (C407). The output stayed valid — unreferenced relationships are
// legal — but the package grew without bound and there is no removal API to
// reclaim any of it.
//
// The sweep is deliberately narrow. It only ever considers relationship ids the
// removed content itself referenced, and it drops one only after re-checking
// that nothing else in the same part still references it. Relationship ids are
// unique per part, so an id carried by removed body content can never also be
// the styles or numbering relationship. Preserved package parts are never
// deleted (the same policy dropSessionHeader documents); only media added in
// this session, and only when no relationship anywhere still points at it.

// relRefAttrRe matches the relationship-id-bearing attributes that appear in
// raw-preserved markup (r:id, r:embed, r:link, r:href), under any prefix — a
// producer is free to bind the relationships namespace to something other than
// "r".
var relRefAttrRe = regexp.MustCompile(`\b[A-Za-z_][\w.-]*:(?:id|embed|link|href|pict|dm|lo|qs|cs)="(rId\d+)"`)

// addRawRelRefs adds every relationship id referenced by raw markup to set.
func addRawRelRefs(set map[string]bool, raw []byte) {
	for _, m := range relRefAttrRe.FindAllSubmatch(raw, -1) {
		set[string(m[1])] = true
	}
}

// addRunRelRefs adds every relationship id a run references — through a
// drawing, a legacy VML w:pict, an OLE w:object, an mc:AlternateContent, or any
// raw-preserved child.
func addRunRelRefs(set map[string]bool, r *oxml.CT_R) {
	if r == nil {
		return
	}
	for _, dr := range r.Drawing {
		if dr != nil {
			addRawRelRefs(set, dr.RawContent)
		}
	}
	for _, e := range r.Pict {
		if e != nil {
			addRawRelRefs(set, e.RawContent)
		}
	}
	for _, e := range r.Object {
		if e != nil {
			addRawRelRefs(set, e.RawContent)
		}
	}
	for _, e := range r.AlternateContent {
		if e != nil {
			addRawRelRefs(set, e.RawContent)
		}
	}
	for _, e := range r.Raw {
		if e != nil {
			addRawRelRefs(set, e.RawContent)
		}
	}
}

// addParagraphRelRefs adds every relationship id the paragraph references: its
// hyperlinks' r:id and the references of every run it contains, including runs
// nested in hyperlinks and tracked-change wrappers.
func addParagraphRelRefs(set map[string]bool, p *oxml.CT_P) {
	if p == nil {
		return
	}
	for _, h := range p.Hyperlink {
		if h != nil && h.RID != "" {
			set[h.RID] = true
		}
	}
	for _, r := range oxmlParagraphRuns(p) {
		addRunRelRefs(set, r)
	}
	for _, e := range p.Raw {
		if e != nil {
			addRawRelRefs(set, e.RawContent)
		}
	}
	if p.PPr != nil && p.PPr.SectPr != nil {
		addSectPrRelRefs(set, p.PPr.SectPr)
	}
}

// addSectPrRelRefs adds the relationship ids a section's properties reference:
// its header and footer references and its raw-preserved printerSettings.
func addSectPrRelRefs(set map[string]bool, sectPr *oxml.CT_SectPr) {
	if sectPr == nil {
		return
	}
	for _, ref := range sectPr.HeaderReference {
		if ref != nil && ref.RID != "" {
			set[ref.RID] = true
		}
	}
	for _, ref := range sectPr.FooterReference {
		if ref != nil && ref.RID != "" {
			set[ref.RID] = true
		}
	}
	if sectPr.PrinterSettings != nil {
		addRawRelRefs(set, sectPr.PrinterSettings.RawContent)
	}
}

// partRelRefs returns every relationship id still referenced from the content of
// the given part: the whole body (including its section properties) for the main
// document part, or a header/footer part's own content.
func (d *Document) partRelRefs(part string) map[string]bool {
	set := make(map[string]bool)
	if part == d.mainPart() {
		if d.doc() != nil && d.doc().Body != nil {
			for _, p := range d.doc().Body.AllParagraphs() {
				addParagraphRelRefs(set, p)
			}
			addSectPrRelRefs(set, d.doc().Body.SectPr)
		}
		return set
	}
	if hp, ok := d.headers[part]; ok && hp != nil && hp.hdr != nil {
		for _, p := range hp.hdr.AllParagraphs() {
			addParagraphRelRefs(set, p)
		}
	}
	if fp, ok := d.footers[part]; ok && fp != nil && fp.ftr != nil {
		for _, p := range fp.ftr.AllParagraphs() {
			addParagraphRelRefs(set, p)
		}
	}
	return set
}

// sweepRemovedRelRefs drops the relationships of the paragraph's owning part
// that were referenced by content just removed from it and by nothing else. It
// is a no-op when the removed content referenced no relationship, so the common
// case (SetText on a plain paragraph) does no work at all and never walks the
// part.
func (p *Paragraph) sweepRemovedRelRefs(removed map[string]bool) {
	if len(removed) == 0 || p == nil || p.document == nil {
		return
	}
	owner := p.ownerPartName()
	still := p.document.partRelRefs(owner)
	var drop []string
	for id := range removed {
		if !still[id] {
			drop = append(drop, id)
		}
	}
	p.document.dropPartRelationships(owner, drop)
}

// dropPartRelationships removes the named relationships from a part's set and
// reclaims any media part added in this session that no relationship targets any
// more. Parts that came from the opened package are left in place: they are
// preserved bytes, and the round-trip contract is that preserved parts are
// written back.
func (d *Document) dropPartRelationships(part string, ids []string) {
	if len(ids) == 0 {
		return
	}
	drop := make(map[string]bool, len(ids))
	for _, id := range ids {
		drop[id] = true
	}
	var orphaned []string // resolved part names the dropped rels targeted
	kept := d.relationships[part][:0]
	for _, rel := range d.relationships[part] {
		if rel == nil {
			continue
		}
		if !drop[rel.ID] {
			kept = append(kept, rel)
			continue
		}
		if rel.TargetMode != opc.TargetModeExternal {
			orphaned = append(orphaned, opc.ResolvePartName(part, rel.Target))
		}
	}
	d.relationships[part] = kept
	d.dropUnreferencedImageParts(orphaned)
}

// dropUnreferencedImageParts removes session-added media parts among names that
// no relationship in any part still targets.
func (d *Document) dropUnreferencedImageParts(names []string) {
	if len(names) == 0 || len(d.imageParts) == 0 {
		return
	}
	candidate := make(map[string]bool, len(names))
	for _, n := range names {
		candidate[n] = true
	}
	for src, rels := range d.relationships {
		for _, rel := range rels {
			if rel == nil || rel.TargetMode == opc.TargetModeExternal {
				continue
			}
			delete(candidate, opc.ResolvePartName(src, rel.Target))
		}
	}
	if len(candidate) == 0 {
		return
	}
	kept := d.imageParts[:0]
	for _, ip := range d.imageParts {
		if ip != nil && candidate[ip.partName] {
			continue
		}
		kept = append(kept, ip)
	}
	d.imageParts = kept
}
