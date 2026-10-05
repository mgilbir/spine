package docxrender

import (
	"fmt"
	"math"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/spine/render"
)

// Translation overview.
//
// The body of the document is walked once, in order, and turned into sections
// of wordBlock values. A block is one unit of the flow that pagination can place
// and (for text) split: a paragraph is one block, or several where a manual
// page break splits it. Each block carries the markup of its content and the
// flow properties pagination reads (spacing, keep rules, page breaks). Layout
// (render_layout.go) emits one forme document per section, with every block in
// a wrapper element carrying the block's index, lays it out once at the
// section's content width, and the paginator (render_paginate.go) cuts the
// result into pages at block and line boundaries.
//
// Adding a block kind. A block kind is an element that can appear in the body
// flow (w:tbl, w:sdt, ...). To add one:
//
//  1. Write a wordBlockFunc: it receives the flow and the element, appends
//     wordBlock values with wordFlow.add, and reports content it cannot draw
//     through the renderer's leaveOut and approximate methods, which fail in
//     strict mode and warn in best-effort mode.
//  2. Register it from an init function with wordRegisterBlock. The foundation
//     registers placeholders that leave the element out (wordLeftOutBlocks in
//     render_unsupported.go); remove the placeholder entry for the kind.
//  3. If the block does not split at text lines (a table splits between rows),
//     set wordBlock.units to return its break units from the laid-out fragment.
//  4. The block's content is generated markup: write only escaped text with
//     wordEscape, and only CSS built with wordCSS from numbers and from
//     wordFonts' synthetic families. Never copy a source string into markup.
//
// Run content (images, footnote references, ...) and paragraph content
// (hyperlinks, fields, ...) are extended the same way with wordRegisterRun and
// wordRegisterInline, a paragraph's start (a list marker) with
// wordParagraphStarts, and page furniture (headers, footers) with
// wordPageDecorators.

// wordBlock is one pagination unit of a section's flow.
type wordBlock struct {
	kind string
	// inner is the block's markup, placed inside a wrapper element.
	inner string
	// before and after are the paragraph spacing in pixels, drawn as wrapper
	// padding so they never collapse.
	before, after float64
	// beforeAuto and afterAuto mark HTML-style automatic spacing.
	beforeAuto, afterAuto bool
	contextual            bool
	styleID               string
	pageBreakBefore       bool
	keepNext, keepLines   bool
	widow                 bool
	// continuation is a block that continues a paragraph after a manual page
	// break: it has no space before and never suppresses at a page top.
	continuation bool
	// tab is a paragraph whose tabs are set by resolveTabs (render_tabs.go)
	// before layout; its inner markup is built there. expectLines is the
	// number of lines its text should fill, checked after layout.
	tab         *wordTabBlock
	expectLines int
	// units returns the places the block may be split, from its laid out
	// wrapper fragment. Nil selects the lines of the paragraph inside.
	units wordUnitsFunc
}

// wordUnit is an indivisible vertical extent of a block, in section layout
// pixels.
type wordUnit struct{ top, bottom float64 }

type wordUnitsFunc func(wrapper *layout.Fragment) []wordUnit

// wordSection is the blocks of one section and its page properties.
type wordSection struct {
	props  wordSectProps
	blocks []*wordBlock
	// marks are the footnote references of the section's text, in order, and
	// pool the blocks of their notes (render_notes.go).
	marks []*wordMark
	pool  []*wordBlock
	// hasEndnotes says the section's blocks end with endnotes, from block
	// endnotesFrom.
	hasEndnotes  bool
	endnotesFrom int
}

// wordFlow is the state of the body walk.
type wordFlow struct {
	r      *wordRenderer
	secs   []*wordSection
	cur    []*wordBlock
	fields []wordFieldState
	// breakPending is a manual page break that ended its paragraph: Word
	// keeps the paragraph mark on the page with the break, so the next block
	// starts the new page.
	breakPending bool
	// hf is set while a header or footer part is translated, and note while
	// the text of a note is.
	hf   *wordHFCtx
	note *wordNoteCtx
}

// visible reports whether content is part of drawn text, which it is outside
// fields and inside field results.
func (f *wordFlow) visible() bool {
	for _, s := range f.fields {
		if !s.result {
			return false
		}
	}
	return true
}

// add appends a block to the current section.
func (f *wordFlow) add(b *wordBlock) error {
	if err := f.r.charge(2); err != nil {
		return err
	}
	if f.breakPending {
		b.pageBreakBefore, f.breakPending = true, false
	}
	f.cur = append(f.cur, b)
	return nil
}

// wordBlockFunc translates one body-level element.
type wordBlockFunc func(f *wordFlow, n *wordNode) error

var wordBlockKinds = map[string]wordBlockFunc{}

