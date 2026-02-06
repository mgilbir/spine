package oxml

import (
	"encoding/xml"
	"fmt"

	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// AlternateContent represents mc:AlternateContent from ECMA-376 Part 3.
// The mc structure is typed, but the inner content of mc:Choice and mc:Fallback
// is xsd:any from external schemas (p14, p15, etc.) and stored as raw bytes.
type AlternateContent struct {
	Requires        string // mc:Choice/@Requires (e.g., "p14")
	ChoiceContent   []byte // inner XML of mc:Choice (xsd:any from external schemas)
	FallbackContent []byte // inner XML of mc:Fallback (xsd:any)
}

// UnmarshalXML implements custom XML unmarshaling for AlternateContent.
// Parses mc:Choice and mc:Fallback children, capturing inner content as raw bytes.
func (ac *AlternateContent) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "Choice":
				// Extract Requires attribute
				for _, attr := range t.Attr {
					if attr.Name.Local == "Requires" {
						ac.Requires = attr.Value
					}
				}
				// Capture inner content as raw bytes
				var inner struct {
					Content []byte `xml:",innerxml"`
				}
				if err := d.DecodeElement(&inner, &t); err != nil {
					return err
				}
				ac.ChoiceContent = inner.Content
			case "Fallback":
				var inner struct {
					Content []byte `xml:",innerxml"`
				}
				if err := d.DecodeElement(&inner, &t); err != nil {
					return err
				}
				ac.FallbackContent = inner.Content
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

// MarshalToBuilder implements xmlb.BuilderMarshaler for AlternateContent.
// Uses inline namespace declarations matching Office conventions.
func (ac *AlternateContent) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	nsMC := xmlb.NSMarkupCompatibility
	prefixMC := xmlb.PrefixMarkupCompatibility

	// Build inline namespace attrs for mc:AlternateContent element.
	// Always declare xmlns:mc. Also declare extension NS from Requires prefix.
	var nsAttrs []xmlb.Attr
	if ac.Requires != "" {
		if extNS, ok := xmlb.ExtensionPrefixToNS[ac.Requires]; ok {
			nsAttrs = append(nsAttrs, xmlb.Attr{Name: "xmlns:" + ac.Requires, Value: extNS})
			// Register so child raw content with this prefix resolves correctly
			b.RegisterNamespace(extNS, ac.Requires)
		}
	}

	b.StartElementInlineNS(nsMC, prefixMC, "AlternateContent", nsAttrs...)

	// mc:Choice with Requires attribute
	b.StartElement(nsMC, "Choice", xmlb.StrAttr("Requires", ac.Requires))
	b.WriteRaw(ac.ChoiceContent)
	b.EndElement(nsMC, "Choice")

	// mc:Fallback with xmlns="" (Office convention to reset default NS)
	b.StartElement(nsMC, "Fallback", xmlb.Attr{Name: "xmlns", Value: ""})
	b.WriteRaw(ac.FallbackContent)
	b.EndElement(nsMC, "Fallback")

	b.EndElementInlineNS(prefixMC, "AlternateContent")

	// Reset so next usage gets fresh inline declarations
	b.ResetNamespaceDeclaration(nsMC)
	if ac.Requires != "" {
		if extNS, ok := xmlb.ExtensionPrefixToNS[ac.Requires]; ok {
			b.ResetNamespaceDeclaration(extNS)
		}
	}
}

// Slide is the root element of a slide part.
type Slide struct {
	XMLName          xml.Name          `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sld"`
	Show             *bool             `xml:"show,attr,omitempty"`
	CSld             *CommonSlideData  `xml:"cSld"`
	ClrMapOvr        *ColorMapOverride `xml:"clrMapOvr,omitempty"`
	Transition       *Transition       `xml:"transition,omitempty"`
	AlternateContent *AlternateContent `xml:"http://schemas.openxmlformats.org/markup-compatibility/2006 AlternateContent,omitempty"`
	Timing           *Timing           `xml:"timing,omitempty"`
	ExtLst           *ExtensionList    `xml:"extLst,omitempty"`
}

// SlideLayout is the root element of a slide layout part.
// Attribute order matches XSD CT_SlideLayout definition.
type SlideLayout struct {
	XMLName            xml.Name          `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sldLayout"`
	ShowMasterSp       *bool             `xml:"showMasterSp,attr,omitempty"`
	ShowMasterPhAnim   *bool             `xml:"showMasterPhAnim,attr,omitempty"`
	Type               string            `xml:"type,attr,omitempty"`
	Preserve           bool              `xml:"preserve,attr,omitempty"`
	UserDrawn          bool              `xml:"userDrawn,attr,omitempty"`
	MatchingName       string            `xml:"matchingName,attr,omitempty"`
	CSld               *CommonSlideData  `xml:"cSld"`
	ClrMapOvr          *ColorMapOverride `xml:"clrMapOvr,omitempty"`
	Transition         *Transition       `xml:"transition,omitempty"`
	AlternateContent   *AlternateContent `xml:"http://schemas.openxmlformats.org/markup-compatibility/2006 AlternateContent,omitempty"`
	Timing             *Timing           `xml:"timing,omitempty"`
	Hf                 *HeaderFooter     `xml:"hf,omitempty"`
	ExtLst             *ExtensionList    `xml:"extLst,omitempty"`
}

