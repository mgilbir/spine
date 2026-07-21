package xlsx

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// Comparison operators for a cellIs rule (NewCellIsRule). Between and NotBetween
// take two formula operands; the others take one.
const (
	CondOpEqual              = "equal"
	CondOpNotEqual           = "notEqual"
	CondOpGreaterThan        = "greaterThan"
	CondOpGreaterThanOrEqual = "greaterThanOrEqual"
	CondOpLessThan           = "lessThan"
	CondOpLessThanOrEqual    = "lessThanOrEqual"
	CondOpBetween            = "between"
	CondOpNotBetween         = "notBetween"
)

// Operators for the containsText rule family (NewTextRule).
const (
	CondTextContains    = "containsText"
	CondTextNotContains = "notContains"
	CondTextBeginsWith  = "beginsWith"
	CondTextEndsWith    = "endsWith"
)

// DifferentialStyle is the formatting a rule applies when it matches. It is
// stored in the styles part as a differential format (dxf) and referenced from
// the rule by index. Any subset of the fields may be set; nil fields are left
// unchanged by the rule. It mirrors the existing cell-style building blocks
// (FontStyle/FillStyle/BorderStyle) so callers reuse one vocabulary.
//
// Fill note: in a differential format Excel carries the visible solid-fill
// color in the pattern background, unlike a cell fill (which uses the
// foreground). NewCellIsRule and friends translate FillStyle accordingly, so a
// caller sets FillStyle.FgColor (or BgColor) to the highlight color they want.
type DifferentialStyle struct {
	Font   *FontStyle
	Fill   *FillStyle
	Border *BorderStyle
}

// ColorScalePoint is one gradient stop of a color-scale rule: a threshold
// paired with the color reached at that threshold.
type ColorScalePoint struct {
	// Type is the threshold kind: "min", "max", "num", "percent", "percentile"
	// or "formula".
	Type string
	// Value is the threshold value or formula; empty for "min"/"max".
	Value string
	// Color is the stop color as a 6- or 8-digit hex RGB string (e.g. "F8696B"
	// or "FFF8696B").
	Color string
}

// ConditionalRule is a single conditional-formatting rule produced by one of
// the New*Rule constructors and passed to Sheet.AddConditionalFormat. Its
// priority and (where applicable) differential-format index are assigned when
// it is added to a sheet, so a rule value must not be reused across ranges.
type ConditionalRule struct {
	rule oxml.CT_CfRule
	// dxf is the differential format to allocate for rules that reference one
	// (cellIs, text, top10, aboveAverage, duplicate/unique, expression,
	// timePeriod); nil for self-formatting rules (colorScale/dataBar/iconSet).
	dxf *oxml.CT_Dxf
	// textOp defers formula synthesis for the containsText family until the
	// range anchor is known at AddConditionalFormat time.
	textOp string
	text   string
	err    error
}

// StopIfTrue marks the rule so Excel stops evaluating lower-priority rules on
// the same cell once this one matches. It returns the rule for chaining.
func (r ConditionalRule) StopIfTrue() ConditionalRule {
	t := true
	r.rule.StopIfTrue = &t
	return r
}

// NewCellIsRule builds a cellIs rule that compares each cell against one or two
// formula operands with the given operator (a CondOp* constant) and applies
// style when it matches. Between/NotBetween need two formulas; the other
// operators need one. A formula operand is any Excel expression: a literal
// ("100"), a quoted string (`"done"`) or a reference ("$B$1").
func NewCellIsRule(operator string, style DifferentialStyle, formulas ...string) ConditionalRule {
	want := 1
	if operator == CondOpBetween || operator == CondOpNotBetween {
		want = 2
	}
	if len(formulas) != want {
		return ConditionalRule{err: fmt.Errorf("xlsx: cellIs operator %q needs %d formula operand(s), got %d", operator, want, len(formulas))}
	}
	dxf := diffStyleToDxf(style)
	return ConditionalRule{
		rule: oxml.CT_CfRule{
			Type:     "cellIs",
			Operator: operator,
			Formula:  append([]string(nil), formulas...),
		},
		dxf: &dxf,
	}
}