// wordKey names an element for the registries: the local name for
// WordprocessingML, "{namespace}name" otherwise.
func wordKey(n *wordNode) string {
	if n.space == nsW {
		return n.name
	}
	return "{" + n.space + "}" + n.name
}

// wordRegisterBlock registers the translator of a body-level element kind. A
// kind may be registered once.
func wordRegisterBlock(key string, fn wordBlockFunc) {
	if _, dup := wordBlockKinds[key]; dup {
		panic("docxrender: duplicate block kind " + key)
	}
	wordBlockKinds[key] = fn
}

func init() {
	wordRegisterBlock("p", (*wordFlow).paragraph)
	wordRegisterBlock("sdt", (*wordFlow).blockContent)
	// Custom XML elements only wrap content.
	wordRegisterBlock("customXml", func(f *wordFlow, n *wordNode) error { return f.blocksOf(n) })
	for _, k := range []string{"bookmarkStart", "bookmarkEnd", "permStart", "permEnd", "proofErr"} {
		wordRegisterBlock(k, func(*wordFlow, *wordNode) error { return nil })
	}
}

// blockContent translates the blocks inside a transparent container
// (w:sdt/w:sdtContent).
func (f *wordFlow) blockContent(n *wordNode) error {
	content := n.child("sdtContent")
	if content == nil {
		return nil
	}
	return f.blocksOf(content)
}

// blocksOf translates the children of a container as body-level blocks.
func (f *wordFlow) blocksOf(parent *wordNode) error {
	for _, c := range parent.children {
		if err := f.r.ctx.Err(); err != nil {
			return err
		}
		fn := wordBlockKinds[wordKey(c)]
		if fn == nil {
			if err := f.r.leaveOut("block element " + wordKey(c)); err != nil {
				return err
			}
			continue
		}
		if err := fn(f, c); err != nil {
			return err
		}
	}
	return nil
}

// closeSection ends the current section with its sectPr.
func (f *wordFlow) closeSection(sectPr *wordNode) error {
	props, err := f.r.parseSectPr(sectPr)
	if err != nil {
		return err
	}
	props.idx = len(f.secs)
	ws := &wordSection{props: props, blocks: f.cur}
	if f.hf == nil && f.note == nil {
		if err = f.r.closeNotes(ws); err != nil {
			return err
		}
	}
	f.secs = append(f.secs, ws)
	f.cur = nil
	return nil
}

// translateBody walks w:body into sections.
func (r *wordRenderer) translateBody(body *wordNode) ([]*wordSection, error) {
	r.notes.reset()
	if err := r.scanNotes(body); err != nil {
		return nil, err
	}
	f := &wordFlow{r: r}
	var final *wordNode
	for _, c := range body.children {
		if err := r.ctx.Err(); err != nil {
			return nil, err
		}
		if c.is("sectPr") {
			final = c
			continue
		}
		fn := wordBlockKinds[wordKey(c)]
		if fn == nil {
			if err := r.leaveOut("block element " + wordKey(c)); err != nil {
				return nil, err
			}
			continue
		}
		if err := fn(f, c); err != nil {
			return nil, err
		}
	}
	if len(f.fields) > 0 {
		// A field left open at the end of the body: its remaining content was
		// instruction text and is not drawn. Not an error.
		f.fields = nil
	}
	if final == nil {
		// A body without section properties takes the default page.
		final = &wordNode{space: nsW, name: "sectPr"}
	}
	if len(f.cur) > 0 || len(f.secs) == 0 {
		if err := f.closeSection(final); err != nil {
			return nil, err
		}
	}
	if err := r.placeEndnotes(f.secs); err != nil {
		return nil, err
	}
	return f.secs, nil
}

// wordSectProps is a section's page geometry in pixels and its break type.
type wordSectProps struct {
	w, h                     float64
	top, right, bottom, left float64
	// typ is the break that starts the section: nextPage, continuous,
	// evenPage, oddPage.
	typ  string
	node *wordNode
	// idx is the section's position in the document.
	idx int
	// hdrDist and ftrDist are the distances of the header and footer from the
	// page edges (w:pgMar header and footer).
	hdrDist, ftrDist float64
	// header and footer are the part relationship ids the section references,
	// by type (wordHFDefault, wordHFFirst, wordHFEven). They are as written:
	// wordInheritHF fills the gaps from the previous section.
	header, footer [wordHFTypes]string
	// titlePg selects the first-page header and footer for the section's first
	// page.
	titlePg bool
	// pgStart restarts page numbering at the section's first page when
	// hasStart; pgFmt is the number format (w:pgNumType fmt), "" for decimal.
	hasStart bool
	pgStart  int
	pgFmt    string
}

// contentW is the width of the text area of a page of the section.
func (s wordSectProps) contentW() float64 { return s.w - s.left - s.right }

