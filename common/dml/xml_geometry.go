// This file provides DrawingML XML geometry types from dml-main.xsd.

package dml

import (
	"encoding/xml"
	"fmt"
	"strconv"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// PrstGeom represents CT_PresetGeometry2D (a:prstGeom)
type PrstGeom struct {
	Prst  string `xml:"prst,attr"`
	AvLst *AvLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/main avLst,omitempty"`
}

// CustGeom represents CT_CustomGeometry2D (a:custGeom)
type CustGeom struct {
	AvLst   *AvLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main avLst,omitempty"`
	GdLst   *GdLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gdLst,omitempty"`
	AhLst   *AhLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main ahLst,omitempty"`
	CxnLst  *CxnLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main cxnLst,omitempty"`
	RectXML *RectXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main rect,omitempty"`
	PathLst *PathLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pathLst,omitempty"`
}

// AvLst represents CT_GeomGuideList (a:avLst)
type AvLst struct {
	Gd []*Gd `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gd,omitempty"`
}

// GdLst represents CT_GeomGuideList (a:gdLst)
type GdLst struct {
	Gd []*Gd `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gd,omitempty"`
}

// Gd represents CT_GeomGuide (a:gd)
type Gd struct {
	Name string `xml:"name,attr"`
	Fmla string `xml:"fmla,attr"`
}

// PathLst represents CT_Path2DList (a:pathLst)
type PathLst struct {
	Path []*PathXML2D `xml:"http://schemas.openxmlformats.org/drawingml/2006/main path,omitempty"`
}

// pathCmdKind identifies a path command type.
type pathCmdKind int

const (
	pathCmdMoveTo pathCmdKind = iota
	pathCmdLnTo
	pathCmdArcTo
	pathCmdQuadBezTo
	pathCmdCubicBezTo
	pathCmdClose
)

// pathCmdRef references a path command by kind and index.
type pathCmdRef struct {
	kind  pathCmdKind
	index int
}

// PathXML2D represents CT_Path2D (a:path)
// Uses custom unmarshal/marshal to preserve interleaved command ordering
// (per XSD: xs:choice maxOccurs="unbounded").
type PathXML2D struct {
	W    int64  `xml:"w,attr,omitempty"`
	H    int64  `xml:"h,attr,omitempty"`
	Fill string `xml:"fill,attr,omitempty"`
	// Stroke and ExtrusionOk default to true when absent, so they are pointers:
	// nil means "unspecified" and an explicit false must be emitted as "0"
	// rather than omitted (which readers treat as true).
	Stroke      *bool            `xml:"-"`
	ExtrusionOk *bool            `xml:"-"`
	MoveTo      []*MoveToXML     `xml:"-"`
	LnTo        []*LnToXML       `xml:"-"`
	ArcTo       []*ArcToXML      `xml:"-"`
	QuadBezTo   []*QuadBezToXML  `xml:"-"`
	CubicBezTo  []*CubicBezToXML `xml:"-"`
	Close       []*CloseXML      `xml:"-"`
	cmdOrder    []pathCmdRef
}

// boolAttrValue formats a boolean as the canonical OOXML attribute value.
func boolAttrValue(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// UnmarshalXML implements custom unmarshaling for PathXML2D to preserve command order.
func (p *PathXML2D) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	// Parse attributes
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "w":
			n, err := strconv.ParseInt(attr.Value, 10, 64)
			if err != nil {
				return fmt.Errorf("dml: a:path w attribute %q: %w", attr.Value, err)
			}
			p.W = n
		case "h":
			n, err := strconv.ParseInt(attr.Value, 10, 64)
			if err != nil {
				return fmt.Errorf("dml: a:path h attribute %q: %w", attr.Value, err)
			}
			p.H = n
		case "fill":
			p.Fill = attr.Value
		case "stroke":
			v := attr.Value == "1" || attr.Value == "true"
			p.Stroke = &v
		case "extrusionOk":
			v := attr.Value == "1" || attr.Value == "true"
			p.ExtrusionOk = &v
		}
	}

	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "moveTo":
				v := &MoveToXML{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				p.cmdOrder = append(p.cmdOrder, pathCmdRef{pathCmdMoveTo, len(p.MoveTo)})
				p.MoveTo = append(p.MoveTo, v)
			case "lnTo":
				v := &LnToXML{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				p.cmdOrder = append(p.cmdOrder, pathCmdRef{pathCmdLnTo, len(p.LnTo)})
				p.LnTo = append(p.LnTo, v)
			case "arcTo":
				v := &ArcToXML{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				p.cmdOrder = append(p.cmdOrder, pathCmdRef{pathCmdArcTo, len(p.ArcTo)})
				p.ArcTo = append(p.ArcTo, v)
			case "quadBezTo":
				v := &QuadBezToXML{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				p.cmdOrder = append(p.cmdOrder, pathCmdRef{pathCmdQuadBezTo, len(p.QuadBezTo)})
				p.QuadBezTo = append(p.QuadBezTo, v)
			case "cubicBezTo":
				v := &CubicBezToXML{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				p.cmdOrder = append(p.cmdOrder, pathCmdRef{pathCmdCubicBezTo, len(p.CubicBezTo)})
				p.CubicBezTo = append(p.CubicBezTo, v)
			case "close":
				v := &CloseXML{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				p.cmdOrder = append(p.cmdOrder, pathCmdRef{pathCmdClose, len(p.Close)})
				p.Close = append(p.Close, v)
			default:
				if err := d.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			return nil
		}
	}
}

// MarshalToBuilder implements xmlb.BuilderMarshaler to preserve command order.
func (p *PathXML2D) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if p.W != 0 {
		attrs = append(attrs, xmlb.IntAttr("w", p.W))
	}
	if p.H != 0 {
		attrs = append(attrs, xmlb.IntAttr("h", p.H))
	}
	if p.Fill != "" {
		attrs = append(attrs, xmlb.Attr{Name: "fill", Value: p.Fill})
	}
	if p.Stroke != nil {
		attrs = append(attrs, xmlb.Attr{Name: "stroke", Value: boolAttrValue(*p.Stroke)})
	}
	if p.ExtrusionOk != nil {
		attrs = append(attrs, xmlb.Attr{Name: "extrusionOk", Value: boolAttrValue(*p.ExtrusionOk)})
	}
	b.StartElement(ns, localName, attrs...)

	if len(p.cmdOrder) > 0 {
		for _, ref := range p.cmdOrder {
			switch ref.kind {
			case pathCmdMoveTo:
				if ref.index < len(p.MoveTo) {
					b.MarshalElement(ns, "moveTo", p.MoveTo[ref.index])
				}
			case pathCmdLnTo:
				if ref.index < len(p.LnTo) {
					b.MarshalElement(ns, "lnTo", p.LnTo[ref.index])
				}
			case pathCmdArcTo:
				if ref.index < len(p.ArcTo) {
					b.MarshalElement(ns, "arcTo", p.ArcTo[ref.index])
				}
			case pathCmdQuadBezTo:
				if ref.index < len(p.QuadBezTo) {
					b.MarshalElement(ns, "quadBezTo", p.QuadBezTo[ref.index])
				}
			case pathCmdCubicBezTo:
				if ref.index < len(p.CubicBezTo) {
					b.MarshalElement(ns, "cubicBezTo", p.CubicBezTo[ref.index])
				}
			case pathCmdClose:
				if ref.index < len(p.Close) {
					b.MarshalElement(ns, "close", p.Close[ref.index])
				}
			}
		}
	} else {
		for _, v := range p.MoveTo {
			b.MarshalElement(ns, "moveTo", v)
		}
		for _, v := range p.LnTo {
			b.MarshalElement(ns, "lnTo", v)
		}
		for _, v := range p.ArcTo {
			b.MarshalElement(ns, "arcTo", v)
		}
		for _, v := range p.QuadBezTo {
			b.MarshalElement(ns, "quadBezTo", v)
		}
		for _, v := range p.CubicBezTo {
			b.MarshalElement(ns, "cubicBezTo", v)
		}
		for _, v := range p.Close {
			b.MarshalElement(ns, "close", v)
		}
	}

	b.EndElement(ns, localName)
}

// MarshalXML implements xml.Marshaler for PathXML2D, ensuring path commands
// are serialized even though they use xml:"-" struct tags.
func (p *PathXML2D) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	// Set attributes
	if p.W != 0 {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "w"}, Value: fmt.Sprintf("%d", p.W)})
	}
	if p.H != 0 {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "h"}, Value: fmt.Sprintf("%d", p.H)})
	}
	if p.Fill != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "fill"}, Value: p.Fill})
	}
	if p.Stroke != nil {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "stroke"}, Value: boolAttrValue(*p.Stroke)})
	}
	if p.ExtrusionOk != nil {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "extrusionOk"}, Value: boolAttrValue(*p.ExtrusionOk)})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}

	ns := "http://schemas.openxmlformats.org/drawingml/2006/main"

	if len(p.cmdOrder) > 0 {
		for _, ref := range p.cmdOrder {
			switch ref.kind {
			case pathCmdMoveTo:
				if ref.index < len(p.MoveTo) {
					if err := e.EncodeElement(p.MoveTo[ref.index], xml.StartElement{Name: xml.Name{Space: ns, Local: "moveTo"}}); err != nil {
						return err
					}
				}
			case pathCmdLnTo:
				if ref.index < len(p.LnTo) {
					if err := e.EncodeElement(p.LnTo[ref.index], xml.StartElement{Name: xml.Name{Space: ns, Local: "lnTo"}}); err != nil {
						return err
					}
				}
			case pathCmdArcTo:
				if ref.index < len(p.ArcTo) {
					if err := e.EncodeElement(p.ArcTo[ref.index], xml.StartElement{Name: xml.Name{Space: ns, Local: "arcTo"}}); err != nil {
						return err
					}
				}
			case pathCmdQuadBezTo:
				if ref.index < len(p.QuadBezTo) {
					if err := e.EncodeElement(p.QuadBezTo[ref.index], xml.StartElement{Name: xml.Name{Space: ns, Local: "quadBezTo"}}); err != nil {
						return err
					}
				}
			case pathCmdCubicBezTo:
				if ref.index < len(p.CubicBezTo) {
					if err := e.EncodeElement(p.CubicBezTo[ref.index], xml.StartElement{Name: xml.Name{Space: ns, Local: "cubicBezTo"}}); err != nil {
						return err
					}
				}
			case pathCmdClose:
				if ref.index < len(p.Close) {
					if err := e.EncodeElement(p.Close[ref.index], xml.StartElement{Name: xml.Name{Space: ns, Local: "close"}}); err != nil {
						return err
					}
				}
			}
		}
	} else {
		for _, v := range p.MoveTo {
			if err := e.EncodeElement(v, xml.StartElement{Name: xml.Name{Space: ns, Local: "moveTo"}}); err != nil {
				return err
			}
		}
		for _, v := range p.LnTo {
			if err := e.EncodeElement(v, xml.StartElement{Name: xml.Name{Space: ns, Local: "lnTo"}}); err != nil {
				return err
			}
		}
		for _, v := range p.ArcTo {
			if err := e.EncodeElement(v, xml.StartElement{Name: xml.Name{Space: ns, Local: "arcTo"}}); err != nil {
				return err
			}
		}
		for _, v := range p.QuadBezTo {
			if err := e.EncodeElement(v, xml.StartElement{Name: xml.Name{Space: ns, Local: "quadBezTo"}}); err != nil {
				return err
			}
		}
		for _, v := range p.CubicBezTo {
			if err := e.EncodeElement(v, xml.StartElement{Name: xml.Name{Space: ns, Local: "cubicBezTo"}}); err != nil {
				return err
			}
		}
		for _, v := range p.Close {
			if err := e.EncodeElement(v, xml.StartElement{Name: xml.Name{Space: ns, Local: "close"}}); err != nil {
				return err
			}
		}
	}

	return e.EncodeToken(start.End())
}

