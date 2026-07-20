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

// SparklineGroup is a live handle on one x14:sparklineGroup on a sheet, returned
// by Sheet.Sparklines and Sheet.AddSparklineGroup. Its setters and Delete write
// through to the workbook so a subsequent save persists them. Handles obtained
// from one Sparklines call share the sheet's parsed model; deleting a group
// invalidates the other handles from that call (their positions shift), so
// re-fetch with Sparklines after a Delete.
type SparklineGroup struct {
	sheet *Sheet
	g     *oxml.CT_SparklineGroup
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

// setColor updates the pointed-to color slot: an empty hex clears it (Excel
// falls back to its default), otherwise it sets an rgb color. It flushes the
// change through to the workbook.
func (g *SparklineGroup) setColor(slot **oxml.SparklineColor, hex string) {
	if strings.TrimSpace(hex) == "" {
		*slot = nil
	} else {
		*slot = &oxml.SparklineColor{Rgb: normalizeHexColor(hex)}
	}
	if g.sheet != nil {
		g.sheet.flushSparklines()
	}
}

// SetSeriesColor sets the group's series color (empty clears it).
func (g *SparklineGroup) SetSeriesColor(hex string) { g.setColor(&g.g.ColorSeries, hex) }

// SetNegativeColor sets the color of negative points (win/loss and column
// sparklines). Empty clears it.
func (g *SparklineGroup) SetNegativeColor(hex string) { g.setColor(&g.g.ColorNegative, hex) }

// SetAxisColor sets the horizontal-axis color. Empty clears it.
func (g *SparklineGroup) SetAxisColor(hex string) { g.setColor(&g.g.ColorAxis, hex) }

// SetMarkersColor sets the color of the point markers (line sparklines). Empty
// clears it.
func (g *SparklineGroup) SetMarkersColor(hex string) { g.setColor(&g.g.ColorMarkers, hex) }

// SetFirstColor sets the color of the first point. Empty clears it.
func (g *SparklineGroup) SetFirstColor(hex string) { g.setColor(&g.g.ColorFirst, hex) }

// SetLastColor sets the color of the last point. Empty clears it.
func (g *SparklineGroup) SetLastColor(hex string) { g.setColor(&g.g.ColorLast, hex) }

// SetHighColor sets the color of the highest point. Empty clears it.
func (g *SparklineGroup) SetHighColor(hex string) { g.setColor(&g.g.ColorHigh, hex) }

// SetLowColor sets the color of the lowest point. Empty clears it.
func (g *SparklineGroup) SetLowColor(hex string) { g.setColor(&g.g.ColorLow, hex) }

// setBool updates a pointed-to optional boolean slot and flushes the change.
func (g *SparklineGroup) setBool(slot **bool, v bool) {
	b := v
	*slot = &b
	if g.sheet != nil {
		g.sheet.flushSparklines()
	}
}

// SetMarkers toggles point markers on the group's sparklines (line type).
func (g *SparklineGroup) SetMarkers(on bool) { g.setBool(&g.g.Markers, on) }

// SetHigh toggles highlighting of the highest point.
func (g *SparklineGroup) SetHigh(on bool) { g.setBool(&g.g.High, on) }

// SetLow toggles highlighting of the lowest point.
func (g *SparklineGroup) SetLow(on bool) { g.setBool(&g.g.Low, on) }

// SetFirst toggles highlighting of the first point.
func (g *SparklineGroup) SetFirst(on bool) { g.setBool(&g.g.First, on) }

// SetLast toggles highlighting of the last point.
func (g *SparklineGroup) SetLast(on bool) { g.setBool(&g.g.Last, on) }

// SetNegative toggles highlighting of negative points.
func (g *SparklineGroup) SetNegative(on bool) { g.setBool(&g.g.Negative, on) }

// Markers reports whether point markers are enabled (a line-sparkline group
// with markers turned on). It is false when the flag is unset.
func (g *SparklineGroup) Markers() bool { return g.g.Markers != nil && *g.g.Markers }

// Delete removes this sparkline group from its sheet, writing the change through
// to the workbook. When it was the sheet's only group the sparkline extension is
// removed entirely. Other handles obtained from the same Sparklines call are
// invalidated by the removal; re-fetch with Sparklines if more edits are needed.
func (g *SparklineGroup) Delete() {
	if g.sheet == nil {
		return
	}
	sg := g.sheet.sparklineGroups()
	for i := range sg.Groups {
		if &sg.Groups[i] == g.g {
			sg.Groups = append(sg.Groups[:i], sg.Groups[i+1:]...)
			g.sheet.flushSparklines()
			g.sheet = nil
			return
		}
	}
}

// Sparklines returns the sparkline groups defined on the sheet (read from the
// worksheet extension list), or nil when the sheet has none. The returned groups
// are live handles: their setters and Delete write through to the workbook.
func (s *Sheet) Sparklines() []*SparklineGroup {
	sg := s.sparklineGroups()
	if sg == nil || len(sg.Groups) == 0 {
		return nil
	}
	out := make([]*SparklineGroup, 0, len(sg.Groups))
	for i := range sg.Groups {
		out = append(out, &SparklineGroup{sheet: s, g: &sg.Groups[i]})
	}
	return out
}

// sparklineGroups returns the sheet's parsed sparkline-groups model, loading it
// lazily from the worksheet extension list on first use and caching it so every
// handle mutates one shared model. It never returns nil.
func (s *Sheet) sparklineGroups() *oxml.CT_SparklineGroups {
	if s.sparklineCache != nil {
		return s.sparklineCache
	}
	if s.worksheet != nil && s.worksheet.ExtLst != nil {
		if ext := findSparklineExt(s.worksheet.ExtLst); ext != nil {
			if sg, err := oxml.ParseSparklineGroups(ext.RawContent); err == nil && sg != nil {
				s.sparklineCache = sg
				return sg
			}
		}
	}
	s.sparklineCache = &oxml.CT_SparklineGroups{}
	return s.sparklineCache
}

// flushSparklines writes the cached sparkline model back into the worksheet
// extension list and marks the sheet dirty. When the model is empty it removes
// the sparkline extension (and the extLst if that empties it) so no stub is
// left behind; otherwise it creates the extension (with the x14 declaration) if
// absent and refreshes its raw content.
func (s *Sheet) flushSparklines() {
	sg := s.sparklineGroups()
	s.markDirty()
	s.ensureWorksheet()

	if len(sg.Groups) == 0 {
		if s.worksheet.ExtLst == nil {
			return
		}
		kept := s.worksheet.ExtLst.Ext[:0]
		for _, e := range s.worksheet.ExtLst.Ext {
			if e.URI != oxml.SparklineExtURI {
				kept = append(kept, e)
			}
		}
		s.worksheet.ExtLst.Ext = kept
		if len(s.worksheet.ExtLst.Ext) == 0 {
			s.worksheet.ExtLst = nil
		}
		return
	}

	if s.worksheet.ExtLst == nil {
		s.worksheet.ExtLst = &oxml.CT_ExtensionList{}
	}
	s.worksheet.EnsureChildOrder("extLst")
	ext := findSparklineExt(s.worksheet.ExtLst)
	if ext == nil {
		s.worksheet.ExtLst.Ext = append(s.worksheet.ExtLst.Ext, oxml.CT_Extension{
			URI:           oxml.SparklineExtURI,
			InlineNSDecls: []xmlb.NSDecl{{Prefix: "x14", URI: oxml.NSX14}},
		})
		ext = &s.worksheet.ExtLst.Ext[len(s.worksheet.ExtLst.Ext)-1]
	}
	ext.RawContent = sg.Marshal()
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

	s.ensureWorksheet()
	groups := s.sparklineGroups()
	groups.Groups = append(groups.Groups, group)
	s.flushSparklines()

	return &SparklineGroup{sheet: s, g: &groups.Groups[len(groups.Groups)-1]}, nil
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
