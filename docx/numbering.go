package docx

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

// NumberFormat enumerates the numbering formats a list level can use (the
// w:numFmt value of a numbering level). The set covers the common formats;
// any other OOXML value can be passed as NumberFormat("…").
type NumberFormat string

const (
	// NumberFormatDecimal numbers levels 1, 2, 3, …
	NumberFormatDecimal NumberFormat = "decimal"
	// NumberFormatBullet renders a static bullet glyph (the level text) instead
	// of a counter.
	NumberFormatBullet NumberFormat = "bullet"
	// NumberFormatLowerRoman numbers levels i, ii, iii, …
	NumberFormatLowerRoman NumberFormat = "lowerRoman"
	// NumberFormatUpperRoman numbers levels I, II, III, …
	NumberFormatUpperRoman NumberFormat = "upperRoman"
	// NumberFormatLowerLetter numbers levels a, b, c, …
	NumberFormatLowerLetter NumberFormat = "lowerLetter"
	// NumberFormatUpperLetter numbers levels A, B, C, …
	NumberFormatUpperLetter NumberFormat = "upperLetter"
	// NumberFormatOrdinal numbers levels 1st, 2nd, 3rd, …
	NumberFormatOrdinal NumberFormat = "ordinal"
	// NumberFormatCardinalText spells the number: one, two, three, …
	NumberFormatCardinalText NumberFormat = "cardinalText"
	// NumberFormatOrdinalText spells the ordinal: first, second, third, …
	NumberFormatOrdinalText NumberFormat = "ordinalText"
	// NumberFormatNone renders no counter.
	NumberFormatNone NumberFormat = "none"
)

// NumberingManager provides create access to custom list/numbering definitions
// (word/numbering.xml). Obtain one with Document.Numbering (or the
// ListDefinitions alias).
//
// Definitions added here layer on top of any numbering the document already
// carried: existing definitions are preserved verbatim (so an unmodified part
// round-trips byte-for-byte), and new abstract/instance definitions are
// appended in schema position.
type NumberingManager struct {
	document *Document
}

// Numbering returns the manager for the document's numbering definitions,
// creating the numbering model if the document has none.
func (d *Document) Numbering() *NumberingManager {
	d.ensureNumbering()
	return &NumberingManager{document: d}
}

// ListDefinitions is an alias for Numbering, spelling the manager in terms of
// the list definitions it builds.
func (d *Document) ListDefinitions() *NumberingManager {
	return d.Numbering()
}

// ListDefinition is a builder for a custom multi-level numbering definition (a
// w:abstractNum). Configure its levels, then obtain a ListStyle with ListStyle
// to apply the definition to paragraphs via Paragraph.SetListStyle.
//
// A definition describes how a list *looks*; the counters live on the numbering
// instances built from it (ListStyle and RestartedListStyle), which is what
// makes "restart this list at 1" a matter of allocating a second instance
// rather than editing the definition.
//
// The builder covers the level properties Word's list dialog exposes (format,
// level text, start value, font, indent, hanging indent, justification). Level
// properties the model carries but does not surface here — w:lvlRestart,
// w:isLgl, w:suff, w:pStyle, w:numStyleLink — are not settable; a definition
// parsed from an opened package keeps them, since existing definitions are
// preserved verbatim and never rewritten from this model.
type ListDefinition struct {
	document *Document
	abstract *oxml.CT_AbstractNum
	absID    int
	// numID caches the numbering-instance id allocated by the first ListStyle
	// call so repeated calls return the same instance.
	numID  int
	levels map[int]*oxml.CT_Lvl
}

// AddDefinition creates a new, empty abstract numbering definition and returns a
// builder for it. The definition is registered immediately; configuring its
// levels through the returned builder mutates it in place.
func (nm *NumberingManager) AddDefinition() *ListDefinition {
	d := nm.document
	absID := d.nextAbstractNumID()
	abstract := &oxml.CT_AbstractNum{
		AbstractNumId:  strconv.Itoa(absID),
		MultiLevelType: &oxml.CT_String{Val: "hybridMultilevel"},
	}
	d.numbering.AbstractNum = append(d.numbering.AbstractNum, abstract)
	d.numberingModified = true
	return &ListDefinition{
		document: d,
		abstract: abstract,
		absID:    absID,
		levels:   make(map[int]*oxml.CT_Lvl),
	}
}

