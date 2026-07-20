package oxml

import (
	"encoding/xml"
	"strconv"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// Sparkline namespaces and the worksheet-extension URI Excel uses to carry the
// 2010 sparkline feature. Sparklines live in the worksheet extLst as an <ext>
// whose uri is SparklineExtURI; x14 is the extension element namespace and xm
// carries the data-range formulas / location references.
const (
	// NSX14 is the Microsoft 2009/9 SpreadsheetML extension namespace (x14).
	NSX14 = "http://schemas.microsoft.com/office/spreadsheetml/2009/9/main"
	// NSXM is the Microsoft 2006 Excel formula/reference namespace (xm).
	NSXM = "http://schemas.microsoft.com/office/excel/2006/main"
	// SparklineExtURI identifies the sparkline groups worksheet extension.
	SparklineExtURI = "{05C60535-1F16-4fd2-B633-F4F36F0B64E0}"
)

// Sparkline type values (ST_SparklineType). "stacked" is Excel's spelling for
// the win/loss sparkline.
const (
	SparklineTypeLine    = "line"
	SparklineTypeColumn  = "column"
	SparklineTypeStacked = "stacked"
)

// CT_SparklineGroups models x14:sparklineGroups, the child of the sparkline
// worksheet extension. It holds one or more sparkline groups.
type CT_SparklineGroups struct {
	Groups []CT_SparklineGroup
}

// CT_SparklineGroup models x14:sparklineGroup: a set of sparklines sharing a
// type and color scheme. Only fields Excel commonly emits are modeled; optional
// attributes and colors are pointers so absent ones are not re-emitted.
type CT_SparklineGroup struct {
	ManualMax  *float64
	ManualMin  *float64
	LineWeight *float64
	// Type is "line" (default, emitted as absent), "column" or "stacked".
	Type                string
	DateAxis            *bool
	DisplayEmptyCellsAs string
	Markers             *bool
	High                *bool
	Low                 *bool
	First               *bool
	Last                *bool
	Negative            *bool
	DisplayXAxis        *bool
	DisplayHidden       *bool
	MinAxisType         string
	MaxAxisType         string
	RightToLeft         *bool

	ColorSeries   *SparklineColor
	ColorNegative *SparklineColor
	ColorAxis     *SparklineColor
	ColorMarkers  *SparklineColor
	ColorFirst    *SparklineColor
	ColorLast     *SparklineColor
	ColorHigh     *SparklineColor
	ColorLow      *SparklineColor

	// F is the group-level data reference (xm:f), rarely used; per-sparkline
	// references live on each CT_Sparkline instead.
	F          string
	Sparklines []CT_Sparkline
}

// SparklineColor models an x14 color child (colorSeries, colorNegative, ...).
// It mirrors the SpreadsheetML CT_Color attribute set.
type SparklineColor struct {
	Auto    *bool
	Indexed *uint32
	Rgb     string
	Theme   *uint32
	Tint    *float64
}

// CT_Sparkline models x14:sparkline: one drawn sparkline mapping a source data
// range (xm:f) to the cell it is rendered in (xm:sqref).
type CT_Sparkline struct {
	F     string
	Sqref string
}

// ParseSparklineGroups parses the raw inner content of a sparkline worksheet
// extension (the bytes between <ext ...> and </ext>, i.e. an x14:sparklineGroups
// element) into a typed model. The x14 prefix is declared on the enclosing ext
// (outside raw), so parsing is lenient about the element namespace and keys on
// local names.
func ParseSparklineGroups(raw []byte) (*CT_SparklineGroups, error) {
	dec := xml.NewDecoder(strings.NewReader(string(raw)))
	sg := &CT_SparklineGroups{}
	for {
		tok, err := dec.Token()
		if err != nil {
			// io.EOF or a truncated fragment: return what was parsed.
			break
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local == "sparklineGroups" {
			if err := sg.decode(dec); err != nil {
				return nil, err
			}
			return sg, nil
		}
	}
	return sg, nil
}

// decode reads sparklineGroup children until the sparklineGroups end tag.
func (sg *CT_SparklineGroups) decode(dec *xml.Decoder) error {
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "sparklineGroup" {
				var g CT_SparklineGroup
				if err := g.decode(dec, t); err != nil {
					return err
				}
				sg.Groups = append(sg.Groups, g)
			} else if err := dec.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

// decode reads a single sparklineGroup element's attributes and children.
func (g *CT_SparklineGroup) decode(dec *xml.Decoder, start xml.StartElement) error {
	for _, a := range start.Attr {
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		switch a.Name.Local {
		case "manualMax":
			g.ManualMax = parseFloatPtr(a.Value)
		case "manualMin":
			g.ManualMin = parseFloatPtr(a.Value)
		case "lineWeight":
			g.LineWeight = parseFloatPtr(a.Value)
		case "type":
			g.Type = a.Value
		case "dateAxis":
			g.DateAxis = boolPtr(parseOnOff(a.Value))
		case "displayEmptyCellsAs":
			g.DisplayEmptyCellsAs = a.Value
		case "markers":
			g.Markers = boolPtr(parseOnOff(a.Value))
		case "high":
			g.High = boolPtr(parseOnOff(a.Value))
		case "low":
			g.Low = boolPtr(parseOnOff(a.Value))
		case "first":
			g.First = boolPtr(parseOnOff(a.Value))
		case "last":
			g.Last = boolPtr(parseOnOff(a.Value))
		case "negative":
			g.Negative = boolPtr(parseOnOff(a.Value))
		case "displayXAxis":
			g.DisplayXAxis = boolPtr(parseOnOff(a.Value))
		case "displayHidden":
			g.DisplayHidden = boolPtr(parseOnOff(a.Value))
		case "minAxisType":
			g.MinAxisType = a.Value
		case "maxAxisType":
			g.MaxAxisType = a.Value
		case "rightToLeft":
			g.RightToLeft = boolPtr(parseOnOff(a.Value))
		}
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "colorSeries":
				g.ColorSeries = parseSparklineColor(t)
				_ = dec.Skip()
			case "colorNegative":
				g.ColorNegative = parseSparklineColor(t)
				_ = dec.Skip()
			case "colorAxis":
				g.ColorAxis = parseSparklineColor(t)
				_ = dec.Skip()
			case "colorMarkers":
				g.ColorMarkers = parseSparklineColor(t)
				_ = dec.Skip()
			case "colorFirst":
				g.ColorFirst = parseSparklineColor(t)
				_ = dec.Skip()
			case "colorLast":
				g.ColorLast = parseSparklineColor(t)
				_ = dec.Skip()
			case "colorHigh":
				g.ColorHigh = parseSparklineColor(t)
				_ = dec.Skip()
			case "colorLow":
				g.ColorLow = parseSparklineColor(t)
				_ = dec.Skip()
			case "f":
				// Group-level xm:f (guard against the nested sparkline f).
				var s string
				if err := dec.DecodeElement(&s, &t); err != nil {
					return err
				}
				g.F = s
			case "sparklines":
				if err := g.decodeSparklines(dec); err != nil {
					return err
				}
			default:
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			return nil
		}
	}
}

// decodeSparklines reads the x14:sparklines list.
func (g *CT_SparklineGroup) decodeSparklines(dec *xml.Decoder) error {
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "sparkline" {
				sp, err := parseSparkline(dec)
				if err != nil {
					return err
				}
				g.Sparklines = append(g.Sparklines, sp)
			} else if err := dec.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

// parseSparkline reads one x14:sparkline (its xm:f data range and xm:sqref
// location cell).
func parseSparkline(dec *xml.Decoder) (CT_Sparkline, error) {
	var sp CT_Sparkline
	for {
		tok, err := dec.Token()
		if err != nil {
			return sp, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "f":
				var s string
				if err := dec.DecodeElement(&s, &t); err != nil {
					return sp, err
				}
				sp.F = s
			case "sqref":
				var s string
				if err := dec.DecodeElement(&s, &t); err != nil {
					return sp, err
				}
				sp.Sqref = s
			default:
				if err := dec.Skip(); err != nil {
					return sp, err
				}
			}
		case xml.EndElement:
			return sp, nil
		}
	}
}

// parseSparklineColor reads the CT_Color attribute set from a color element.
func parseSparklineColor(start xml.StartElement) *SparklineColor {
	c := &SparklineColor{}
	for _, a := range start.Attr {
		switch a.Name.Local {
		case "auto":
			c.Auto = boolPtr(parseOnOff(a.Value))
		case "indexed":
			c.Indexed = parseUintPtr(a.Value)
		case "rgb":
			c.Rgb = a.Value
		case "theme":
			c.Theme = parseUintPtr(a.Value)
		case "tint":
			c.Tint = parseFloatPtr(a.Value)
		}
	}
	return c
}

// Marshal serializes the sparkline groups into the raw inner content of a
// sparkline worksheet extension: an x14:sparklineGroups element that declares
// xmlns:xm and whose x14 prefix is bound by the enclosing <ext>. The returned
// bytes are stored as CT_Extension.RawContent.
func (sg *CT_SparklineGroups) Marshal() []byte {
	b := xmlb.NewBuilder()
	b.RegisterNamespace(NSX14, "x14")
	b.RegisterNamespace(NSXM, "xm")
	b.StartElement(NSX14, "sparklineGroups", xmlb.Attr{Name: "xmlns:xm", Value: NSXM})
	for i := range sg.Groups {
		sg.Groups[i].marshal(b)
	}
	b.EndElement(NSX14, "sparklineGroups")
	return b.Bytes()
}

// marshal writes a single sparklineGroup, its colors and its sparklines.
func (g *CT_SparklineGroup) marshal(b *xmlb.Builder) {
	var attrs []xmlb.Attr
	if g.ManualMax != nil {
		attrs = append(attrs, floatAttr("manualMax", *g.ManualMax))
	}
	if g.ManualMin != nil {
		attrs = append(attrs, floatAttr("manualMin", *g.ManualMin))
	}
	if g.LineWeight != nil {
		attrs = append(attrs, floatAttr("lineWeight", *g.LineWeight))
	}
	if g.Type != "" && g.Type != SparklineTypeLine {
		attrs = append(attrs, xmlb.StrAttr("type", g.Type))
	}
	if g.DateAxis != nil {
		attrs = append(attrs, xmlb.BoolAttr("dateAxis", *g.DateAxis))
	}
	if g.DisplayEmptyCellsAs != "" {
		attrs = append(attrs, xmlb.StrAttr("displayEmptyCellsAs", g.DisplayEmptyCellsAs))
	}
	if g.Markers != nil {
		attrs = append(attrs, xmlb.BoolAttr("markers", *g.Markers))
	}
	if g.High != nil {
		attrs = append(attrs, xmlb.BoolAttr("high", *g.High))
	}
	if g.Low != nil {
		attrs = append(attrs, xmlb.BoolAttr("low", *g.Low))
	}
	if g.First != nil {
		attrs = append(attrs, xmlb.BoolAttr("first", *g.First))
	}
	if g.Last != nil {
		attrs = append(attrs, xmlb.BoolAttr("last", *g.Last))
	}
	if g.Negative != nil {
		attrs = append(attrs, xmlb.BoolAttr("negative", *g.Negative))
	}
	if g.DisplayXAxis != nil {
		attrs = append(attrs, xmlb.BoolAttr("displayXAxis", *g.DisplayXAxis))
	}
	if g.DisplayHidden != nil {
		attrs = append(attrs, xmlb.BoolAttr("displayHidden", *g.DisplayHidden))
	}
	if g.MinAxisType != "" {
		attrs = append(attrs, xmlb.StrAttr("minAxisType", g.MinAxisType))
	}
	if g.MaxAxisType != "" {
		attrs = append(attrs, xmlb.StrAttr("maxAxisType", g.MaxAxisType))
	}
	if g.RightToLeft != nil {
		attrs = append(attrs, xmlb.BoolAttr("rightToLeft", *g.RightToLeft))
	}

	b.StartElement(NSX14, "sparklineGroup", attrs...)
	marshalSparklineColor(b, "colorSeries", g.ColorSeries)
	marshalSparklineColor(b, "colorNegative", g.ColorNegative)
	marshalSparklineColor(b, "colorAxis", g.ColorAxis)
	marshalSparklineColor(b, "colorMarkers", g.ColorMarkers)
	marshalSparklineColor(b, "colorFirst", g.ColorFirst)
	marshalSparklineColor(b, "colorLast", g.ColorLast)
	marshalSparklineColor(b, "colorHigh", g.ColorHigh)
	marshalSparklineColor(b, "colorLow", g.ColorLow)
	if g.F != "" {
		b.WriteElement(NSXM, "f", g.F)
	}
	b.StartElement(NSX14, "sparklines")
	for _, sp := range g.Sparklines {
		b.StartElement(NSX14, "sparkline")
		b.WriteElement(NSXM, "f", sp.F)
		b.WriteElement(NSXM, "sqref", sp.Sqref)
		b.EndElement(NSX14, "sparkline")
	}
	b.EndElement(NSX14, "sparklines")
	b.EndElement(NSX14, "sparklineGroup")
}

// marshalSparklineColor writes a color child (self-closing) when set.
func marshalSparklineColor(b *xmlb.Builder, localName string, c *SparklineColor) {
	if c == nil {
		return
	}
	var attrs []xmlb.Attr
	if c.Auto != nil {
		attrs = append(attrs, xmlb.BoolAttr("auto", *c.Auto))
	}
	if c.Indexed != nil {
		attrs = append(attrs, xmlb.UintAttr("indexed", *c.Indexed))
	}
	if c.Rgb != "" {
		attrs = append(attrs, xmlb.StrAttr("rgb", c.Rgb))
	}
	if c.Theme != nil {
		attrs = append(attrs, xmlb.UintAttr("theme", *c.Theme))
	}
	if c.Tint != nil {
		attrs = append(attrs, floatAttr("tint", *c.Tint))
	}
	b.EmptyElement(NSX14, localName, attrs...)
}

// --- small parse/format helpers --------------------------------------------

func floatAttr(name string, v float64) xmlb.Attr {
	return xmlb.Attr{Name: name, Value: strconv.FormatFloat(v, 'f', -1, 64)}
}

func boolPtr(v bool) *bool { return &v }

func parseFloatPtr(s string) *float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return nil
	}
	return &v
}

func parseUintPtr(s string) *uint32 {
	n, err := strconv.ParseUint(strings.TrimSpace(s), 10, 32)
	if err != nil {
		return nil
	}
	v := uint32(n)
	return &v
}