// parseSectPr reads a section's properties.
func (r *wordRenderer) parseSectPr(n *wordNode) (wordSectProps, error) {
	s := wordSectProps{typ: "nextPage", node: n, hdrDist: 720.0 / 15, ftrDist: 720.0 / 15}
	var hasSize, hasMar bool
	var gutter float64
	for _, c := range n.children {
		if c.space != nsW {
			if err := r.leaveOut("section property " + wordKey(c)); err != nil {
				return s, err
			}
			continue
		}
		var err error
		switch c.name {
		case "pgSz":
			hasSize = true
			if s.w, err = wordLength(attrOrEmpty(c, "w"), "w:pgSz"); err == nil {
				s.h, err = wordLength(attrOrEmpty(c, "h"), "w:pgSz")
			}
		case "pgMar":
			hasMar = true
			for _, m := range []struct {
				name string
				dst  *float64
			}{{"top", &s.top}, {"right", &s.right}, {"bottom", &s.bottom}, {"left", &s.left}} {
				if *m.dst, err = wordLength(attrOrEmpty(c, m.name), "w:pgMar"); err != nil {
					break
				}
			}
			if err == nil {
				if g, ok := c.attr("gutter"); ok {
					gutter, err = wordLength(g, "w:pgMar gutter")
				}
			}
			for _, m := range []struct {
				name string
				dst  *float64
			}{{"header", &s.hdrDist}, {"footer", &s.ftrDist}} {
				if v, ok := c.attr(m.name); ok && err == nil {
					*m.dst, err = wordLength(v, "w:pgMar "+m.name)
				}
			}
		case "type":
			switch v := c.val(); v {
			case "nextPage", "continuous", "evenPage", "oddPage":
				s.typ = v
			case "nextColumn":
				s.typ = "nextPage"
				err = r.approximate("section break type nextColumn")
			default:
				err = fmt.Errorf("%w: w:type", render.ErrInvalid)
			}
		case "cols":
			if v, ok := c.attr("num"); ok && v != "1" && v != "" {
				err = r.approximate("multiple text columns")
			}
		case "docGrid":
			// Without a grid type the pitch is only a default for tables.
			if v := attrOrEmpty(c, "type"); v != "" && v != "default" {
				err = r.approximate("document grid")
			}
		case "headerReference", "footerReference":
			err = s.addHFRef(c)
		case "titlePg":
			s.titlePg, err = wordOnOff(c, "w:titlePg")
		case "pgNumType":
			err = r.parsePgNumType(&s, c)
		case "pgBorders":
			err = r.leaveOut("page borders")
		case "lnNumType":
			err = r.leaveOut("line numbering")
		case "vAlign":
			if c.val() != "top" && c.val() != "" {
				err = r.leaveOut("vertical page alignment")
			}
		case "textDirection":
			if v := c.val(); v != "lrTb" && v != "lr" {
				err = r.leaveOut("section text direction")
			}
		case "bidi":
			if on, ok := c.on(); ok && on {
				err = r.approximate("right-to-left section")
			}
		case "sectPrChange":
			err = r.approximate("tracked section change")
		case "rtlGutter", "formProt", "noEndnote", "paperSrc", "printerSettings", "footnotePr", "endnotePr":
			// No effect on the body flow of the profile's pages.
		default:
			err = r.leaveOut("section property w:" + c.name)
		}
		if err != nil {
			return s, err
		}
	}
	if !hasSize || !hasMar {
		// The page Word's US Normal template sets: Letter with one-inch
		// margins. Where it comes from is the application's, so best effort
		// reports it.
		if err := r.approximate("page size or margins not set, drawn on Letter with one-inch margins"); err != nil {
			return s, err
		}
		if !hasSize {
			s.w, s.h = 8.5*96, 11*96
		}
		if !hasMar {
			s.top, s.right, s.bottom, s.left = 96, 96, 96, 96
		}
	}
	if s.top < 0 || s.bottom < 0 || s.left < 0 || s.right < 0 || gutter < 0 || s.hdrDist < 0 || s.ftrDist < 0 {
		return s, fmt.Errorf("%w: negative page margins", render.ErrUnsupported)
	}
	// The gutter widens the left margin; a top gutter is the gutterAtTop
	// setting, which loadSettings reports.
	s.left += gutter
	if s.w <= 0 || s.h <= 0 || s.left+s.right >= s.w || s.top+s.bottom >= s.h {
		return s, fmt.Errorf("%w: page geometry", render.ErrInvalid)
	}
	lim := r.limits
	if math.Ceil(s.w) > float64(lim.MaxDimension) || math.Ceil(s.h) > float64(lim.MaxDimension) || math.Ceil(s.w)*math.Ceil(s.h) > float64(lim.MaxPixels) {
		return s, render.ErrLimit
	}
	return s, nil
}