// AbstractNumID returns the definition's abstract numbering id (w:abstractNumId).
func (ld *ListDefinition) AbstractNumID() int { return ld.absID }

// Level returns the builder for the given level (0-based), creating it with
// sensible defaults (decimal format, "%n." level text, start 1, and the
// standard 0.5"-per-level indent with a 0.25" hang) on first access. Levels are
// kept in ascending order regardless of the order they are first touched.
func (ld *ListDefinition) Level(level int) *ListLevel {
	if lvl, ok := ld.levels[level]; ok {
		return &ListLevel{document: ld.document, lvl: lvl}
	}
	indent := 720 * (level + 1) // 720 twips = 0.5 inch per level
	lvl := &oxml.CT_Lvl{
		Ilvl:    strconv.Itoa(level),
		Start:   &oxml.CT_DecimalNumber{Val: 1},
		NumFmt:  &oxml.CT_NumFmt{Val: string(NumberFormatDecimal)},
		LvlText: &oxml.CT_LvlText{Val: fmt.Sprintf("%%%d.", level+1)},
		LvlJc:   &oxml.CT_Jc{Val: "left"},
		PPr: &oxml.CT_PPr{
			Ind: &oxml.CT_Ind{
				Left:    strconv.Itoa(indent),
				Hanging: "360", // 360 twips = 0.25 inch
			},
		},
	}
	ld.levels[level] = lvl
	ld.rebuildLevels()
	ld.document.numberingModified = true
	return &ListLevel{document: ld.document, lvl: lvl}
}

// SetLevel is a shorthand that configures a level's format and level text in one
// call and returns the level builder for any further tuning.
func (ld *ListDefinition) SetLevel(level int, format NumberFormat, lvlText string) *ListLevel {
	return ld.Level(level).SetFormat(format).SetText(lvlText)
}