// NewExpressionRule builds an expression rule that applies style to every cell
// for which formula evaluates to TRUE. The formula is relative to the top-left
// cell of the range (e.g. "MOD(ROW(),2)=0" to shade even rows).
func NewExpressionRule(formula string, style DifferentialStyle) ConditionalRule {
	if strings.TrimSpace(formula) == "" {
		return ConditionalRule{err: fmt.Errorf("xlsx: expression rule needs a formula")}
	}
	dxf := diffStyleToDxf(style)
	return ConditionalRule{
		rule: oxml.CT_CfRule{
			Type:    "expression",
			Formula: []string{formula},
		},
		dxf: &dxf,
	}
}

// NewTextRule builds a rule from the containsText family (operator is a
// CondText* constant) that applies style to cells whose text contains, does not
// contain, begins with or ends with text. The matching formula Excel expects is
// synthesized automatically against the range's anchor cell.
func NewTextRule(operator, text string, style DifferentialStyle) ConditionalRule {
	switch operator {
	case CondTextContains, CondTextNotContains, CondTextBeginsWith, CondTextEndsWith:
	default:
		return ConditionalRule{err: fmt.Errorf("xlsx: unknown text operator %q", operator)}
	}
	if text == "" {
		return ConditionalRule{err: fmt.Errorf("xlsx: text rule needs a search string")}
	}
	dxf := diffStyleToDxf(style)
	// Excel sets both type and operator for the text family. The CfType and
	// operator enums agree except for "not contains", whose type is
	// "notContainsText" but whose operator is "notContains".
	var cfType, cfOp string
	switch operator {
	case CondTextContains:
		cfType, cfOp = "containsText", "containsText"
	case CondTextNotContains:
		cfType, cfOp = "notContainsText", "notContains"
	case CondTextBeginsWith:
		cfType, cfOp = "beginsWith", "beginsWith"
	case CondTextEndsWith:
		cfType, cfOp = "endsWith", "endsWith"
	}
	return ConditionalRule{
		rule: oxml.CT_CfRule{
			Type:     cfType,
			Operator: cfOp,
			Text:     text,
		},
		dxf:    &dxf,
		textOp: operator,
		text:   text,
	}
}

// NewDuplicateValuesRule builds a rule that applies style to values appearing
// more than once in the range.
func NewDuplicateValuesRule(style DifferentialStyle) ConditionalRule {
	dxf := diffStyleToDxf(style)
	return ConditionalRule{rule: oxml.CT_CfRule{Type: "duplicateValues"}, dxf: &dxf}
}

// NewUniqueValuesRule builds a rule that applies style to values appearing
// exactly once in the range.
func NewUniqueValuesRule(style DifferentialStyle) ConditionalRule {
	dxf := diffStyleToDxf(style)
	return ConditionalRule{rule: oxml.CT_CfRule{Type: "uniqueValues"}, dxf: &dxf}
}

// NewTop10Rule builds a top10 rule that applies style to the top (or, when
// bottom is true, the bottom) rank values in the range. When percent is true
// rank is a percentage (1-100) rather than a count.
func NewTop10Rule(rank uint32, bottom, percent bool, style DifferentialStyle) ConditionalRule {
	dxf := diffStyleToDxf(style)
	r := oxml.CT_CfRule{Type: "top10", Rank: &rank}
	if bottom {
		b := true
		r.Bottom = &b
	}
	if percent {
		p := true
		r.Percent = &p
	}
	return ConditionalRule{rule: r, dxf: &dxf}
}

// NewAboveAverageRule builds an aboveAverage rule that applies style to cells
// above (above=true) or below (above=false) the range average.
func NewAboveAverageRule(above bool, style DifferentialStyle) ConditionalRule {
	dxf := diffStyleToDxf(style)
	// aboveAverage="1" is Excel's default and is emitted explicitly here so the
	// direction round-trips unambiguously; "0" selects below-average.
	v := above
	return ConditionalRule{rule: oxml.CT_CfRule{Type: "aboveAverage", AboveAverage: &v}, dxf: &dxf}
}