// MoveToXML represents CT_Path2DMoveTo (a:moveTo)
type MoveToXML struct {
	Pt *PtXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pt"`
}

// LnToXML represents CT_Path2DLineTo (a:lnTo)
type LnToXML struct {
	Pt *PtXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pt"`
}

// ArcToXML represents CT_Path2DArcTo (a:arcTo). All four attributes are
// required by the schema; stAng/swAng must not be omitted when zero (a
// zero-angle arc is valid), so they carry no omitempty.
type ArcToXML struct {
	WR    string `xml:"wR,attr"`
	HR    string `xml:"hR,attr"`
	StAng string `xml:"stAng,attr"`
	SwAng string `xml:"swAng,attr"`
}

// QuadBezToXML represents CT_Path2DQuadBezierTo (a:quadBezTo)
type QuadBezToXML struct {
	Pt []*PtXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pt"`
}

// CubicBezToXML represents CT_Path2DCubicBezierTo (a:cubicBezTo)
type CubicBezToXML struct {
	Pt []*PtXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pt"`
}

// CloseXML represents CT_Path2DClose (a:close)
type CloseXML struct{}

// PtXML represents CT_AdjPoint2D (a:pt)
type PtXML struct {
	X string `xml:"x,attr"`
	Y string `xml:"y,attr"`
}

