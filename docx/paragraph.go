package docx

import (
	"math"
	"strconv"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

// Paragraph represents a paragraph in a Word document.
type Paragraph struct {
	document *Document
	p        *oxml.CT_P
	// hfPart names the header/footer part that owns this paragraph (e.g.
	// "/word/header1.xml"). Empty for paragraphs in the main document part.
	// Part-scoped resources (image relationships) are registered against the
	// owning part so their r:embed references resolve from that part's rels.
	hfPart string
}

// Text returns the text content of the paragraph, including text nested in
// hyperlinks, simple fields, tracked insertions, and structured document tags.
func (p *Paragraph) Text() string {
	return p.p.Text()
}

// SetText sets the text content, replacing ALL content children — runs,
// hyperlinks, structured document tags, tracked changes, fields, and
// raw-preserved inline elements — so no stale text (e.g. hyperlink display
// text) survives next to the new content. Paragraph properties are kept.
//
// Relationships that only the removed content referenced (a hyperlink's
// External rel, an image's r:embed, and any media part added in this session
// that nothing else points at) are reclaimed, so repeatedly filling a template
// no longer accretes dead relationships (C407).
func (p *Paragraph) SetText(text string) {
	p.touch()
	removed := make(map[string]bool)
	addParagraphRelRefs(removed, p.p)
	p.p.ClearContent()
	p.p.AppendR(&oxml.CT_R{
		T: []*oxml.CT_Text{{Space: "preserve", Text: text}},
	})
	p.sweepRemovedRelRefs(removed)
}

// Runs returns all runs in the paragraph.
func (p *Paragraph) Runs() []*Run {
	runs := make([]*Run, len(p.p.R))
	for i, r := range p.p.R {
		runs[i] = &Run{paragraph: p, r: r}
	}
	return runs
}

// AddRun adds a new run to the paragraph.
func (p *Paragraph) AddRun() *Run {
	p.touch()
	r := &oxml.CT_R{}
	p.p.AppendR(r)
	return &Run{paragraph: p, r: r}
}

// Style returns the paragraph style name.
func (p *Paragraph) Style() string {
	if p.p.PPr != nil && p.p.PPr.PStyle != nil {
		return p.p.PPr.PStyle.Val
	}
	return ""
}

// SetStyle sets the paragraph style.
func (p *Paragraph) SetStyle(style string) {
	p.touch()
	if p.p.PPr == nil {
		p.p.PPr = &oxml.CT_PPr{}
	}
	p.p.PPr.PStyle = &oxml.CT_String{Val: style}
}

// Alignment returns the paragraph alignment, or AlignmentLeft when the
// paragraph declares none — which conflates "unset, so inherited from the
// style" with an explicit left alignment. Use AlignmentOK to tell them apart.
func (p *Paragraph) Alignment() Alignment {
	a, _ := p.AlignmentOK()
	return a
}

// AlignmentOK returns the paragraph's own alignment (w:jc) and whether it
// declares one. It is the ok-bool form the newer getters use; Alignment keeps
// the older single-value shape. An unrecognized w:jc value (there are a dozen
// beyond the four this package models — distribute, thaiDistribute, ...)
// reports AlignmentLeft with ok true: the paragraph does declare an alignment,
// it is just not one this API can name.
func (p *Paragraph) AlignmentOK() (Alignment, bool) {
	if p.p.PPr == nil || p.p.PPr.Jc == nil {
		return AlignmentLeft, false
	}
	switch p.p.PPr.Jc.Val {
	case "center":
		return AlignmentCenter, true
	case "right", "end":
		return AlignmentRight, true
	case "both", "distribute":
		return AlignmentJustify, true
	}
	return AlignmentLeft, true
}

// SetAlignment sets the paragraph alignment.
func (p *Paragraph) SetAlignment(align Alignment) {
	p.touch()
	if p.p.PPr == nil {
		p.p.PPr = &oxml.CT_PPr{}
	}
	val := "left"
	switch align {
	case AlignmentCenter:
		val = "center"
	case AlignmentRight:
		val = "right"
	case AlignmentJustify:
		val = "both"
	}
	p.p.PPr.Jc = &oxml.CT_Jc{Val: val}
}

// Clear removes all runs from the paragraph, including their entries in the
// recorded child order, so a later AddRun does not resolve a stale reference
// to the new run and duplicate it. Hyperlinks and other non-run children are
// kept — use SetText to replace everything.
//
// Relationships referenced only by the removed runs are reclaimed (C407).
func (p *Paragraph) Clear() {
	p.touch()
	removed := make(map[string]bool)
	for _, r := range p.p.R {
		addRunRelRefs(removed, r)
	}
	p.p.SetRuns(nil)
	p.sweepRemovedRelRefs(removed)
}

// --- Spacing ---

// SpaceBefore returns the spacing before the paragraph in points.
func (p *Paragraph) SpaceBefore() float64 {
	if p.p.PPr != nil && p.p.PPr.Spacing != nil && p.p.PPr.Spacing.Before != "" {
		return twipsToPoints(p.p.PPr.Spacing.Before)
	}
	return 0
}

// SetSpaceBefore sets the spacing before the paragraph in points.
func (p *Paragraph) SetSpaceBefore(points float64) {
	p.ensureSpacing().Before = pointsToTwips(points)
}

// SpaceAfter returns the spacing after the paragraph in points.
func (p *Paragraph) SpaceAfter() float64 {
	if p.p.PPr != nil && p.p.PPr.Spacing != nil && p.p.PPr.Spacing.After != "" {
		return twipsToPoints(p.p.PPr.Spacing.After)
	}
	return 0
}

// SetSpaceAfter sets the spacing after the paragraph in points.
func (p *Paragraph) SetSpaceAfter(points float64) {
	p.ensureSpacing().After = pointsToTwips(points)
}

// SetLineSpacing sets proportional line spacing. 1.0 = single, 1.5, 2.0, etc.
// Internally this uses lineRule="auto" with the value in 240ths of a line.
func (p *Paragraph) SetLineSpacing(multiplier float64) {
	sp := p.ensureSpacing()
	// Word stores auto line spacing as 240 * multiplier
	sp.Line = strconv.Itoa(int(math.Round(multiplier * 240)))
	sp.LineRule = "auto"
}

// SetLineSpacingExact sets exact line spacing in points.
func (p *Paragraph) SetLineSpacingExact(points float64) {
	sp := p.ensureSpacing()
	sp.Line = pointsToTwips(points)
	sp.LineRule = "exact"
}

// --- Indentation ---

// SetIndentLeft sets the left indentation in points.
func (p *Paragraph) SetIndentLeft(points float64) {
	p.ensureInd().Left = pointsToTwips(points)
}

// SetIndentRight sets the right indentation in points.
func (p *Paragraph) SetIndentRight(points float64) {
	p.ensureInd().Right = pointsToTwips(points)
}

// SetIndentFirstLine sets the first-line indent in points.
func (p *Paragraph) SetIndentFirstLine(points float64) {
	ind := p.ensureInd()
	ind.FirstLine = pointsToTwips(points)
	ind.Hanging = "" // mutually exclusive with hanging
}

// SetIndentHanging sets the hanging indent in points.
func (p *Paragraph) SetIndentHanging(points float64) {
	ind := p.ensureInd()
	ind.Hanging = pointsToTwips(points)
	ind.FirstLine = "" // mutually exclusive with first-line
}

// --- Tab stops ---

// TabAlignment names the alignment of a paragraph tab stop (w:tab@val,
// ST_TabJc). The string values are the WordprocessingML tokens.
type TabAlignment string

const (
	TabAlignLeft    TabAlignment = "left"
	TabAlignCenter  TabAlignment = "center"
	TabAlignRight   TabAlignment = "right"
	TabAlignDecimal TabAlignment = "decimal"
	TabAlignBar     TabAlignment = "bar"
	TabAlignClear   TabAlignment = "clear"
)

// TabLeader names the leader character drawn in the space a tab stop spans
// (w:tab@leader, ST_TabTlc). The string values are the WordprocessingML tokens.
type TabLeader string

const (
	TabLeaderNone       TabLeader = "none"
	TabLeaderDot        TabLeader = "dot"
	TabLeaderHyphen     TabLeader = "hyphen"
	TabLeaderUnderscore TabLeader = "underscore"
	TabLeaderHeavy      TabLeader = "heavy"
	TabLeaderMiddleDot  TabLeader = "middleDot"
)

// TabStop describes a single paragraph tab stop.
type TabStop struct {
	// Position is the tab stop position in points, measured from the paragraph
	// left margin (or from the value's reference for bar/right stops).
	Position float64
	// Alignment is the tab stop alignment. The empty value is treated as left.
	Alignment TabAlignment
	// Leader is the leader character; the empty value (or TabLeaderNone) draws
	// no leader.
	Leader TabLeader
}

// Tabs returns the paragraph's explicit tab stops (w:tabs), in document order.
func (p *Paragraph) Tabs() []TabStop {
	if p.p.PPr == nil || p.p.PPr.Tabs == nil {
		return nil
	}
	stops := make([]TabStop, 0, len(p.p.PPr.Tabs.Tab))
	for i := range p.p.PPr.Tabs.Tab {
		t := &p.p.PPr.Tabs.Tab[i]
		stops = append(stops, TabStop{
			Position:  twipsToPoints(t.Pos),
			Alignment: TabAlignment(t.Val),
			Leader:    TabLeader(t.Leader),
		})
	}
	return stops
}

// AddTabStop appends a tab stop at the given position (in points) with the
// given alignment and leader. A zero Alignment defaults to a left tab; a zero
// (or TabLeaderNone) Leader draws no leader.
func (p *Paragraph) AddTabStop(stop TabStop) {
	p.ensurePPr()
	if p.p.PPr.Tabs == nil {
		p.p.PPr.Tabs = &oxml.CT_Tabs{}
	}
	align := stop.Alignment
	if align == "" {
		align = TabAlignLeft
	}
	ts := oxml.CT_TabStop{
		Val: string(align),
		Pos: pointsToTwips(stop.Position),
	}
	if stop.Leader != "" && stop.Leader != TabLeaderNone {
		ts.Leader = string(stop.Leader)
	}
	p.p.PPr.Tabs.Tab = append(p.p.PPr.Tabs.Tab, ts)
}

// ClearTabStops removes all explicit tab stops from the paragraph.
func (p *Paragraph) ClearTabStops() {
	p.touch()
	if p.p.PPr != nil {
		p.p.PPr.Tabs = nil
	}
}

// --- Flow Control ---

// SetKeepWithNext sets whether the paragraph should be kept with the next paragraph.
func (p *Paragraph) SetKeepWithNext(keep bool) {
	p.ensurePPr()
	if keep {
		p.p.PPr.KeepNext = &oxml.CT_OnOff{}
	} else {
		p.p.PPr.KeepNext = nil
	}
}

// SetKeepTogether sets whether the paragraph lines should be kept together on one page.
func (p *Paragraph) SetKeepTogether(keep bool) {
	p.ensurePPr()
	if keep {
		p.p.PPr.KeepLines = &oxml.CT_OnOff{}
	} else {
		p.p.PPr.KeepLines = nil
	}
}

// SetPageBreakBefore sets whether a page break should occur before the paragraph.
func (p *Paragraph) SetPageBreakBefore(brk bool) {
	p.ensurePPr()
	if brk {
		p.p.PPr.PageBreakBefore = &oxml.CT_OnOff{}
	} else {
		p.p.PPr.PageBreakBefore = nil
	}
}

// --- helpers ---

// touch flags the header/footer part this paragraph belongs to as modified, so
// an edit made through a live handle into a reopened header/footer is written
// back instead of being masked by the preserved original bytes. It is a no-op
// for paragraphs in the main document part (markHdrFtrModified only acts on a
// preserved header/footer part).
func (p *Paragraph) touch() {
	if p == nil || p.document == nil {
		return
	}
	// Every body, header and footer mutator reaches this funnel, so it is also
	// where the content edit is recorded for dcterms:modified (see
	// modified.go). Unlike the header/footer flag below, that record is not
	// conditional on the story: an edit to a main-part paragraph needs no
	// regeneration flag, but it is still an edit.
	p.document.markEdited()
	if p.hfPart != "" {
		p.document.markHdrFtrModified(p.hfPart)
	}
}

func (p *Paragraph) ensurePPr() {
	// Property setters funnel through here before mutating, so flagging the
	// owning header/footer part here covers them in one place. A no-op for
	// paragraphs in the main document part.
	p.touch()
	if p.p.PPr == nil {
		p.p.PPr = &oxml.CT_PPr{}
	}
}

func (p *Paragraph) ensureSpacing() *oxml.CT_Spacing {
	p.ensurePPr()
	if p.p.PPr.Spacing == nil {
		p.p.PPr.Spacing = &oxml.CT_Spacing{}
	}
	return p.p.PPr.Spacing
}

func (p *Paragraph) ensureInd() *oxml.CT_Ind {
	p.ensurePPr()
	if p.p.PPr.Ind == nil {
		p.p.PPr.Ind = &oxml.CT_Ind{}
	}
	return p.p.PPr.Ind
}

// pointsToTwips converts points to twips (1 point = 20 twips).
func pointsToTwips(points float64) string {
	return strconv.Itoa(int(math.Round(points * 20)))
}

// twipsToPoints converts a twips string to points.
func twipsToPoints(twips string) float64 {
	v, err := strconv.Atoi(twips)
	if err != nil {
		return 0
	}
	return float64(v) / 20.0
}

// Alignment represents paragraph alignment.
type Alignment int

const (
	AlignmentLeft Alignment = iota
	AlignmentCenter
	AlignmentRight
	AlignmentJustify
)