// NewTimePeriodRule builds a timePeriod rule that applies style to date cells
// falling in period (e.g. "today", "yesterday", "last7Days", "thisMonth"). The
// comparison formula Excel expects is synthesized against the range anchor.
func NewTimePeriodRule(period string, style DifferentialStyle) ConditionalRule {
	if period == "" {
		return ConditionalRule{err: fmt.Errorf("xlsx: timePeriod rule needs a period")}
	}
	dxf := diffStyleToDxf(style)
	return ConditionalRule{
		rule:   oxml.CT_CfRule{Type: "timePeriod", TimePeriod: period},
		dxf:    &dxf,
		textOp: "timePeriod:" + period,
	}
}

// NewColorScaleRule builds a colorScale rule with two or three gradient stops
// (a 2-color or 3-color scale). It carries its own formatting, so it does not
// allocate a differential format.
func NewColorScaleRule(points ...ColorScalePoint) ConditionalRule {
	if len(points) != 2 && len(points) != 3 {
		return ConditionalRule{err: fmt.Errorf("xlsx: colorScale needs 2 or 3 points, got %d", len(points))}
	}
	cs := &oxml.CT_ColorScale{}
	for _, p := range points {
		cs.Cfvo = append(cs.Cfvo, oxml.CT_Cfvo{Type: p.Type, Val: p.Value})
	}
	// Per the schema all cfvo elements precede all color elements.
	for _, p := range points {
		cs.Color = append(cs.Color, oxml.CT_Color{Rgb: normalizeHexColor(p.Color)})
	}
	return ConditionalRule{rule: oxml.CT_CfRule{Type: "colorScale", ColorScale: cs}}
}

// NewDataBarRule builds a dataBar rule with the given bar color and lower/upper
// bounds. A bound with an empty Type defaults to "min" (low) / "max" (high). It
// carries its own formatting.
func NewDataBarRule(color string, low, high ConditionalValueObject) ConditionalRule {
	if low.Type == "" {
		low.Type = "min"
	}
	if high.Type == "" {
		high.Type = "max"
	}
	db := &oxml.CT_DataBar{
		Cfvo: []oxml.CT_Cfvo{
			{Type: low.Type, Val: low.Value},
			{Type: high.Type, Val: high.Value},
		},
	}
	if color != "" {
		db.Color = &oxml.CT_Color{Rgb: normalizeHexColor(color)}
	}
	return ConditionalRule{rule: oxml.CT_CfRule{Type: "dataBar", DataBar: db}}
}

// NewIconSetRule builds an iconSet rule using the named icon collection (e.g.
// "3TrafficLights1", "4Arrows", "5Rating"). When no thresholds are given, one
// percent threshold per icon is generated (evenly spaced from 0). It carries
// its own formatting.
func NewIconSetRule(iconSet string, thresholds ...ConditionalValueObject) ConditionalRule {
	if iconSet == "" {
		return ConditionalRule{err: fmt.Errorf("xlsx: iconSet rule needs an icon set name")}
	}
	is := &oxml.CT_IconSet{IconSet: iconSet}
	if len(thresholds) == 0 {
		n := iconSetCount(iconSet)
		for i := 0; i < n; i++ {
			is.Cfvo = append(is.Cfvo, oxml.CT_Cfvo{Type: "percent", Val: strconv.Itoa(i * 100 / n)})
		}
	} else {
		for _, t := range thresholds {
			is.Cfvo = append(is.Cfvo, oxml.CT_Cfvo{Type: t.Type, Val: t.Value})
		}
	}
	return ConditionalRule{rule: oxml.CT_CfRule{Type: "iconSet", IconSet: is}}
}

