package oxml

import (
	"encoding/xml"
	"fmt"

	"github.com/mgilbir/spine/common/dml"
	coxml "github.com/mgilbir/spine/common/oxml"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// AlternateContent is an alias for the shared mc:AlternateContent type.
type AlternateContent = coxml.AlternateContent

// Slide is the root element of a slide part.
//
// AlternateContent holds every root-level mc:AlternateContent in document
// order (C223: a single pointer collapsed multiple siblings to the last).
// Each element's position relative to the typed children is tracked by
// acAnchors; see root_marshal.go.
type Slide struct {
	XMLName          xml.Name            `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sld"`
	ShowMasterSp     *bool               `xml:"showMasterSp,attr,omitempty"`
	ShowMasterPhAnim *bool               `xml:"showMasterPhAnim,attr,omitempty"`
	Show             *bool               `xml:"show,attr,omitempty"`
	CSld             *CommonSlideData    `xml:"cSld"`
	ClrMapOvr        *ColorMapOverride   `xml:"clrMapOvr,omitempty"`
	Transition       *Transition         `xml:"transition,omitempty"`
	AlternateContent []*AlternateContent `xml:"http://schemas.openxmlformats.org/markup-compatibility/2006 AlternateContent,omitempty"`
	Timing           *Timing             `xml:"timing,omitempty"`
	ExtLst           *ExtensionList      `xml:"extLst,omitempty"`
	acAnchors        []string
}

// SlideLayout is the root element of a slide layout part.
// Attribute order matches XSD CT_SlideLayout definition.
type SlideLayout struct {
	XMLName          xml.Name            `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sldLayout"`
	ShowMasterSp     *bool               `xml:"showMasterSp,attr,omitempty"`
	ShowMasterPhAnim *bool               `xml:"showMasterPhAnim,attr,omitempty"`
	Type             string              `xml:"type,attr,omitempty"`
	Preserve         bool                `xml:"preserve,attr,omitempty"`
	UserDrawn        bool                `xml:"userDrawn,attr,omitempty"`
	MatchingName     string              `xml:"matchingName,attr,omitempty"`
	CSld             *CommonSlideData    `xml:"cSld"`
	ClrMapOvr        *ColorMapOverride   `xml:"clrMapOvr,omitempty"`
	Transition       *Transition         `xml:"transition,omitempty"`
	AlternateContent []*AlternateContent `xml:"http://schemas.openxmlformats.org/markup-compatibility/2006 AlternateContent,omitempty"`
	Timing           *Timing             `xml:"timing,omitempty"`
	Hf               *HeaderFooter       `xml:"hf,omitempty"`
	ExtLst           *ExtensionList      `xml:"extLst,omitempty"`
	acAnchors        []string
}

// SlideMaster is the root element of a slide master part.
type SlideMaster struct {
	XMLName          xml.Name            `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sldMaster"`
	Preserve         bool                `xml:"preserve,attr,omitempty"`
	CSld             *CommonSlideData    `xml:"cSld"`
	ClrMap           *ColorMap           `xml:"clrMap,omitempty"`
	SlideLayoutIDs   *SlideLayoutIDs     `xml:"sldLayoutIdLst,omitempty"`
	Transition       *Transition         `xml:"transition,omitempty"`
	AlternateContent []*AlternateContent `xml:"http://schemas.openxmlformats.org/markup-compatibility/2006 AlternateContent,omitempty"`
	Timing           *Timing             `xml:"timing,omitempty"`
	Hf               *HeaderFooter       `xml:"hf,omitempty"`
	TxStyles         *TxStyles           `xml:"txStyles,omitempty"`
	ExtLst           *ExtensionList      `xml:"extLst,omitempty"`
	acAnchors        []string
}

// SlideLayoutIDs contains a list of slide layout ID references.
type SlideLayoutIDs struct {
	SlideLayoutID []SlideLayoutID `xml:"sldLayoutId"`
}

// SlideLayoutID references a slide layout.
type SlideLayoutID struct {
	ID     uint32         `xml:"id,attr,omitempty"`
	RID    string         `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
	ExtLst *ExtensionList `xml:"extLst,omitempty"`
}

// MarshalXML implements custom XML marshaling for SlideLayoutID.
// Uses r:id attribute to match OOXML conventions (requires xmlns:r declaration in parent).
func (s SlideLayoutID) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if s.ID > 0 {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "id"}, Value: fmt.Sprintf("%d", s.ID)})
	}
	// Use r:id directly - the r prefix is declared in the root slideMaster element
	start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "r:id"}, Value: s.RID})
	return e.EncodeElement(struct{}{}, start)
}

// UnmarshalXML implements custom XML unmarshaling for SlideLayoutID.
// Handles both namespaced (relationships:id) and prefixed (r:id) formats,
// and captures the optional extLst child (C225).
func (s *SlideLayoutID) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "id" && (attr.Name.Space == "" || attr.Name.Space == NsPresentationML):
			// Numeric ID
			var id uint32
			_, _ = fmt.Sscanf(attr.Value, "%d", &id)
			s.ID = id
		case attr.Name.Local == "id" && attr.Name.Space == NsRelationships:
			// Relationship ID with full namespace
			s.RID = attr.Value
		case attr.Name.Local == "r:id":
			// Relationship ID with r: prefix (our marshaled format)
			s.RID = attr.Value
		}
	}
	return unmarshalIDEntryChildren(d, &s.ExtLst)
}

// CommonSlideData contains elements common to slides, layouts, and masters.
//
// The optional custDataLst and controls children (the latter holds ActiveX
// controls) are captured as raw bytes so a save re-emits them instead of
// deleting them (C33). Their schema position is fixed (bg?, spTree,
// custDataLst?, controls?, extLst?), so raw fields preserve position too.
type CommonSlideData struct {
	Name        string         `xml:"name,attr,omitempty"`
	Bg          *Background    `xml:"bg,omitempty"`
	SpTree      *ShapeTree     `xml:"spTree"`
	CustDataLst []byte         `xml:"-"`
	Controls    []byte         `xml:"-"`
	ExtLst      *ExtensionList `xml:"extLst,omitempty"`
}

// UnmarshalXML implements custom unmarshaling for CommonSlideData so the
// unmodeled custDataLst and controls children are kept as raw bytes.
func (c *CommonSlideData) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Space == "" && attr.Name.Local == "name" {
			c.Name = attr.Value
		}
	}
	captureRaw := func(t xml.StartElement) ([]byte, error) {
		var inner struct {
			Content []byte `xml:",innerxml"`
		}
		if err := d.DecodeElement(&inner, &t); err != nil {
			return nil, err
		}
		return encodeRawChild(t, inner.Content), nil
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "bg":
				c.Bg = &Background{}
				if err := d.DecodeElement(c.Bg, &t); err != nil {
					return err
				}
			case "spTree":
				c.SpTree = &ShapeTree{}
				if err := d.DecodeElement(c.SpTree, &t); err != nil {
					return err
				}
			case "custDataLst":
				if c.CustDataLst, err = captureRaw(t); err != nil {
					return err
				}
			case "controls":
				if c.Controls, err = captureRaw(t); err != nil {
					return err
				}
			case "extLst":
				c.ExtLst = &ExtensionList{}
				if err := d.DecodeElement(c.ExtLst, &t); err != nil {
					return err
				}
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

// MarshalToBuilder implements xmlb.BuilderMarshaler, re-emitting the raw
// custDataLst and controls children in their schema position.
func (c *CommonSlideData) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if c.Name != "" {
		attrs = append(attrs, xmlb.StrAttr("name", c.Name))
	}
	b.StartElement(ns, localName, attrs...)
	if c.Bg != nil {
		b.MarshalElement(ns, "bg", c.Bg)
	}
	if c.SpTree != nil {
		b.MarshalElement(ns, "spTree", c.SpTree)
	}
	if len(c.CustDataLst) > 0 {
		b.WriteRaw(c.CustDataLst)
	}
	if len(c.Controls) > 0 {
		b.WriteRaw(c.Controls)
	}
	if c.ExtLst != nil {
		b.MarshalElement(ns, "extLst", c.ExtLst)
	}
	b.EndElement(ns, localName)
}

// Background represents a slide background (p:bg).
type Background struct {
	BwMode string           `xml:"bwMode,attr,omitempty"`
	BgPr   *BackgroundProps `xml:"bgPr,omitempty"`
	BgRef  *dml.FillRef     `xml:"bgRef,omitempty"`
}

// BackgroundProps contains background fill properties.
type BackgroundProps struct {
	NoFill    *dml.NoFillXML   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main noFill,omitempty"`
	SolidFill *dml.SolidFill   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	GradFill  *dml.GradFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gradFill,omitempty"`
	BlipFill  *dml.BlipFillXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blipFill,omitempty"`
	PattFill  *dml.PattFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pattFill,omitempty"`
	EffectLst *dml.EffectLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main effectLst,omitempty"`
	ExtLst    *dml.ExtLst      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
}

// ShapeTree is the container for shapes on a slide.
// It implements custom unmarshal/marshal to preserve child element ordering,
// which determines z-order (per XSD: xs:choice maxOccurs="unbounded").
type ShapeTree struct {
	NvGrpSpPr    *NvGrpSpPr         `xml:"nvGrpSpPr"`
	GrpSpPr      *GrpSpPr           `xml:"grpSpPr"`
	Sp           []*Shape           `xml:"-"`
	Pic          []*Picture         `xml:"-"`
	GraphicFrame []*GraphicFrame    `xml:"-"`
	GrpSp        []*GroupShape      `xml:"-"`
	CxnSp        []*ConnectionShape `xml:"-"`
	// AltContent holds mc:AlternateContent children (ink, 2010+ shapes with
	// fallbacks); RawXML holds any other unmodeled child (p:contentPart,
	// extLst, ...) as reconstructed raw bytes. Both are kept in childOrder so
	// a save re-emits them in position instead of deleting them (C32). They
	// are never referenced by the domain model's shapeRefs.
	AltContent []*AlternateContent `xml:"-"`
	RawXML     [][]byte            `xml:"-"`
	childOrder []ChildRef          // tracks interleaved child order
}

// ChildKind identifies a shape child element type.
type ChildKind int

const (
	ChildSp ChildKind = iota
	ChildPic
	ChildGraphicFrame
	ChildGrpSp
	ChildCxnSp
	// ChildAltContent and ChildRawXML index the AltContent and RawXML slices:
	// preserved content the domain model never materializes or removes.
	ChildAltContent
	ChildRawXML
)

// ChildRef references a child element by kind and index into its typed slice.
type ChildRef struct {
	Kind  ChildKind
	Index int
}

// ChildOrder returns the child order tracking slice.
// This is used during shape materialization to iterate shapes in their original z-order.
func (st *ShapeTree) ChildOrder() []ChildRef {
	return st.childOrder
}

// SetChildRef updates the child reference at the given index.
func (st *ShapeTree) SetChildRef(i int, ref ChildRef) {
	if i >= 0 && i < len(st.childOrder) {
		st.childOrder[i] = ref
	}
}

// ClearChildOrder removes the child order tracking (used when shapes are rebuilt programmatically).
func (st *ShapeTree) ClearChildOrder() {
	st.childOrder = nil
}

// RemoveChildren rebuilds the shape tree, omitting the given child references
// and preserving every other child (including kinds not materialized by the
// domain model, such as connectors and non-table graphic frames) in order.
func (st *ShapeTree) RemoveChildren(refs []ChildRef) {
	if len(refs) == 0 {
		return
	}
	drop := make(map[ChildRef]bool, len(refs))
	for _, r := range refs {
		drop[r] = true
	}

	if len(st.childOrder) == 0 {
		// Tree without interleaved order tracking (built programmatically, e.g.
		// rebuilt from the domain model): filter the typed slices directly.
		// Marshal order stays by-type, matching how the tree was written.
		var sp []*Shape
		for i, v := range st.Sp {
			if !drop[ChildRef{ChildSp, i}] {
				sp = append(sp, v)
			}
		}
		var pic []*Picture
		for i, v := range st.Pic {
			if !drop[ChildRef{ChildPic, i}] {
				pic = append(pic, v)
			}
		}
		var gf []*GraphicFrame
		for i, v := range st.GraphicFrame {
			if !drop[ChildRef{ChildGraphicFrame, i}] {
				gf = append(gf, v)
			}
		}
		var grp []*GroupShape
		for i, v := range st.GrpSp {
			if !drop[ChildRef{ChildGrpSp, i}] {
				grp = append(grp, v)
			}
		}
		var cxn []*ConnectionShape
		for i, v := range st.CxnSp {
			if !drop[ChildRef{ChildCxnSp, i}] {
				cxn = append(cxn, v)
			}
		}
		st.Sp, st.Pic, st.GraphicFrame, st.GrpSp, st.CxnSp = sp, pic, gf, grp, cxn
		return
	}

	var (
		sp    []*Shape
		pic   []*Picture
		gf    []*GraphicFrame
		grp   []*GroupShape
		cxn   []*ConnectionShape
		ac    []*AlternateContent
		raw   [][]byte
		order []ChildRef
	)
	for _, ref := range st.childOrder {
		if drop[ref] {
			continue
		}
		switch ref.Kind {
		case ChildSp:
			if ref.Index < len(st.Sp) {
				order = append(order, ChildRef{ChildSp, len(sp)})
				sp = append(sp, st.Sp[ref.Index])
			}
		case ChildPic:
			if ref.Index < len(st.Pic) {
				order = append(order, ChildRef{ChildPic, len(pic)})
				pic = append(pic, st.Pic[ref.Index])
			}
		case ChildGraphicFrame:
			if ref.Index < len(st.GraphicFrame) {
				order = append(order, ChildRef{ChildGraphicFrame, len(gf)})
				gf = append(gf, st.GraphicFrame[ref.Index])
			}
		case ChildGrpSp:
			if ref.Index < len(st.GrpSp) {
				order = append(order, ChildRef{ChildGrpSp, len(grp)})
				grp = append(grp, st.GrpSp[ref.Index])
			}
		case ChildCxnSp:
			if ref.Index < len(st.CxnSp) {
				order = append(order, ChildRef{ChildCxnSp, len(cxn)})
				cxn = append(cxn, st.CxnSp[ref.Index])
			}
		case ChildAltContent:
			if ref.Index < len(st.AltContent) {
				order = append(order, ChildRef{ChildAltContent, len(ac)})
				ac = append(ac, st.AltContent[ref.Index])
			}
		case ChildRawXML:
			if ref.Index < len(st.RawXML) {
				order = append(order, ChildRef{ChildRawXML, len(raw)})
				raw = append(raw, st.RawXML[ref.Index])
			}
		}
	}
	st.Sp, st.Pic, st.GraphicFrame, st.GrpSp, st.CxnSp = sp, pic, gf, grp, cxn
	st.AltContent, st.RawXML = ac, raw
	st.childOrder = order
}

// AppendSp appends a shape as the last child, keeping z-order tracking
// consistent on trees parsed from a file (appended children render on top).
func (st *ShapeTree) AppendSp(sp *Shape) {
	st.Sp = append(st.Sp, sp)
	if st.childOrder != nil {
		st.childOrder = append(st.childOrder, ChildRef{ChildSp, len(st.Sp) - 1})
	}
}

// AppendPic appends a picture as the last child (see AppendSp).
func (st *ShapeTree) AppendPic(pic *Picture) {
	st.Pic = append(st.Pic, pic)
	if st.childOrder != nil {
		st.childOrder = append(st.childOrder, ChildRef{ChildPic, len(st.Pic) - 1})
	}
}

// AppendGraphicFrame appends a graphic frame as the last child (see AppendSp).
func (st *ShapeTree) AppendGraphicFrame(gf *GraphicFrame) {
	st.GraphicFrame = append(st.GraphicFrame, gf)
	if st.childOrder != nil {
		st.childOrder = append(st.childOrder, ChildRef{ChildGraphicFrame, len(st.GraphicFrame) - 1})
	}
}

// AppendGrpSp appends a group shape as the last child (see AppendSp).
func (st *ShapeTree) AppendGrpSp(grp *GroupShape) {
	st.GrpSp = append(st.GrpSp, grp)
	if st.childOrder != nil {
		st.childOrder = append(st.childOrder, ChildRef{ChildGrpSp, len(st.GrpSp) - 1})
	}
}

// MaxShapeID returns the highest cNvPr id anywhere in the tree, descending
// into group shapes (0 when the tree holds none). New shapes must use ids
// above it: PowerPoint requires slide-wide uniqueness.
func (st *ShapeTree) MaxShapeID() uint32 {
	var max uint32
	bump := func(cNvPr *dml.CNvPr) {
		if cNvPr != nil && cNvPr.Id > max {
			max = cNvPr.Id
		}
	}
	for _, sp := range st.Sp {
		if sp.NvSpPr != nil {
			bump(sp.NvSpPr.CNvPr)
		}
	}
	for _, pic := range st.Pic {
		if pic.NvPicPr != nil {
			bump(pic.NvPicPr.CNvPr)
		}
	}
	for _, gf := range st.GraphicFrame {
		if gf.NvGraphicFramePr != nil {
			bump(gf.NvGraphicFramePr.CNvPr)
		}
	}
	for _, cs := range st.CxnSp {
		if cs.NvCxnSpPr != nil {
			bump(cs.NvCxnSpPr.CNvPr)
		}
	}
	for _, gs := range st.GrpSp {
		if id := gs.maxShapeID(); id > max {
			max = id
		}
	}
	return max
}

func (gs *GroupShape) maxShapeID() uint32 {
	var max uint32
	bump := func(cNvPr *dml.CNvPr) {
		if cNvPr != nil && cNvPr.Id > max {
			max = cNvPr.Id
		}
	}
	if gs.NvGrpSpPr != nil {
		bump(gs.NvGrpSpPr.CNvPr)
	}
	for _, sp := range gs.Shapes {
		if sp.NvSpPr != nil {
			bump(sp.NvSpPr.CNvPr)
		}
	}
	for _, pic := range gs.Pictures {
		if pic.NvPicPr != nil {
			bump(pic.NvPicPr.CNvPr)
		}
	}
	for _, gf := range gs.GraphicFrames {
		if gf.NvGraphicFramePr != nil {
			bump(gf.NvGraphicFramePr.CNvPr)
		}
	}
	for _, cs := range gs.ConnectionShapes {
		if cs.NvCxnSpPr != nil {
			bump(cs.NvCxnSpPr.CNvPr)
		}
	}
	for _, sub := range gs.GroupShapes {
		if id := sub.maxShapeID(); id > max {
			max = id
		}
	}
	return max
}

// UnmarshalXML implements custom unmarshaling for ShapeTree to preserve child order.
func (st *ShapeTree) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "nvGrpSpPr":
				st.NvGrpSpPr = &NvGrpSpPr{}
				if err := d.DecodeElement(st.NvGrpSpPr, &t); err != nil {
					return err
				}
			case "grpSpPr":
				st.GrpSpPr = &GrpSpPr{}
				if err := d.DecodeElement(st.GrpSpPr, &t); err != nil {
					return err
				}
			case "sp":
				sp := &Shape{}
				if err := d.DecodeElement(sp, &t); err != nil {
					return err
				}
				st.childOrder = append(st.childOrder, ChildRef{ChildSp, len(st.Sp)})
				st.Sp = append(st.Sp, sp)
			case "pic":
				pic := &Picture{}
				if err := d.DecodeElement(pic, &t); err != nil {
					return err
				}
				st.childOrder = append(st.childOrder, ChildRef{ChildPic, len(st.Pic)})
				st.Pic = append(st.Pic, pic)
			case "graphicFrame":
				gf := &GraphicFrame{}
				if err := d.DecodeElement(gf, &t); err != nil {
					return err
				}
				st.childOrder = append(st.childOrder, ChildRef{ChildGraphicFrame, len(st.GraphicFrame)})
				st.GraphicFrame = append(st.GraphicFrame, gf)
			case "grpSp":
				gs := &GroupShape{}
				if err := d.DecodeElement(gs, &t); err != nil {
					return err
				}
				st.childOrder = append(st.childOrder, ChildRef{ChildGrpSp, len(st.GrpSp)})
				st.GrpSp = append(st.GrpSp, gs)
			case "cxnSp":
				cs := &ConnectionShape{}
				if err := d.DecodeElement(cs, &t); err != nil {
					return err
				}
				st.childOrder = append(st.childOrder, ChildRef{ChildCxnSp, len(st.CxnSp)})
				st.CxnSp = append(st.CxnSp, cs)
			default:
				if t.Name.Space == xmlb.NSMarkupCompatibility && t.Name.Local == "AlternateContent" {
					ac := &AlternateContent{}
					if err := d.DecodeElement(ac, &t); err != nil {
						return err
					}
					st.childOrder = append(st.childOrder, ChildRef{ChildAltContent, len(st.AltContent)})
					st.AltContent = append(st.AltContent, ac)
					continue
				}
				// Unmodeled child (p:contentPart, extLst, ...): capture the
				// whole element raw so a save re-emits it in position (C32).
				var inner struct {
					Content []byte `xml:",innerxml"`
				}
				if err := d.DecodeElement(&inner, &t); err != nil {
					return err
				}
				st.childOrder = append(st.childOrder, ChildRef{ChildRawXML, len(st.RawXML)})
				st.RawXML = append(st.RawXML, encodeRawChild(t, inner.Content))
			}
		case xml.EndElement:
			return nil
		}
	}
}