// RectXML represents CT_GeomRect (a:rect)
type RectXML struct {
	L string `xml:"l,attr"`
	T string `xml:"t,attr"`
	R string `xml:"r,attr"`
	B string `xml:"b,attr"`
}

// CxnLst represents CT_ConnectionSiteList (a:cxnLst)
type CxnLst struct {
	Cxn []*CxnXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main cxn,omitempty"`
}

// CxnXML represents CT_ConnectionSite (a:cxn)
type CxnXML struct {
	Ang string `xml:"ang,attr"`
	Pos *PtXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pos"`
}

// AhLst represents CT_AdjustHandleList (a:ahLst)
type AhLst struct {
	AhXY    []*AhXY    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main ahXY,omitempty"`
	AhPolar []*AhPolar `xml:"http://schemas.openxmlformats.org/drawingml/2006/main ahPolar,omitempty"`
}

// AhXY represents CT_XYAdjustHandle (a:ahXY)
type AhXY struct {
	GdRefX string `xml:"gdRefX,attr,omitempty"`
	MinX   string `xml:"minX,attr,omitempty"`
	MaxX   string `xml:"maxX,attr,omitempty"`
	GdRefY string `xml:"gdRefY,attr,omitempty"`
	MinY   string `xml:"minY,attr,omitempty"`
	MaxY   string `xml:"maxY,attr,omitempty"`
	Pos    *PtXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pos"`
}

