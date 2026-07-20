package xlsx

import (
	"fmt"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// Sparkline group types accepted by AddSparklineGroup. Win/loss is Excel's
// "stacked" sparkline; the string constant hides that spelling from callers.
const (
	SparklineLine    = "line"
	SparklineColumn  = "column"
	SparklineWinLoss = "winloss"
)

// SparklineData maps one source data range to the single cell the sparkline is
// drawn in.
type SparklineData struct {
	// DataRange is the source range, e.g. "Sheet1!A1:D1" (a sheet-qualified
	// reference is recommended so the sparkline survives being moved).
	DataRange string
	// LocationCell is the single cell the sparkline is rendered in, e.g. "E1".
	LocationCell string
}

// SparklineOptions configures a new sparkline group added with
// AddSparklineGroup.
type SparklineOptions struct {
	// Type is SparklineLine (default), SparklineColumn or SparklineWinLoss.
	Type string
	// SeriesColor is the sparkline series color as a 6- or 8-digit hex RGB
	// string (e.g. "376092" or "FF376092"); empty leaves the color unset so
	// Excel applies its default.
	SeriesColor string
	// Data is the one or more (data range, location cell) mappings drawn by
	// the group; at least one is required.
	Data []SparklineData
}

// SparklineGroup is a read-only view of one x14:sparklineGroup on a sheet,
// returned by Sheet.Sparklines.
type SparklineGroup struct {
	g *oxml.CT_SparklineGroup
}

// Sparkline is one (data range, location cell) mapping within a group.
type Sparkline struct {
	// DataRange is the source data reference (xm:f).
	DataRange string
	// LocationCell is the cell the sparkline is drawn in (xm:sqref).
	LocationCell string
}

// Type returns the group's sparkline type: SparklineLine, SparklineColumn or
// SparklineWinLoss.
func (g *SparklineGroup) Type() string {
	switch g.g.Type {
	case oxml.SparklineTypeColumn:
		return SparklineColumn
	case oxml.SparklineTypeStacked:
		return SparklineWinLoss
	default:
		return SparklineLine
	}
}

// SeriesColor returns the group's series color as a hex RGB string (as stored,
// which may be 8-digit ARGB), or "" when the color is theme-based or unset.
func (g *SparklineGroup) SeriesColor() string {
	if g.g.ColorSeries == nil {
		return ""
	}
	return g.g.ColorSeries.Rgb
}

// Sparklines returns the group's (data range, location cell) mappings.
func (g *SparklineGroup) Sparklines() []Sparkline {
	out := make([]Sparkline, 0, len(g.g.Sparklines))
	for _, sp := range g.g.Sparklines {
		out = append(out, Sparkline{DataRange: sp.F, LocationCell: sp.Sqref})
	}
	return out
}

// Sparklines returns the sparkline groups defined on the sheet (read from the
// worksheet extension list), or nil when the sheet has none. The returned
// groups are a read-only snapshot; modifying them does not affect the workbook.
func (s *Sheet) Sparklines() []*SparklineGroup {
	if s.worksheet == nil || s.worksheet.ExtLst == nil {
		return nil
	}
	ext := findSparklineExt(s.worksheet.ExtLst)
	if ext == nil {
		return nil
	}
	sg, err := oxml.ParseSparklineGroups(ext.RawContent)
	if err != nil || sg == nil {
		return nil
	}
	out := make([]*SparklineGroup, 0, len(sg.Groups))
	for i := range sg.Groups {
		out = append(out, &SparklineGroup{g: &sg.Groups[i]})
	}
	return out
}

// AddSparklineGroup adds a sparkline group to the sheet's worksheet extension
// list. Type defaults to SparklineLine; at least one (data range, location
// cell) mapping is required. When the sheet already carries sparkline groups,
// the new group is appended to the existing extension. It returns a read-only
// view of the group just added.
func (s *Sheet) AddSparklineGroup(opts SparklineOptions) (*SparklineGroup, error) {
	if len(opts.Data) == 0 {
		return nil, fmt.Errorf("xlsx: AddSparklineGroup requires at least one data mapping")
	}
	typ, err := sparklineType(opts.Type)
	if err != nil {
		return nil, err
	}
	group := oxml.CT_SparklineGroup{Type: typ}
	if opts.SeriesColor != "" {
		group.ColorSeries = &oxml.SparklineColor{Rgb: normalizeHexColor(opts.SeriesColor)}
	}
	for _, d := range opts.Data {
		if strings.TrimSpace(d.DataRange) == "" || strings.TrimSpace(d.LocationCell) == "" {
			return nil, fmt.Errorf("xlsx: AddSparklineGroup: each mapping needs a data range and a location cell")
		}
		group.Sparklines = append(group.Sparklines, oxml.CT_Sparkline{
			F:     d.DataRange,
			Sqref: d.LocationCell,
		})
	}

	s.markDirty()
	s.ensureWorksheet()

	if s.worksheet.ExtLst == nil {
		s.worksheet.ExtLst = &oxml.CT_ExtensionList{}
	}
	s.worksheet.EnsureChildOrder("extLst")

	ext := findSparklineExt(s.worksheet.ExtLst)
	var groups *oxml.CT_SparklineGroups
	if ext == nil {
		// No sparkline extension yet: create one carrying the x14 declaration.
		s.worksheet.ExtLst.Ext = append(s.worksheet.ExtLst.Ext, oxml.CT_Extension{
			URI:           oxml.SparklineExtURI,
			InlineNSDecls: []xmlb.NSDecl{{Prefix: "x14", URI: oxml.NSX14}},
		})
		ext = &s.worksheet.ExtLst.Ext[len(s.worksheet.ExtLst.Ext)-1]
		groups = &oxml.CT_SparklineGroups{}
	} else {
		// Merge into the existing sparkline groups, preserving prior groups.
		parsed, err := oxml.ParseSparklineGroups(ext.RawContent)
		if err != nil {
			return nil, fmt.Errorf("xlsx: AddSparklineGroup: parse existing sparklines: %w", err)
		}
		groups = parsed
	}

	groups.Groups = append(groups.Groups, group)
	ext.RawContent = groups.Marshal()

	return &SparklineGroup{g: &groups.Groups[len(groups.Groups)-1]}, nil
}

// findSparklineExt returns the extension carrying sparkline groups, or nil.
func findSparklineExt(extLst *oxml.CT_ExtensionList) *oxml.CT_Extension {
	if extLst == nil {
		return nil
	}
	for i := range extLst.Ext {
		if extLst.Ext[i].URI == oxml.SparklineExtURI {
			return &extLst.Ext[i]
		}
	}
	return nil
}

// sparklineType maps a public type string to the ST_SparklineType value Excel
// writes, defaulting to line.
func sparklineType(t string) (string, error) {
	switch t {
	case "", SparklineLine:
		return oxml.SparklineTypeLine, nil
	case SparklineColumn:
		return oxml.SparklineTypeColumn, nil
	case SparklineWinLoss:
		return oxml.SparklineTypeStacked, nil
	default:
		return "", fmt.Errorf("xlsx: unknown sparkline type %q (want line, column or winloss)", t)
	}
}