// MarshalToBuilder implements xmlb.BuilderMarshaler to preserve child element order.
func (st *ShapeTree) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName)

	if st.NvGrpSpPr != nil {
		b.MarshalElement(ns, "nvGrpSpPr", st.NvGrpSpPr)
	}
	if st.GrpSpPr != nil {
		b.MarshalElement(ns, "grpSpPr", st.GrpSpPr)
	}

	if len(st.childOrder) > 0 {
		// Write children in their original interleaved order
		for _, ref := range st.childOrder {
			switch ref.Kind {
			case ChildSp:
				if ref.Index < len(st.Sp) {
					b.MarshalElement(ns, "sp", st.Sp[ref.Index])
				}
			case ChildPic:
				if ref.Index < len(st.Pic) {
					b.MarshalElement(ns, "pic", st.Pic[ref.Index])
				}
			case ChildGraphicFrame:
				if ref.Index < len(st.GraphicFrame) {
					b.MarshalElement(ns, "graphicFrame", st.GraphicFrame[ref.Index])
				}
			case ChildGrpSp:
				if ref.Index < len(st.GrpSp) {
					b.MarshalElement(ns, "grpSp", st.GrpSp[ref.Index])
				}
			case ChildCxnSp:
				if ref.Index < len(st.CxnSp) {
					b.MarshalElement(ns, "cxnSp", st.CxnSp[ref.Index])
				}
			case ChildAltContent:
				if ref.Index < len(st.AltContent) {
					b.MarshalElement(xmlb.NSMarkupCompatibility, "AlternateContent", st.AltContent[ref.Index])
				}
			case ChildRawXML:
				if ref.Index < len(st.RawXML) {
					b.WriteRaw(st.RawXML[ref.Index])
				}
			}
		}
	} else {
		// No order tracking (programmatically built tree) - write by type
		for _, sp := range st.Sp {
			b.MarshalElement(ns, "sp", sp)
		}
		for _, pic := range st.Pic {
			b.MarshalElement(ns, "pic", pic)
		}
		for _, gf := range st.GraphicFrame {
			b.MarshalElement(ns, "graphicFrame", gf)
		}
		for _, gs := range st.GrpSp {
			b.MarshalElement(ns, "grpSp", gs)
		}
		for _, cs := range st.CxnSp {
			b.MarshalElement(ns, "cxnSp", cs)
		}
		for _, ac := range st.AltContent {
			b.MarshalElement(xmlb.NSMarkupCompatibility, "AlternateContent", ac)
		}
		for _, raw := range st.RawXML {
			b.WriteRaw(raw)
		}
	}

	b.EndElement(ns, localName)
}