// iconSetCount reads the leading icon count of an icon-set name (e.g. "3" from
// "3TrafficLights1"), defaulting to 3 for an unrecognized name.
func iconSetCount(name string) int {
	if len(name) > 0 && name[0] >= '3' && name[0] <= '5' {
		return int(name[0] - '0')
	}
	return 3
}

// AddConditionalFormat adds a conditional-formatting block over cellRange (a
// single range like "B2:B10" or a space-separated list like "A1:A10 C1:C10")
// with the given rules, in the order supplied. Rules that apply a differential
// format allocate (and deduplicate) a dxf entry in the styles part; every rule
// is assigned a sheet-unique priority above any already present, so later calls
// layer on top of earlier ones. It returns an error if the range is invalid,
// no rules are given, or a rule was built with invalid parameters.
func (s *Sheet) AddConditionalFormat(cellRange string, rules ...ConditionalRule) error {
	if len(rules) == 0 {
		return fmt.Errorf("xlsx: AddConditionalFormat requires at least one rule")
	}
	sqref, anchor, err := normalizeSqref(cellRange)
	if err != nil {
		return err
	}
	for i := range rules {
		if rules[i].err != nil {
			return rules[i].err
		}
		if rules[i].dxf != nil && s.workbook == nil {
			return fmt.Errorf("xlsx: AddConditionalFormat: sheet has no workbook to hold the differential format")
		}
	}

	s.markDirty()
	s.ensureWorksheet()

	block := oxml.CT_ConditionalFormatting{Sqref: sqref}
	priority := s.nextConditionalPriority()
	for i := range rules {
		cr := &rules[i]
		rule := cr.rule
		rule.Formula = append([]string(nil), cr.rule.Formula...)
		if cr.textOp != "" {
			rule.Formula = []string{synthConditionalFormula(cr.textOp, cr.text, anchor)}
		}
		rule.Priority = int32(priority)
		priority++
		if cr.dxf != nil {
			id := s.workbook.allocateDxf(*cr.dxf)
			rule.DxfId = &id
		}
		block.CfRule = append(block.CfRule, rule)
	}
	s.ws().ConditionalFormatting = append(s.ws().ConditionalFormatting, block)
	s.ws().AppendConditionalFormattingOrder()
	return nil
}

// nextConditionalPriority returns one above the highest priority currently used
// by any rule on the sheet (1 when there are none), so newly added rules take
// precedence in the order they are added without colliding with existing ones.
func (s *Sheet) nextConditionalPriority() int {
	maxP := 0
	if s.ws() != nil {
		for i := range s.ws().ConditionalFormatting {
			for _, r := range s.ws().ConditionalFormatting[i].CfRule {
				if int(r.Priority) > maxP {
					maxP = int(r.Priority)
				}
			}
		}
	}
	return maxP + 1
}

// normalizeSqref parses a single range or a space-separated range list into a
// canonical sqref string and the anchor cell (top-left of the first range) used
// for synthesizing relative rule formulas.
func normalizeSqref(cellRange string) (sqref, anchor string, err error) {
	fields := strings.Fields(cellRange)
	if len(fields) == 0 {
		return "", "", ErrInvalidRange
	}
	parts := make([]string, len(fields))
	for i, f := range fields {
		rng, err := parseCellRangeRef(f)
		if err != nil {
			return "", "", err
		}
		parts[i] = rng.ref()
		if i == 0 {
			anchor = FormatCellRef(rng.minRow, rng.minCol)
		}
	}
	return strings.Join(parts, " "), anchor, nil
}

// synthConditionalFormula builds the <formula> Excel expects for rule kinds
// whose matching predicate is expressed relative to the range anchor: the
// containsText family and timePeriod.
func synthConditionalFormula(op, text, anchor string) string {
	if period, ok := strings.CutPrefix(op, "timePeriod:"); ok {
		return timePeriodFormula(period, anchor)
	}
	q := `"` + strings.ReplaceAll(text, `"`, `""`) + `"`
	switch op {
	case CondTextContains:
		return "NOT(ISERROR(SEARCH(" + q + "," + anchor + ")))"
	case CondTextNotContains:
		return "ISERROR(SEARCH(" + q + "," + anchor + "))"
	case CondTextBeginsWith:
		return "LEFT(" + anchor + ",LEN(" + q + "))=" + q
	case CondTextEndsWith:
		return "RIGHT(" + anchor + ",LEN(" + q + "))=" + q
	}
	return ""
}