// AhPolar represents CT_PolarAdjustHandle (a:ahPolar)
type AhPolar struct {
	GdRefR   string `xml:"gdRefR,attr,omitempty"`
	MinR     string `xml:"minR,attr,omitempty"`
	MaxR     string `xml:"maxR,attr,omitempty"`
	GdRefAng string `xml:"gdRefAng,attr,omitempty"`
	MinAng   string `xml:"minAng,attr,omitempty"`
	MaxAng   string `xml:"maxAng,attr,omitempty"`
	Pos      *PtXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pos"`
}

// Xfrm represents CT_Transform2D (a:xfrm)
type Xfrm struct {
	Rot   int32   `xml:"rot,attr,omitempty"`
	FlipH bool    `xml:"flipH,attr,omitempty"`
	FlipV bool    `xml:"flipV,attr,omitempty"`
	Off   *OffXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main off,omitempty"`
	Ext   *ExtXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main ext,omitempty"`
}

// OffXML represents CT_Point2D (a:off)
type OffXML struct {
	X int64 `xml:"x,attr"`
	Y int64 `xml:"y,attr"`
}

// ExtXML represents CT_PositiveSize2D (a:ext)
type ExtXML struct {
	Cx int64 `xml:"cx,attr"`
	Cy int64 `xml:"cy,attr"`
}

