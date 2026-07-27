package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_Styles is the root element of the styles part (w:styles).
type CT_Styles struct {
	XMLName      xml.Name         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main styles"`
	DocDefaults  *CT_DocDefaults  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main docDefaults,omitempty"`
	LatentStyles *CT_LatentStyles `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main latentStyles,omitempty"`
	Style        []*CT_Style      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main style"`
	// OriginalNSDecls and OriginalRootAttrs preserve the source root's namespace
	// declarations and verbatim attribute list. Without them a regenerated
	// styles.xml replayed a fixed three-declaration set, dropping mc:Ignorable
	// and every extension declaration (xmlns:w14, …) the captured children below
	// need in order to resolve their prefixes (C370). Nil for a part created
	// from scratch, which gets the standard declaration set.
	OriginalNSDecls   []xmlb.NSDecl   `xml:"-"`
	OriginalRootAttrs []xmlb.RootAttr `xml:"-"`
	// CapturedChildren records the source child sequence, including root-level
	// children the model does not type.
	CapturedChildren *xmlb.ChildCapture `xml:"-"`
}

// UnmarshalXML captures the root's namespace declarations and verbatim
// attribute list for round-trip fidelity, then decodes the children through the
// ordered-children capture so unmodeled root children survive.
func (s *CT_Styles) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	s.XMLName = start.Name
	s.OriginalRootAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	s.OriginalNSDecls = captureRootNSDecls(start.Attr)
	return xmlb.UnmarshalOrderedChildren(d, s)
}

// CT_DocDefaults represents document-wide default properties.
type CT_DocDefaults struct {
	RPrDefault *CT_RPrDefault `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rPrDefault,omitempty"`
	PPrDefault *CT_PPrDefault `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pPrDefault,omitempty"`
}

// CT_RPrDefault represents default run properties.
type CT_RPrDefault struct {
	RPr *CT_RPr `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rPr,omitempty"`
}

// CT_PPrDefault represents default paragraph properties.
type CT_PPrDefault struct {
	PPr *CT_PPr `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pPr,omitempty"`
}

// CT_Style represents a single style definition.
type CT_Style struct {
	Type            string            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main type,attr,omitempty"`
	StyleId         string            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main styleId,attr,omitempty"`
	Default         string            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main default,attr,omitempty"`
	CustomStyle     string            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main customStyle,attr,omitempty"`
	Name            *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main name,omitempty"`
	Aliases         *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main aliases,omitempty"`
	BasedOn         *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main basedOn,omitempty"`
	Next            *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main next,omitempty"`
	Link            *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main link,omitempty"`
	AutoRedefine    *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main autoRedefine,omitempty"`
	Hidden          *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hidden,omitempty"`
	UiPriority      *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main uiPriority,omitempty"`
	SemiHidden      *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main semiHidden,omitempty"`
	UnhideWhenUsed  *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main unhideWhenUsed,omitempty"`
	QFormat         *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main qFormat,omitempty"`
	Locked          *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main locked,omitempty"`
	Personal        *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main personal,omitempty"`
	PersonalCompose *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main personalCompose,omitempty"`
	PersonalReply   *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main personalReply,omitempty"`
	RsId            *CT_LongHexNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsid,omitempty"`
	PPr             *CT_PPr           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pPr,omitempty"`
	RPr             *CT_RPr           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rPr,omitempty"`
	TblPr           *CT_TblPr         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblPr,omitempty"`
	TrPr            *CT_TrPr          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main trPr,omitempty"`
	TcPr            *CT_TcPr          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tcPr,omitempty"`
	TblStylePr      []*CT_TblStylePr  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblStylePr,omitempty"`
}

// CT_TblStylePr represents table style conditional formatting.
type CT_TblStylePr struct {
	Type  string    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main type,attr"`
	PPr   *CT_PPr   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pPr,omitempty"`
	RPr   *CT_RPr   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rPr,omitempty"`
	TblPr *CT_TblPr `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblPr,omitempty"`
	TrPr  *CT_TrPr  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main trPr,omitempty"`
	TcPr  *CT_TcPr  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tcPr,omitempty"`
}

// CT_LatentStyles represents latent style definitions.
type CT_LatentStyles struct {
	DefLockedState    string             `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main defLockedState,attr,omitempty"`
	DefUIPriority     string             `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main defUIPriority,attr,omitempty"`
	DefSemiHidden     string             `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main defSemiHidden,attr,omitempty"`
	DefUnhideWhenUsed string             `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main defUnhideWhenUsed,attr,omitempty"`
	DefQFormat        string             `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main defQFormat,attr,omitempty"`
	Count             string             `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main count,attr,omitempty"`
	LsdException      []*CT_LsdException `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lsdException"`
}

// CT_LsdException represents a latent style exception.
type CT_LsdException struct {
	Name           string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main name,attr"`
	Locked         string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main locked,attr,omitempty"`
	UiPriority     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main uiPriority,attr,omitempty"`
	SemiHidden     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main semiHidden,attr,omitempty"`
	UnhideWhenUsed string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main unhideWhenUsed,attr,omitempty"`
	QFormat        string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main qFormat,attr,omitempty"`
}