// SlideMaster is the root element of a slide master part.
type SlideMaster struct {
	XMLName          xml.Name          `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sldMaster"`
	Preserve         bool              `xml:"preserve,attr,omitempty"`
	CSld             *CommonSlideData  `xml:"cSld"`
	ClrMap           *ColorMap         `xml:"clrMap,omitempty"`
	SlideLayoutIDs   *SlideLayoutIDs   `xml:"sldLayoutIdLst,omitempty"`
	Transition       *Transition       `xml:"transition,omitempty"`
	AlternateContent *AlternateContent `xml:"http://schemas.openxmlformats.org/markup-compatibility/2006 AlternateContent,omitempty"`
	Timing           *Timing           `xml:"timing,omitempty"`
	Hf               *HeaderFooter     `xml:"hf,omitempty"`
	TxStyles         *TxStyles         `xml:"txStyles,omitempty"`
	ExtLst           *ExtensionList    `xml:"extLst,omitempty"`
}

// SlideLayoutIDs contains a list of slide layout ID references.
type SlideLayoutIDs struct {
	SlideLayoutID []SlideLayoutID `xml:"sldLayoutId"`
}

// SlideLayoutID references a slide layout.
type SlideLayoutID struct {
	ID  uint32 `xml:"id,attr,omitempty"`
	RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
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
// Handles both namespaced (relationships:id) and prefixed (r:id) formats.
func (s *SlideLayoutID) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "id" && (attr.Name.Space == "" || attr.Name.Space == NsPresentationML):
			// Numeric ID
			var id uint32
			fmt.Sscanf(attr.Value, "%d", &id)
			s.ID = id
		case attr.Name.Local == "id" && attr.Name.Space == NsRelationships:
			// Relationship ID with full namespace
			s.RID = attr.Value
		case attr.Name.Local == "r:id":
			// Relationship ID with r: prefix (our marshaled format)
			s.RID = attr.Value
		}
	}
	return d.Skip()
}

// CommonSlideData contains elements common to slides, layouts, and masters.
type CommonSlideData struct {
	Name   string         `xml:"name,attr,omitempty"`
	Bg     *Background    `xml:"bg,omitempty"`
	SpTree *ShapeTree     `xml:"spTree"`
	ExtLst *ExtensionList `xml:"extLst,omitempty"`
}

// Background represents a slide background (p:bg).
type Background struct {
	BwMode string          `xml:"bwMode,attr,omitempty"`
	BgPr   *BackgroundProps `xml:"bgPr,omitempty"`
	BgRef  *dml.FillRef    `xml:"bgRef,omitempty"`
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
	Sp           []*Shape            `xml:"-"`
	Pic          []*Picture          `xml:"-"`
	GraphicFrame []*GraphicFrame     `xml:"-"`
	GrpSp        []*GroupShape       `xml:"-"`
	CxnSp        []*ConnectionShape  `xml:"-"`
	childOrder   []childRef          // tracks interleaved child order
}

// childKind identifies a shape child element type.
type childKind int

const (
	childSp childKind = iota
	childPic
	childGraphicFrame
	childGrpSp
	childCxnSp
)

// childRef references a child element by kind and index into its typed slice.
type childRef struct {
	kind  childKind
	index int
}

// ClearChildOrder removes the child order tracking (used when shapes are rebuilt programmatically).
func (st *ShapeTree) ClearChildOrder() {
	st.childOrder = nil
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
				st.childOrder = append(st.childOrder, childRef{childSp, len(st.Sp)})
				st.Sp = append(st.Sp, sp)
			case "pic":
				pic := &Picture{}
				if err := d.DecodeElement(pic, &t); err != nil {
					return err
				}
				st.childOrder = append(st.childOrder, childRef{childPic, len(st.Pic)})
				st.Pic = append(st.Pic, pic)
			case "graphicFrame":
				gf := &GraphicFrame{}
				if err := d.DecodeElement(gf, &t); err != nil {
					return err
				}
				st.childOrder = append(st.childOrder, childRef{childGraphicFrame, len(st.GraphicFrame)})
				st.GraphicFrame = append(st.GraphicFrame, gf)
			case "grpSp":
				gs := &GroupShape{}
				if err := d.DecodeElement(gs, &t); err != nil {
					return err
				}
				st.childOrder = append(st.childOrder, childRef{childGrpSp, len(st.GrpSp)})
				st.GrpSp = append(st.GrpSp, gs)
			case "cxnSp":
				cs := &ConnectionShape{}
				if err := d.DecodeElement(cs, &t); err != nil {
					return err
				}
				st.childOrder = append(st.childOrder, childRef{childCxnSp, len(st.CxnSp)})
				st.CxnSp = append(st.CxnSp, cs)
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
			switch ref.kind {
			case childSp:
				if ref.index < len(st.Sp) {
					b.MarshalElement(ns, "sp", st.Sp[ref.index])
				}
			case childPic:
				if ref.index < len(st.Pic) {
					b.MarshalElement(ns, "pic", st.Pic[ref.index])
				}
			case childGraphicFrame:
				if ref.index < len(st.GraphicFrame) {
					b.MarshalElement(ns, "graphicFrame", st.GraphicFrame[ref.index])
				}
			case childGrpSp:
				if ref.index < len(st.GrpSp) {
					b.MarshalElement(ns, "grpSp", st.GrpSp[ref.index])
				}
			case childCxnSp:
				if ref.index < len(st.CxnSp) {
					b.MarshalElement(ns, "cxnSp", st.CxnSp[ref.index])
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