// GrpXfrm represents CT_GroupTransform2D (a:xfrm for groups)
type GrpXfrm struct {
	Rot   int32   `xml:"rot,attr,omitempty"`
	FlipH bool    `xml:"flipH,attr,omitempty"`
	FlipV bool    `xml:"flipV,attr,omitempty"`
	Off   *OffXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main off,omitempty"`
	Ext   *ExtXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main ext,omitempty"`
	ChOff *OffXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main chOff,omitempty"`
	ChExt *ExtXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main chExt,omitempty"`
}

// --- Additional Geometry Types ---

// Tile represents CT_Tile (a:tile) for tiled fill patterns
type Tile struct {
	Tx   int64  `xml:"tx,attr,omitempty"`
	Ty   int64  `xml:"ty,attr,omitempty"`
	Sx   int32  `xml:"sx,attr,omitempty"`
	Sy   int32  `xml:"sy,attr,omitempty"`
	Flip string `xml:"flip,attr,omitempty"` // none, x, y, xy
	Algn string `xml:"algn,attr,omitempty"` // tl, t, tr, l, ctr, r, bl, b, br
}

// Stretch represents CT_StretchInfoProperties (a:stretch)
type Stretch struct {
	FillRect *RelRect `xml:"http://schemas.openxmlformats.org/drawingml/2006/main fillRect,omitempty"`
}

// SrcRect represents CT_RelativeRect for source rectangle (a:srcRect)
type SrcRect struct {
	L Percentage `xml:"l,attr,omitempty"`
	T Percentage `xml:"t,attr,omitempty"`
	R Percentage `xml:"r,attr,omitempty"`
	B Percentage `xml:"b,attr,omitempty"`
}

// --- Preset Shape Types ---
// These are the valid values for PrstGeom.Prst attribute
// See ECMA-376 Part 1, Section 20.1.10.55 ST_ShapeType

