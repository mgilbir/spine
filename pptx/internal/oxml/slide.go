package oxml

import (
	"encoding/xml"
	"fmt"
)

// Slide is the root element of a slide part.
type Slide struct {
	XMLName xml.Name  `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sld"`
	CSld    *CommonSlideData `xml:"cSld"`
	ClrMapOvr *ColorMapOverride `xml:"clrMapOvr,omitempty"`
}

// SlideLayout is the root element of a slide layout part.
type SlideLayout struct {
	XMLName     xml.Name         `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sldLayout"`
	Type        string           `xml:"type,attr,omitempty"`
	Preserve    bool             `xml:"preserve,attr,omitempty"`
	UserDrawn   bool             `xml:"userDrawn,attr,omitempty"`
	MatchingName string          `xml:"matchingName,attr,omitempty"`
	CSld        *CommonSlideData `xml:"cSld"`
	ClrMapOvr   *ColorMapOverride `xml:"clrMapOvr,omitempty"`
}

// SlideMaster is the root element of a slide master part.
type SlideMaster struct {
	XMLName        xml.Name         `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sldMaster"`
	XmlnsR         string           `xml:"xmlns:r,attr,omitempty"`
	Preserve       bool             `xml:"preserve,attr,omitempty"`
	CSld           *CommonSlideData `xml:"cSld"`
	ClrMap         *ColorMap        `xml:"clrMap,omitempty"`
	SlideLayoutIDs *SlideLayoutIDs  `xml:"sldLayoutIdLst,omitempty"`
	TxStyles       *TxStyles        `xml:"txStyles,omitempty"`
}

// SlideLayoutIDs contains a list of slide layout ID references.
type SlideLayoutIDs struct {
	SlideLayoutID []SlideLayoutID `xml:"sldLayoutId"`
}

// SlideLayoutID references a slide layout.
type SlideLayoutID struct {
	ID  uint32 `xml:"id,attr,omitempty"`
	RID string `xml:"-"`
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
	Name    string    `xml:"name,attr,omitempty"`
	SpTree  *ShapeTree `xml:"spTree"`
	ExtLst  *ExtensionList `xml:"extLst,omitempty"`
}

// ShapeTree is the container for shapes on a slide.
type ShapeTree struct {
	NvGrpSpPr     *NonVisualGroupShapeProperties `xml:"nvGrpSpPr"`
	GrpSpPr       *GroupShapeProperties          `xml:"grpSpPr"`
	Sp            []*Shape                        `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sp,omitempty"`
	Pic           []*Picture                      `xml:"http://schemas.openxmlformats.org/presentationml/2006/main pic,omitempty"`
	GraphicFrame  []*GraphicFrame                 `xml:"http://schemas.openxmlformats.org/presentationml/2006/main graphicFrame,omitempty"`
	GrpSp         []*GroupShape                   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main grpSp,omitempty"`
	CxnSp         []*ConnectionShape              `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cxnSp,omitempty"`
}

// NonVisualGroupShapeProperties contains non-visual properties for a group.
type NonVisualGroupShapeProperties struct {
	CNvPr      *NonVisualDrawingProperties `xml:"cNvPr"`
	CNvGrpSpPr *NonVisualGroupShapeDrawingProperties `xml:"cNvGrpSpPr"`
	NvPr       *ApplicationNonVisualDrawingProperties `xml:"nvPr"`
}

// NonVisualDrawingProperties contains non-visual drawing properties.
type NonVisualDrawingProperties struct {
	ID    uint32 `xml:"id,attr"`
	Name  string `xml:"name,attr"`
	Descr string `xml:"descr,attr,omitempty"`
	Title string `xml:"title,attr,omitempty"`
}

// NonVisualGroupShapeDrawingProperties contains non-visual group shape properties.
type NonVisualGroupShapeDrawingProperties struct {
	// Empty for now; can contain grpSpLocks
}

// ApplicationNonVisualDrawingProperties contains application-specific non-visual properties.
type ApplicationNonVisualDrawingProperties struct {
	IsPhoto   bool `xml:"isPhoto,attr,omitempty"`
	UserDrawn bool `xml:"userDrawn,attr,omitempty"`
	Ph        *Placeholder `xml:"ph,omitempty"`
}

// GroupShapeProperties contains visual properties for a group.
type GroupShapeProperties struct {
	Xfrm *GroupTransform2D `xml:"xfrm,omitempty"`
}

// GroupTransform2D specifies the transform for a group.
type GroupTransform2D struct {
	Off    *Offset2D `xml:"off,omitempty"`
	Ext    *Extent2D `xml:"ext,omitempty"`
	ChOff  *Offset2D `xml:"chOff,omitempty"`
	ChExt  *Extent2D `xml:"chExt,omitempty"`
}

// Offset2D specifies a 2D offset.
type Offset2D struct {
	X int64 `xml:"x,attr"`
	Y int64 `xml:"y,attr"`
}

// Extent2D specifies 2D extents.
type Extent2D struct {
	Cx int64 `xml:"cx,attr"`
	Cy int64 `xml:"cy,attr"`
}

// ShapeChoice is a placeholder for shape elements in the shape tree.
// Actual shapes are parsed separately.
type ShapeChoice struct {
	XMLName xml.Name
	Content []byte `xml:",innerxml"`
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
	MasterClrMapping *MasterColorMapping `xml:"masterClrMapping,omitempty"`
	OverrideClrMapping *ColorMap `xml:"overrideClrMapping,omitempty"`
}

// MasterColorMapping indicates to use the master's color mapping.
type MasterColorMapping struct{}

// Placeholder specifies placeholder information.
type Placeholder struct {
	Type       string `xml:"type,attr,omitempty"`
	Orient     string `xml:"orient,attr,omitempty"`
	Sz         string `xml:"sz,attr,omitempty"`
	Idx        uint32 `xml:"idx,attr,omitempty"`
	HasCustomPrompt bool `xml:"hasCustomPrompt,attr,omitempty"`
}

// TxStyles contains text styles for a slide master.
type TxStyles struct {
	TitleStyle *TextListStyle `xml:"titleStyle,omitempty"`
	BodyStyle  *TextListStyle `xml:"bodyStyle,omitempty"`
	OtherStyle *TextListStyle `xml:"otherStyle,omitempty"`
}

// ExtensionList contains extension elements.
type ExtensionList struct {
	Ext []Extension `xml:"ext"`
}

// Extension is a generic extension container.
type Extension struct {
	URI     string `xml:"uri,attr"`
	Content []byte `xml:",innerxml"`
}
