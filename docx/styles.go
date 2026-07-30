package docx

import (
	"math"
	"strconv"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/docx/internal/oxml"
)

// StyleType enumerates the WordprocessingML style categories (the w:type
// attribute of a w:style element).
type StyleType string

const (
	// StyleTypeParagraph is a paragraph style, applied via Paragraph.SetStyle.
	StyleTypeParagraph StyleType = "paragraph"
	// StyleTypeCharacter is a character (run) style.
	StyleTypeCharacter StyleType = "character"
	// StyleTypeTable is a table style.
	StyleTypeTable StyleType = "table"
	// StyleTypeNumbering is a numbering style.
	StyleTypeNumbering StyleType = "numbering"
)

// StyleManager provides create and modify access to a document's style
// definitions (word/styles.xml). Obtain one with Document.Styles.
//
// Reading through the manager (Style, List) never marks the styles part
// modified; only the mutating methods on the manager and on the returned Style
// builders do, so an unmodified document still round-trips byte-for-byte.
type StyleManager struct {
	document *Document
}

// Styles returns the manager for the document's style definitions. It lazily
// materializes the styles model — a created document (or one opened without a
// styles part) gets Word's compact defaults (Normal plus Heading1-9) so that
// styles added here sit alongside the built-ins that AddHeading references.
func (d *Document) Styles() *StyleManager {
	d.ensureStyles()
	return &StyleManager{document: d}
}

// ensureStyles guarantees d.styles is non-nil, seeding it with the same default
// set the create path writes.
func (d *Document) ensureStyles() {
	if d.styles == nil {
		d.styles = defaultStyles()
	}
}

// Style is a builder over a single style definition (w:style). Its setters
// return the receiver so calls chain; each one marks the styles part modified.
type Style struct {
	document *Document
	s        *oxml.CT_Style
}

// AddParagraphStyle creates a new paragraph style with the given style id and
// display name and returns a builder for it. If a style with the id already
// exists it is returned as-is (its type and name left untouched) so the method
// is idempotent.
func (m *StyleManager) AddParagraphStyle(id, name string) *Style {
	return m.AddStyle(StyleTypeParagraph, id, name)
}

// AddCharacterStyle creates a new character (run) style; see AddParagraphStyle.
func (m *StyleManager) AddCharacterStyle(id, name string) *Style {
	return m.AddStyle(StyleTypeCharacter, id, name)
}

// AddStyle creates a new style of the given type with the given id and display
// name and returns a builder.
//
// It is idempotent on the style id: an existing style with that id is returned
// unchanged, whatever its type. Requesting a character style whose id is
// already taken by a paragraph style therefore hands back the paragraph style —
// the caller's styleType and name are silently ignored rather than converting
// or replacing the definition, since either would change how existing
// paragraphs render. Check Style.Type on the returned builder when the type
// matters, or pick an unused id.
//
// There is no RemoveStyle: no part of the docx feature API deletes, so a
// replace-style edit accretes definitions rather than replacing them.
func (m *StyleManager) AddStyle(styleType StyleType, id, name string) *Style {
	if existing := m.style(id); existing != nil {
		return existing
	}
	s := &oxml.CT_Style{
		Type:    string(styleType),
		StyleId: id,
		Name:    &oxml.CT_String{Val: name},
	}
	m.document.styles.Style = append(m.document.styles.Style, s)
	m.document.stylesModified = true
	return &Style{document: m.document, s: s}
}

// Style returns the style with the given id, or nil if none exists. Fetching a
// style does not mark the part modified; mutating the returned builder does.
func (m *StyleManager) Style(id string) *Style {
	return m.style(id)
}

// style is the shared lookup used by Style and the Add* methods.
func (m *StyleManager) style(id string) *Style {
	for _, s := range m.document.styles.Style {
		if s.StyleId == id {
			return &Style{document: m.document, s: s}
		}
	}
	return nil
}

// List returns a builder for every style defined in the document, in document
// order.
func (m *StyleManager) List() []*Style {
	list := make([]*Style, 0, len(m.document.styles.Style))
	for _, s := range m.document.styles.Style {
		list = append(list, &Style{document: m.document, s: s})
	}
	return list
}

// --- identity / metadata ---

// ID returns the style id (w:styleId), the value Paragraph.SetStyle references.
func (s *Style) ID() string { return s.s.StyleId }

// Name returns the style's display name.
func (s *Style) Name() string {
	if s.s.Name != nil {
		return s.s.Name.Val
	}
	return ""
}

// Type returns the style type.
func (s *Style) Type() StyleType { return StyleType(s.s.Type) }

// SetName sets the style's display name.
func (s *Style) SetName(name string) *Style {
	s.s.Name = &oxml.CT_String{Val: name}
	return s.modified()
}

