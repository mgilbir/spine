package xlsx

import (
	"strings"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// ConditionalFormat is a read-only view of one <conditionalFormatting> block: a
// set of cell ranges and the rules Excel evaluates against them. To create
// conditional formats use Sheet.AddConditionalFormat with the New*Rule
// constructors. A block opened from a file round-trips byte-for-byte when
// unmodified.
type ConditionalFormat struct {
	// SqRef is the raw space-separated range list the block applies to (e.g.
	// "A1:A10 C1:C10").
	SqRef string
	// Ranges is SqRef split into individual range references.
	Ranges []string
	// Rules are the block's rules in document order.
	Rules []*ConditionalFormatRule
}

// ConditionalFormatRule is a read-only view of one <cfRule>. The set of
// populated fields depends on Type: cellIs uses Operator + Formulas; expression
// uses a single Formula; containsText and friends use Text; timePeriod rules use
// TimePeriod; top10 uses Rank/Percent/Bottom; colorScale, dataBar and iconSet
// carry their respective sub-view.
type ConditionalFormatRule struct {
	// Type is the rule kind: "cellIs", "expression", "colorScale", "dataBar",
	// "iconSet", "top10", "aboveAverage", "duplicateValues", "uniqueValues",
	// "containsText", "timePeriod", etc.
	Type string
	// Operator is the comparison for cellIs/text rules (e.g. "between",
	// "greaterThan", "containsText"); empty when not applicable.
	Operator string
	// Priority orders overlapping rules (lower wins).
	Priority int
	// Formulas are the rule's <formula> operands, in order.
	Formulas []string
	// Text is the search text for containsText/beginsWith/endsWith rules.
	Text string
	// TimePeriod is the period for timePeriod rules (e.g. "today", "last7Days").
	TimePeriod string
	// StopIfTrue reports whether evaluation stops when this rule matches.
	StopIfTrue bool
	// DxfId indexes the differential format (dxf) applied when the rule matches,
	// or nil for rules that carry their own formatting (colorScale/dataBar/iconSet).
	DxfId *uint32
	// Rank is the N of a top10 rule (top/bottom N or N percent).
	Rank *uint32
	// Percent reports whether a top10 rule's Rank is a percentage.
	Percent bool
	// Bottom reports whether a top10 rule selects the bottom rather than the top.
	Bottom bool
	// AboveAverage, when non-nil, is an aboveAverage rule's direction: true for
	// above the average, false for below.
	AboveAverage *bool
	// ColorScale, DataBar and IconSet are populated for the corresponding rule
	// types and nil otherwise.
	ColorScale *ConditionalColorScale
	DataBar    *ConditionalDataBar
	IconSet    *ConditionalIconSet
	// sheet is the owning sheet, used by DifferentialFormat to resolve DxfId
	// against the workbook stylesheet. It is nil for rules built by hand.
	sheet *Sheet
}

// DifferentialFormat returns the resolved differential format (fill/font
// color/border) a rule applies when it matches, looked up from the workbook
// stylesheet via the rule's DxfId. It returns nil for rules that carry their
// own formatting (colorScale/dataBar/iconSet), for rules without a DxfId, or
// when the referenced dxf is absent or empty.
func (r *ConditionalFormatRule) DifferentialFormat() *DifferentialStyle {
	if r == nil || r.DxfId == nil || r.sheet == nil || r.sheet.workbook == nil {
		return nil
	}
	return r.sheet.workbook.dxfStyle(*r.DxfId)
}

// ConditionalValueObject is a conditional-format value object (<cfvo>): a
// threshold that anchors a colorScale stop, dataBar bound or iconSet band.
type ConditionalValueObject struct {
	// Type is the threshold kind: "min", "max", "num", "percent", "percentile",
	// "formula".
	Type string
	// Value is the threshold value or formula (empty for min/max).
	Value string
}

// ConditionalColorScale is the read view of a colorScale rule: paired value
// objects and RGB colors defining the gradient.
type ConditionalColorScale struct {
	Values []ConditionalValueObject
	// Colors are the gradient stops as hex RGB (e.g. "FFF8696B"); an entry is
	// empty when the stop uses a theme or indexed color instead.
	Colors []string
}

// ConditionalDataBar is the read view of a dataBar rule.
type ConditionalDataBar struct {
	Values []ConditionalValueObject
	// Color is the bar fill as hex RGB, or empty for a theme/indexed color.
	Color string
	// ShowValue reports whether the cell value is shown alongside the bar
	// (Excel's default is true; nil means the attribute was absent).
	ShowValue *bool
}

// ConditionalIconSet is the read view of an iconSet rule.
type ConditionalIconSet struct {
	// IconSet names the icon collection (e.g. "3TrafficLights1", "5Arrows").
	IconSet string
	Values  []ConditionalValueObject
	// ShowValue and Reverse mirror the iconSet attributes (nil when absent).
	ShowValue *bool
	Reverse   *bool
}

// ConditionalFormats returns the sheet's conditional-formatting blocks, in
// document order. The returned slice is nil when the sheet has none. To create
// blocks use Sheet.AddConditionalFormat.
func (s *Sheet) ConditionalFormats() []*ConditionalFormat {
	if s.worksheet == nil || len(s.worksheet.ConditionalFormatting) == 0 {
		return nil
	}
	out := make([]*ConditionalFormat, 0, len(s.worksheet.ConditionalFormatting))
	for i := range s.worksheet.ConditionalFormatting {
		cf := &s.worksheet.ConditionalFormatting[i]
		out = append(out, &ConditionalFormat{
			SqRef:  cf.Sqref,
			Ranges: strings.Fields(cf.Sqref),
			Rules:  conditionalRulesFromModel(s, cf.CfRule),
		})
	}
	return out
}

func conditionalRulesFromModel(s *Sheet, rules []oxml.CT_CfRule) []*ConditionalFormatRule {
	if len(rules) == 0 {
		return nil
	}
	out := make([]*ConditionalFormatRule, 0, len(rules))
	for i := range rules {
		r := &rules[i]
		rule := &ConditionalFormatRule{
			Type:         r.Type,
			Operator:     r.Operator,
			Priority:     int(r.Priority),
			Formulas:     append([]string(nil), r.Formula...),
			Text:         r.Text,
			TimePeriod:   r.TimePeriod,
			StopIfTrue:   r.StopIfTrue != nil && *r.StopIfTrue,
			DxfId:        r.DxfId,
			Rank:         r.Rank,
			Percent:      r.Percent != nil && *r.Percent,
			Bottom:       r.Bottom != nil && *r.Bottom,
			AboveAverage: r.AboveAverage,
			sheet:        s,
		}
		if r.ColorScale != nil {
			rule.ColorScale = &ConditionalColorScale{
				Values: cfvosFromModel(r.ColorScale.Cfvo),
				Colors: colorsToHex(r.ColorScale.Color),
			}
		}
		if r.DataBar != nil {
			rule.DataBar = &ConditionalDataBar{
				Values:    cfvosFromModel(r.DataBar.Cfvo),
				Color:     colorToHex(r.DataBar.Color),
				ShowValue: r.DataBar.ShowValue,
			}
		}
		if r.IconSet != nil {
			rule.IconSet = &ConditionalIconSet{
				IconSet:   r.IconSet.IconSet,
				Values:    cfvosFromModel(r.IconSet.Cfvo),
				ShowValue: r.IconSet.ShowValue,
				Reverse:   r.IconSet.Reverse,
			}
		}
		out = append(out, rule)
	}
	return out
}

func cfvosFromModel(cfvos []oxml.CT_Cfvo) []ConditionalValueObject {
	if len(cfvos) == 0 {
		return nil
	}
	out := make([]ConditionalValueObject, len(cfvos))
	for i := range cfvos {
		out[i] = ConditionalValueObject{Type: cfvos[i].Type, Value: cfvos[i].Val}
	}
	return out
}

func colorsToHex(colors []oxml.CT_Color) []string {
	if len(colors) == 0 {
		return nil
	}
	out := make([]string, len(colors))
	for i := range colors {
		out[i] = colors[i].Rgb
	}
	return out
}

func colorToHex(c *oxml.CT_Color) string {
	if c == nil {
		return ""
	}
	return c.Rgb
}