// timePeriodFormula returns the comparison formula Excel writes for the common
// timePeriod values, anchored at cell. Unknown periods yield an empty formula
// (the timePeriod attribute alone still drives evaluation).
func timePeriodFormula(period, anchor string) string {
	switch period {
	case "today":
		return "FLOOR(" + anchor + ",1)=TODAY()"
	case "yesterday":
		return "FLOOR(" + anchor + ",1)=TODAY()-1"
	case "tomorrow":
		return "FLOOR(" + anchor + ",1)=TODAY()+1"
	case "last7Days":
		return "AND(TODAY()-FLOOR(" + anchor + ",1)>=0,TODAY()-FLOOR(" + anchor + ",1)<=6)"
	case "thisWeek":
		return "AND(TODAY()-ROUNDDOWN(" + anchor + ",0)<=WEEKDAY(TODAY())-1,ROUNDDOWN(" + anchor + ",0)-TODAY()<=7-WEEKDAY(TODAY()))"
	case "lastWeek":
		return "AND(TODAY()-ROUNDDOWN(" + anchor + ",0)>=(WEEKDAY(TODAY())),TODAY()-ROUNDDOWN(" + anchor + ",0)<(WEEKDAY(TODAY())+7))"
	case "nextWeek":
		return "AND(ROUNDDOWN(" + anchor + ",0)-TODAY()>(7-WEEKDAY(TODAY())),ROUNDDOWN(" + anchor + ",0)-TODAY()<(15-WEEKDAY(TODAY())))"
	case "thisMonth":
		return "AND(MONTH(" + anchor + ")=MONTH(TODAY()),YEAR(" + anchor + ")=YEAR(TODAY()))"
	case "lastMonth":
		return "AND(MONTH(" + anchor + ")=MONTH(EDATE(TODAY(),-1)),YEAR(" + anchor + ")=YEAR(EDATE(TODAY(),-1)))"
	case "nextMonth":
		return "AND(MONTH(" + anchor + ")=MONTH(EDATE(TODAY(),1)),YEAR(" + anchor + ")=YEAR(EDATE(TODAY(),1)))"
	}
	return ""
}

// --- Differential format (dxf) allocation and conversion --------------------

// allocateDxf returns the index of a dxf matching d in the workbook stylesheet,
// adding it (and marking styles dirty for regeneration) when absent. Reusing an
// existing entry keeps the styles part untouched.
func (w *Workbook) allocateDxf(d oxml.CT_Dxf) uint32 {
	if w.stylesheet == nil {
		w.stylesheet = defaultStylesheet()
		w.stylesDirty = true
	}
	if w.stylesheet.Dxfs == nil {
		w.stylesheet.Dxfs = &oxml.CT_Dxfs{}
	}
	for i := range w.stylesheet.Dxfs.Dxf {
		if dxfEqual(&w.stylesheet.Dxfs.Dxf[i], &d) {
			return uint32(i)
		}
	}
	w.stylesheet.Dxfs.Dxf = append(w.stylesheet.Dxfs.Dxf, d)
	idx := uint32(len(w.stylesheet.Dxfs.Dxf) - 1)
	count := uint32(len(w.stylesheet.Dxfs.Dxf))
	w.stylesheet.Dxfs.Count = &count
	w.stylesDirty = true
	return idx
}