// SetType sets the style type (paragraph, character, table, or numbering).
func (s *Style) SetType(styleType StyleType) *Style {
	s.s.Type = string(styleType)
	return s.modified()
}

// SetBasedOn sets the parent style (w:basedOn) this style inherits from.
//
// A value that would make the style inherit from itself — directly, or through
// a chain that leads back to it — is refused: the style keeps its previous
// parent and the styles part is not marked modified. Word repairs or misrenders
// a cyclic basedOn chain, and nothing downstream of the setter would have
// caught it (Validate checked dangling references, not cycles). Use
// BasedOnCycle to test a value before setting it, or Document.Validate to find
// a cycle a merge introduced (C501).
func (s *Style) SetBasedOn(id string) *Style {
	if id != "" && s.wouldCycle(id) {
		return s
	}
	s.s.BasedOn = &oxml.CT_String{Val: id}
	return s.modified()
}

// BasedOn returns the id of the style this one inherits from (w:basedOn), or ""
// when it inherits from nothing.
func (s *Style) BasedOn() string {
	if s.s.BasedOn == nil {
		return ""
	}
	return s.s.BasedOn.Val
}

// wouldCycle reports whether making this style inherit from parentID would
// close an inheritance cycle: either parentID is the style's own id, or the
// existing basedOn chain above parentID leads back to it.
func (s *Style) wouldCycle(parentID string) bool {
	if parentID == s.s.StyleId {
		return true
	}
	byID := s.document.stylesByID()
	seen := map[string]bool{s.s.StyleId: true}
	for cur := byID[parentID]; cur != nil; {
		if seen[cur.StyleId] {
			return true
		}
		seen[cur.StyleId] = true
		if cur.BasedOn == nil || cur.BasedOn.Val == "" {
			return false
		}
		cur = byID[cur.BasedOn.Val]
	}
	return false
}

// stylesByID indexes the document's style definitions by styleId. A duplicate
// id (legal only by accident) resolves to the first definition, matching the
// lookup StyleManager.Style performs.
func (d *Document) stylesByID() map[string]*oxml.CT_Style {
	out := make(map[string]*oxml.CT_Style)
	if d.styles == nil {
		return out
	}
	for _, st := range d.styles.Style {
		if st != nil && st.StyleId != "" {
			if _, dup := out[st.StyleId]; !dup {
				out[st.StyleId] = st
			}
		}
	}
	return out
}

// SetNext sets the style (w:next) applied to the following paragraph when the
// user presses Enter at the end of a paragraph carrying this style.
func (s *Style) SetNext(id string) *Style {
	s.s.Next = &oxml.CT_String{Val: id}
	return s.modified()
}

// SetLink links a paragraph style to its companion character style (w:link),
// forming a linked style pair.
func (s *Style) SetLink(id string) *Style {
	s.s.Link = &oxml.CT_String{Val: id}
	return s.modified()
}

// SetQuickFormat toggles whether the style appears in the application's gallery
// of recommended styles (w:qFormat).
func (s *Style) SetQuickFormat(on bool) *Style {
	if on {
		s.s.QFormat = &oxml.CT_OnOff{}
	} else {
		s.s.QFormat = nil
	}
	return s.modified()
}

// SetUIPriority sets the sort order (w:uiPriority) the application uses when
// listing styles.
func (s *Style) SetUIPriority(priority int) *Style {
	s.s.UiPriority = &oxml.CT_DecimalNumber{Val: priority}
	return s.modified()
}

// --- paragraph properties (w:pPr) ---

// SetAlignment sets the paragraph alignment for this style (w:pPr/w:jc). It
// applies to paragraph and table styles.
func (s *Style) SetAlignment(align Alignment) *Style {
	val := "left"
	switch align {
	case AlignmentCenter:
		val = "center"
	case AlignmentRight:
		val = "right"
	case AlignmentJustify:
		val = "both"
	}
	s.ensurePPr().Jc = &oxml.CT_Jc{Val: val}
	return s.modified()
}

// SetSpaceBefore sets the spacing before paragraphs in points (w:pPr/w:spacing).
func (s *Style) SetSpaceBefore(points float64) *Style {
	s.ensureSpacing().Before = pointsToTwips(points)
	return s.modified()
}

// SetSpaceAfter sets the spacing after paragraphs in points (w:pPr/w:spacing).
func (s *Style) SetSpaceAfter(points float64) *Style {
	s.ensureSpacing().After = pointsToTwips(points)
	return s.modified()
}

// SetLineSpacing sets proportional line spacing (1.0 = single, 1.5, 2.0, …),
// stored as lineRule="auto" in 240ths of a line.
func (s *Style) SetLineSpacing(multiplier float64) *Style {
	sp := s.ensureSpacing()
	sp.Line = lineSpacingAuto(multiplier)
	sp.LineRule = "auto"
	return s.modified()
}

