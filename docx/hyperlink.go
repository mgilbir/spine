package docx

import (
	"fmt"

	"github.com/mgilbir/spine/docx/internal/oxml"
	"github.com/mgilbir/spine/opc"
)

// hyperlinkStyleID is the character style Word applies to hyperlink display
// text.
const hyperlinkStyleID = "Hyperlink"

// Hyperlink is a hyperlink in a document. Its accessors are shared verbatim
// with the xlsx and pptx hyperlink APIs so the three formats are symmetric:
// URL for an external target, Anchor for an internal one (a bookmark name in
// docx), and Tooltip for the screen-tip. Only the anchoring differs by format.
type Hyperlink struct {
	document *Document
	h        *oxml.CT_Hyperlink
	// part is the source part whose relationships resolve the hyperlink's r:id
	// (the main document part, or a header/footer part).
	part string
	// para is the paragraph the hyperlink lives in, used to build Run handles
	// that resolve back to this hyperlink via Run.Hyperlink().
	para *Paragraph
}

// Runs returns the runs wrapped by this hyperlink (docx-specific). Each run's
// Hyperlink() resolves back to this hyperlink, so the display formatting inside
// a link is reachable — Paragraph.Runs() returns only top-level runs.
func (h *Hyperlink) Runs() []*Run {
	out := make([]*Run, 0, len(h.h.R))
	for _, hr := range h.h.R {
		out = append(out, &Run{paragraph: h.para, r: hr})
	}
	return out
}

// URL returns the hyperlink's external target, or "" when the hyperlink is
// internal (an anchor to a bookmark).
func (h *Hyperlink) URL() string {
	if h.h.RID == "" {
		return ""
	}
	for _, rel := range h.document.relationships[h.part] {
		if rel != nil && rel.ID == h.h.RID {
			return rel.Target
		}
	}
	return ""
}

// Anchor returns the hyperlink's internal target — a bookmark name — or "" when
// the hyperlink is external.
func (h *Hyperlink) Anchor() string { return h.h.Anchor }

// Tooltip returns the hyperlink's screen-tip, or "" when it has none.
func (h *Hyperlink) Tooltip() string { return h.h.Tooltip }

// Text returns the hyperlink's display text (docx-specific), concatenated from
// its child runs.
func (h *Hyperlink) Text() string {
	s := ""
	for _, r := range h.h.R {
		if r == nil {
			continue
		}
		for _, t := range r.T {
			s += t.Text
		}
	}
	return s
}

// SetTooltip sets the hyperlink's screen-tip.
func (h *Hyperlink) SetTooltip(tooltip string) { h.h.Tooltip = tooltip }

// --- read ---

// Hyperlink returns the hyperlink wrapping this run, or nil when the run is not
// inside a hyperlink. Runs() alone cannot reach hyperlinked text; this exposes
// the URL that was otherwise invisible.
func (r *Run) Hyperlink() *Hyperlink {
	p := r.paragraph
	if p == nil {
		return nil
	}
	part := p.ownerPartName()
	for _, h := range p.p.Hyperlink {
		if h == nil {
			continue
		}
		for _, hr := range h.R {
			if hr == r.r {
				return &Hyperlink{document: p.document, h: h, part: part, para: p}
			}
		}
	}
	return nil
}

// Hyperlinks returns the hyperlinks directly in this paragraph, in document
// order.
func (p *Paragraph) Hyperlinks() []*Hyperlink {
	part := p.ownerPartName()
	out := make([]*Hyperlink, 0, len(p.p.Hyperlink))
	for _, h := range p.p.Hyperlink {
		if h != nil {
			out = append(out, &Hyperlink{document: p.document, h: h, part: part, para: p})
		}
	}
	return out
}

// Hyperlinks returns every hyperlink in the document, in document order,
// including hyperlinks nested in tables and structured document tags.
func (d *Document) Hyperlinks() []*Hyperlink {
	var out []*Hyperlink
	if d.document != nil && d.document.Body != nil {
		for _, cp := range d.document.Body.AllParagraphs() {
			para := &Paragraph{document: d, p: cp}
			for _, h := range cp.Hyperlink {
				if h != nil {
					out = append(out, &Hyperlink{document: d, h: h, part: d.mainPart(), para: para})
				}
			}
		}
	}
	for name, hp := range d.headers {
		if hp == nil || hp.hdr == nil {
			continue
		}
		for _, cp := range hp.hdr.P {
			para := &Paragraph{document: d, p: cp, hfPart: name}
			for _, h := range cp.Hyperlink {
				if h != nil {
					out = append(out, &Hyperlink{document: d, h: h, part: name, para: para})
				}
			}
		}
	}
	for name, fp := range d.footers {
		if fp == nil || fp.ftr == nil {
			continue
		}
		for _, cp := range fp.ftr.P {
			para := &Paragraph{document: d, p: cp, hfPart: name}
			for _, h := range cp.Hyperlink {
				if h != nil {
					out = append(out, &Hyperlink{document: d, h: h, part: name, para: para})
				}
			}
		}
	}
	return out
}

// --- write ---

// AddHyperlink appends a hyperlinked run displaying text and pointing at the
// external URL. It allocates a w:hyperlink wrapping the run with an r:id that
// resolves to an External relationship (RelTypeHyperlink) in the part's
// relationships.
func (p *Paragraph) AddHyperlink(text, url string) *Hyperlink {
	owner := p.ownerPartName()
	relID := fmt.Sprintf("rId%d", p.document.nextRelID())
	p.document.addPartRelationship(owner, &opc.Relationship{
		ID:         relID,
		Type:       opc.RelTypeHyperlink,
		Target:     url,
		TargetMode: opc.TargetModeExternal,
	})
	h := &oxml.CT_Hyperlink{RID: relID, History: "1"}
	h.R = []*oxml.CT_R{newHyperlinkRun(text)}
	p.p.AppendHyperlink(h)
	return &Hyperlink{document: p.document, h: h, part: owner, para: p}
}

// AddInternalHyperlink appends a hyperlinked run displaying text and pointing at
// an in-document bookmark (w:anchor). No relationship is created; compose it
// with AddBookmark to link to a bookmark added in the same session.
func (p *Paragraph) AddInternalHyperlink(text, bookmarkName string) *Hyperlink {
	owner := p.ownerPartName()
	h := &oxml.CT_Hyperlink{Anchor: bookmarkName, History: "1"}
	h.R = []*oxml.CT_R{newHyperlinkRun(text)}
	p.p.AppendHyperlink(h)
	return &Hyperlink{document: p.document, h: h, part: owner, para: p}
}

// newHyperlinkRun builds a run carrying the hyperlink display text, styled with
// the Hyperlink character style as Word emits it.
func newHyperlinkRun(text string) *oxml.CT_R {
	r := &oxml.CT_R{RPr: &oxml.CT_RPr{RStyle: &oxml.CT_String{Val: hyperlinkStyleID}}}
	r.SetTexts([]*oxml.CT_Text{{Space: "preserve", Text: text}})
	return r
}

// ownerPartName returns the part that owns the paragraph's relationships: the
// header/footer part for page-furniture paragraphs, the main document part
// otherwise.
func (p *Paragraph) ownerPartName() string {
	if p.hfPart != "" {
		return p.hfPart
	}
	return p.document.mainPart()
}