// Common preset shape names (partial list - full list has 187 shapes):
// line, lineInv, triangle, rtTriangle, rect, diamond, parallelogram, trapezoid,
// pentagon, hexagon, heptagon, octagon, decagon, dodecagon, star4, star5, star6,
// star7, star8, star10, star12, star16, star24, star32, roundRect, round1Rect,
// round2SameRect, round2DiagRect, snipRoundRect, snip1Rect, snip2SameRect,
// snip2DiagRect, plaque, ellipse, teardrop, homePlate, chevron, pieWedge, pie,
// blockArc, donut, noSmoking, rightArrow, leftArrow, upArrow, downArrow,
// stripedRightArrow, notchedRightArrow, bentUpArrow, leftRightArrow, upDownArrow,
// leftUpArrow, leftRightUpArrow, quadArrow, leftArrowCallout, rightArrowCallout,
// upArrowCallout, downArrowCallout, leftRightArrowCallout, upDownArrowCallout,
// quadArrowCallout, bentArrow, uturnArrow, circularArrow, leftCircularArrow,
// leftRightCircularArrow, curvedRightArrow, curvedLeftArrow, curvedUpArrow,
// curvedDownArrow, swooshArrow, cube, can, lightningBolt, heart, sun, moon,
// smileyFace, irregularSeal1, irregularSeal2, foldedCorner, bevel, frame,
// halfFrame, corner, diagStripe, chord, arc, leftBracket, rightBracket,
// leftBrace, rightBrace, bracketPair, bracePair, straightConnector1,
// bentConnector2, bentConnector3, bentConnector4, bentConnector5,
// curvedConnector2, curvedConnector3, curvedConnector4, curvedConnector5,
// callout1, callout2, callout3, accentCallout1, accentCallout2, accentCallout3,
// borderCallout1, borderCallout2, borderCallout3, accentBorderCallout1,
// accentBorderCallout2, accentBorderCallout3, wedgeRectCallout,
// wedgeRoundRectCallout, wedgeEllipseCallout, cloudCallout, cloud,
// ribbon, ribbon2, ellipseRibbon, ellipseRibbon2, leftRightRibbon,
// verticalScroll, horizontalScroll, wave, doubleWave, plus, flowChartProcess,
// flowChartDecision, flowChartInputOutput, flowChartPredefinedProcess,
// flowChartInternalStorage, flowChartDocument, flowChartMultidocument,
// flowChartTerminator, flowChartPreparation, flowChartManualInput,
// flowChartManualOperation, flowChartConnector, flowChartPunchedCard,
// flowChartPunchedTape, flowChartSummingJunction, flowChartOr, flowChartCollate,
// flowChartSort, flowChartExtract, flowChartMerge, flowChartOfflineStorage,
// flowChartOnlineStorage, flowChartMagneticTape, flowChartMagneticDisk,
// flowChartMagneticDrum, flowChartDisplay, flowChartDelay, flowChartAlternateProcess,
// flowChartOffpageConnector, actionButtonBlank, actionButtonHome, actionButtonHelp,
// actionButtonInformation, actionButtonForwardNext, actionButtonBackPrevious,
// actionButtonEnd, actionButtonBeginning, actionButtonReturn, actionButtonDocument,
// actionButtonSound, actionButtonMovie, gear6, gear9, funnel, mathPlus, mathMinus,
// mathMultiply, mathDivide, mathEqual, mathNotEqual, cornerTabs, squareTabs,
// plaqueTabs, chartX, chartStar, chartPlus

// --- Geometry Guide Formulas ---
// Formula syntax for Gd.Fmla attribute:
// */  - multiply and divide: */ x y z = x * y / z
// +-  - add and subtract: +- x y z = x + y - z
// +/  - add and divide: +/ x y z = (x + y) / z
// ?:  - if-else: ?: x y z = if x > 0 then y else z
// abs - absolute value: abs x
// at2 - arc tangent: at2 x y = atan2(y, x) in 60000ths of a degree
// cat2 - cosine arc tangent: cat2 x y z = x * cos(atan2(z, y))
// cos - cosine: cos x y = x * cos(y) where y is in 60000ths of a degree
// max - maximum: max x y
// min - minimum: min x y
// mod - modulus: mod x y z = sqrt(x^2 + y^2 + z^2)
// pin - pin value: pin x y z = clamp y between x and z
// sat2 - sine arc tangent: sat2 x y z = x * sin(atan2(z, y))
// sin - sine: sin x y = x * sin(y) where y is in 60000ths of a degree
// sqrt - square root: sqrt x
// tan - tangent: tan x y = x * tan(y) where y is in 60000ths of a degree
// val - value: val x = literal value x

// --- Connection Site Angle Values ---
// Standard connection site angles in 60000ths of a degree:
// 0 = right (3 o'clock)
// 5400000 = bottom (6 o'clock)
// 10800000 = left (9 o'clock)
// 16200000 = top (12 o'clock)

// --- Ratio and Percentage Units ---
// Many geometry values use these unit types:
// ST_Coordinate = EMUs (914400 EMUs = 1 inch)
// ST_PositiveCoordinate = positive EMUs
// ST_Percentage = 1000ths of a percent (100000 = 100%)
// ST_PositivePercentage = positive percentage
// ST_PositiveFixedPercentage = 0 to 100000
// ST_Angle = 60000ths of a degree (5400000 = 90 degrees)
// ST_PositiveFixedAngle = 0 to 21600000
// ST_AdjCoordinate = either a literal or a formula reference
// ST_AdjAngle = either a literal angle or a formula reference