// SetIndentLeft sets the left indentation in points (w:pPr/w:ind).
func (s *Style) SetIndentLeft(points float64) *Style {
	s.ensureInd().Left = pointsToTwips(points)
	return s.modified()
}

// SetIndentFirstLine sets a positive first-line indent in points; it clears any
// hanging indent, since the two are mutually exclusive in w:ind.
func (s *Style) SetIndentFirstLine(points float64) *Style {
	ind := s.ensureInd()
	ind.FirstLine = pointsToTwips(points)
	ind.Hanging = ""
	return s.modified()
}

// SetIndentHanging sets a hanging indent in points; it clears any first-line
// indent, since the two are mutually exclusive in w:ind.
func (s *Style) SetIndentHanging(points float64) *Style {
	ind := s.ensureInd()
	ind.Hanging = pointsToTwips(points)
	ind.FirstLine = ""
	return s.modified()
}

// --- run properties (w:rPr) ---

// SetFont sets the style's font family (w:rPr/w:rFonts), for both the ASCII and
// high-ANSI ranges.
func (s *Style) SetFont(name string) *Style {
	rpr := s.ensureRPr()
	if rpr.RFonts == nil {
		rpr.RFonts = &oxml.CT_Fonts{}
	}
	rpr.RFonts.Ascii = name
	rpr.RFonts.HAnsi = name
	return s.modified()
}

// SetFontSize sets the font size in points (w:rPr/w:sz, stored in half-points).
func (s *Style) SetFontSize(points float64) *Style {
	rpr := s.ensureRPr()
	hp := halfPoints(points)
	rpr.Sz = &oxml.CT_HpsMeasure{Val: hp}
	rpr.SzCs = &oxml.CT_HpsMeasure{Val: hp}
	return s.modified()
}

// SetBold toggles bold (w:rPr/w:b and w:bCs).
func (s *Style) SetBold(on bool) *Style {
	rpr := s.ensureRPr()
	if on {
		rpr.B = &oxml.CT_OnOff{}
		rpr.BCs = &oxml.CT_OnOff{}
	} else {
		rpr.B = nil
		rpr.BCs = nil
	}
	return s.modified()
}

// SetItalic toggles italic (w:rPr/w:i and w:iCs).
func (s *Style) SetItalic(on bool) *Style {
	rpr := s.ensureRPr()
	if on {
		rpr.I = &oxml.CT_OnOff{}
		rpr.ICs = &oxml.CT_OnOff{}
	} else {
		rpr.I = nil
		rpr.ICs = nil
	}
	return s.modified()
}

// SetColor sets the text color as a hex string, e.g. "FF0000" (w:rPr/w:color).
func (s *Style) SetColor(color string) *Style {
	s.ensureRPr().Color = &oxml.CT_Color{Val: color}
	return s.modified()
}

// --- internal helpers ---

// modified flags the styles part dirty so the round-trip save regenerates it,
// and returns the receiver for chaining.
func (s *Style) modified() *Style {
	s.document.stylesModified = true
	return s
}

// ensurePPr and ensureRPr are the funnels every property setter on a style goes
// through, so flagging here covers them in one place: a setter that edited the
// returned properties without calling modified() would leave styles.xml
// round-tripping its preserved bytes and lose the edit (the C266/C406 shape).
func (s *Style) ensurePPr() *oxml.CT_PPr {
	s.modified()
	if s.s.PPr == nil {
		s.s.PPr = &oxml.CT_PPr{}
	}
	return s.s.PPr
}

func (s *Style) ensureRPr() *oxml.CT_RPr {
	s.modified()
	if s.s.RPr == nil {
		s.s.RPr = &oxml.CT_RPr{}
	}
	return s.s.RPr
}

func (s *Style) ensureSpacing() *oxml.CT_Spacing {
	ppr := s.ensurePPr()
	if ppr.Spacing == nil {
		ppr.Spacing = &oxml.CT_Spacing{}
	}
	return ppr.Spacing
}

func (s *Style) ensureInd() *oxml.CT_Ind {
	ppr := s.ensurePPr()
	if ppr.Ind == nil {
		ppr.Ind = &oxml.CT_Ind{}
	}
	return ppr.Ind
}

// halfPoints renders a point size as the half-point string w:sz stores (12pt →
// "24"). It is the single spelling for every half-point measure in the package
// — w:sz, w:position, w:kern — so a size cannot be written one way here and
// another way through Run.
func halfPoints(points float64) string {
	return xmlb.FormatFloat(points * 2)
}

// lineSpacingAuto renders a proportional line-spacing multiplier as the 240ths
// value w:spacing stores under lineRule="auto", matching Paragraph.SetLineSpacing.
func lineSpacingAuto(multiplier float64) string {
	return strconv.Itoa(int(math.Round(multiplier * 240)))
}