// ColorMap defines the color mapping for a slide master.
type ColorMap struct {
	Bg1      string `xml:"bg1,attr"`
	Tx1      string `xml:"tx1,attr"`
	Bg2      string `xml:"bg2,attr"`
	Tx2      string `xml:"tx2,attr"`
	Accent1  string `xml:"accent1,attr"`
	Accent2  string `xml:"accent2,attr"`
	Accent3  string `xml:"accent3,attr"`
	Accent4  string `xml:"accent4,attr"`
	Accent5  string `xml:"accent5,attr"`
	Accent6  string `xml:"accent6,attr"`
	Hlink    string `xml:"hlink,attr"`
	FolHlink string `xml:"folHlink,attr"`
}

// ColorMapOverride specifies a color map override.
type ColorMapOverride struct {
	MasterClrMapping   *MasterColorMapping `xml:"http://schemas.openxmlformats.org/drawingml/2006/main masterClrMapping,omitempty"`
	OverrideClrMapping *ColorMap           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main overrideClrMapping,omitempty"`
}

// MasterColorMapping indicates to use the master's color mapping.
type MasterColorMapping struct{}

// TxStyles contains text styles for a slide master.
type TxStyles struct {
	TitleStyle *dml.LstStyle `xml:"titleStyle,omitempty"`
	BodyStyle  *dml.LstStyle `xml:"bodyStyle,omitempty"`
	OtherStyle *dml.LstStyle `xml:"otherStyle,omitempty"`
}