// rebuildLevels rewrites the abstract's Lvl slice in ascending ilvl order so the
// serialized definition is well-ordered no matter how the caller built it.
func (ld *ListDefinition) rebuildLevels() {
	keys := make([]int, 0, len(ld.levels))
	for k := range ld.levels {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	ld.abstract.Lvl = ld.abstract.Lvl[:0]
	for _, k := range keys {
		ld.abstract.Lvl = append(ld.abstract.Lvl, ld.levels[k])
	}
	// Reordering the definition's levels is an edit to numbering.xml, so it
	// flags the part here rather than relying on every caller (C406).
	ld.document.numberingModified = true
}

// ListStyle registers a numbering instance (w:num) pointing at this definition
// and returns a ListStyle applicable to paragraphs via Paragraph.SetListStyle.
// Repeated calls return the same instance.
func (ld *ListDefinition) ListStyle() *ListStyle {
	if ld.numID == 0 {
		ld.numID = ld.document.nextNumID()
		ld.document.numbering.Num = append(ld.document.numbering.Num, &oxml.CT_Num{
			NumId:         strconv.Itoa(ld.numID),
			AbstractNumId: &oxml.CT_DecimalNumber{Val: ld.absID},
		})
		ld.document.numberingModified = true
	}
	return &ListStyle{document: ld.document, numID: ld.numID}
}

// RestartedListStyle registers a *second* numbering instance for this
// definition whose counter for the given level restarts at start
// (w:num/w:lvlOverride/w:startOverride), and returns a ListStyle for it.
//
// This is how "restart this list at 1" is expressed in WordprocessingML: the
// abstract definition is shared, but each numbering instance counts
// independently, and a startOverride resets the instance's counter. Apply the
// definition's ListStyle to the paragraphs before the restart point and the one
// returned here to the paragraphs after it:
//
//	def := doc.Numbering().AddDefinition()
//	def.SetLevel(0, docx.NumberFormatDecimal, "%1.")
//	first, second := def.ListStyle(), def.RestartedListStyle(0, 1)
//	doc.AddParagraph().SetListStyle(first, 0)  // 1.
//	doc.AddParagraph().SetListStyle(first, 0)  // 2.
//	doc.AddParagraph().SetListStyle(second, 0) // 1. again
//
// Each call registers a new instance, so a list restarted several times calls
// it once per restart.
func (ld *ListDefinition) RestartedListStyle(level, start int) *ListStyle {
	numID := ld.document.nextNumID()
	ld.document.numbering.Num = append(ld.document.numbering.Num, &oxml.CT_Num{
		NumId:         strconv.Itoa(numID),
		AbstractNumId: &oxml.CT_DecimalNumber{Val: ld.absID},
		LvlOverride: []*oxml.CT_NumLvl{{
			Ilvl:          strconv.Itoa(level),
			StartOverride: &oxml.CT_DecimalNumber{Val: start},
		}},
	})
	ld.document.numberingModified = true
	return &ListStyle{document: ld.document, numID: numID}
}

// ListLevel is a builder over a single level of a ListDefinition.
type ListLevel struct {
	document *Document
	lvl      *oxml.CT_Lvl
}

// SetFormat sets the level's numbering format (w:numFmt), e.g. decimal or bullet.
func (l *ListLevel) SetFormat(format NumberFormat) *ListLevel {
	l.lvl.NumFmt = &oxml.CT_NumFmt{Val: string(format)}
	return l.modified()
}

// SetText sets the level text template (w:lvlText). For counters this uses
// percent placeholders — "%1." → "1.", "%2)" → "a)" — while for a bullet it is
// the literal glyph, e.g. "".
func (l *ListLevel) SetText(text string) *ListLevel {
	l.lvl.LvlText = &oxml.CT_LvlText{Val: text}
	return l.modified()
}

// SetStart sets the value the level counts from (w:start).
func (l *ListLevel) SetStart(start int) *ListLevel {
	l.lvl.Start = &oxml.CT_DecimalNumber{Val: start}
	return l.modified()
}

// SetFont sets the font used to render the level text (w:rPr/w:rFonts), used for
// bullet glyphs such as Symbol or Wingdings.
func (l *ListLevel) SetFont(name string) *ListLevel {
	if l.lvl.RPr == nil {
		l.lvl.RPr = &oxml.CT_RPr{}
	}
	if l.lvl.RPr.RFonts == nil {
		l.lvl.RPr.RFonts = &oxml.CT_Fonts{}
	}
	l.lvl.RPr.RFonts.Ascii = name
	l.lvl.RPr.RFonts.HAnsi = name
	return l.modified()
}

// SetIndent sets the level's left indentation in points (w:pPr/w:ind@left).
func (l *ListLevel) SetIndent(points float64) *ListLevel {
	l.ensureInd().Left = pointsToTwips(points)
	return l.modified()
}

// SetHanging sets the level's hanging indent in points (w:pPr/w:ind@hanging).
func (l *ListLevel) SetHanging(points float64) *ListLevel {
	l.ensureInd().Hanging = pointsToTwips(points)
	return l.modified()
}

// SetAlignment sets how the level text is justified within the indent
// (w:lvlJc).
func (l *ListLevel) SetAlignment(align Alignment) *ListLevel {
	val := "left"
	switch align {
	case AlignmentCenter:
		val = "center"
	case AlignmentRight:
		val = "right"
	}
	l.lvl.LvlJc = &oxml.CT_Jc{Val: val}
	return l.modified()
}

// ensureInd is the funnel the indent setters go through, so flagging here
// covers them in one place rather than leaving each to remember (C406).
func (l *ListLevel) ensureInd() *oxml.CT_Ind {
	l.modified()
	if l.lvl.PPr == nil {
		l.lvl.PPr = &oxml.CT_PPr{}
	}
	if l.lvl.PPr.Ind == nil {
		l.lvl.PPr.Ind = &oxml.CT_Ind{}
	}
	return l.lvl.PPr.Ind
}

func (l *ListLevel) modified() *ListLevel {
	l.document.numberingModified = true
	return l
}