// dxfStyle resolves a dxf index into a DifferentialStyle, or nil when the index
// is out of range or the dxf carries no font/fill/border.
func (w *Workbook) dxfStyle(idx uint32) *DifferentialStyle {
	if w.stylesheet == nil || w.stylesheet.Dxfs == nil || int(idx) >= len(w.stylesheet.Dxfs.Dxf) {
		return nil
	}
	return dxfToDiffStyle(&w.stylesheet.Dxfs.Dxf[idx])
}

// diffStyleToDxf converts a DifferentialStyle into a differential format record.
func diffStyleToDxf(ds DifferentialStyle) oxml.CT_Dxf {
	dxf := oxml.CT_Dxf{}
	if ds.Font != nil {
		f := fontStyleToOxml(ds.Font)
		dxf.Font = &f
	}
	if ds.Fill != nil {
		dxf.Fill = diffFillToOxml(ds.Fill)
	}
	if ds.Border != nil {
		b := borderStyleToOxml(ds.Border)
		dxf.Border = &b
	}
	return dxf
}

// diffFillToOxml builds a differential-format pattern fill. Excel stores the
// visible solid-fill color in the pattern background (bgColor), unlike a cell
// fill (fgColor), so the caller's fill color is mapped there.
func diffFillToOxml(fs *FillStyle) *oxml.CT_Fill {
	pf := &oxml.CT_PatternFill{}
	color := fs.FgColor
	if color == "" {
		color = fs.BgColor
	}
	if color != "" {
		pf.BgColor = &oxml.CT_Color{Rgb: normalizeHexColor(color)}
	}
	// A non-solid named pattern keeps its type; solid (the highlight default)
	// is left implicit, matching what Excel writes for a CF fill.
	if fs.Pattern != "" && fs.Pattern != "solid" {
		pf.PatternType = fs.Pattern
	}
	return &oxml.CT_Fill{PatternFill: pf}
}

// dxfToDiffStyle converts a differential format record back into a
// DifferentialStyle, reversing diffFillToOxml's fill mapping. It returns nil
// when the record has no font, fill or border.
func dxfToDiffStyle(dxf *oxml.CT_Dxf) *DifferentialStyle {
	ds := &DifferentialStyle{}
	if dxf.Font != nil {
		ds.Font = oxmlToFontStyle(dxf.Font)
	}
	if dxf.Fill != nil && dxf.Fill.PatternFill != nil {
		pf := dxf.Fill.PatternFill
		fs := &FillStyle{Pattern: pf.PatternType}
		switch {
		case pf.BgColor != nil && pf.BgColor.Rgb != "":
			fs.FgColor = stripAlphaFromRGB(pf.BgColor.Rgb)
		case pf.FgColor != nil && pf.FgColor.Rgb != "":
			fs.FgColor = stripAlphaFromRGB(pf.FgColor.Rgb)
		}
		if fs.Pattern != "" || fs.FgColor != "" {
			ds.Fill = fs
		}
	}
	if dxf.Border != nil {
		ds.Border = oxmlToBorderStyle(dxf.Border)
	}
	if ds.Font == nil && ds.Fill == nil && ds.Border == nil {
		return nil
	}
	return ds
}

// dxfEqual reports whether two differential formats are identical for
// deduplication.
func dxfEqual(a, b *oxml.CT_Dxf) bool {
	return fontPtrEqual(a.Font, b.Font) &&
		fillPtrEqual(a.Fill, b.Fill) &&
		borderPtrEqual(a.Border, b.Border) &&
		numFmtPtrEqual(a.NumFmt, b.NumFmt) &&
		cellAlignmentEqual(a.Alignment, b.Alignment) &&
		cellProtectionEqual(a.Protection, b.Protection)
}

func fontPtrEqual(a, b *oxml.CT_Font) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return fontEqual(a, b)
}

func fillPtrEqual(a, b *oxml.CT_Fill) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return fillEqual(a, b)
}

func borderPtrEqual(a, b *oxml.CT_Border) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return borderEqual(a, b)
}

func numFmtPtrEqual(a, b *oxml.CT_NumFmt) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return a.NumFmtId == b.NumFmtId && a.FormatCode == b.FormatCode
}
