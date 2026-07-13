package oxml

import (
	"encoding/xml"
	"fmt"

	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// ExtensionList contains PresentationML extension elements (p:extLst).
type ExtensionList struct {
	Ext []Extension `xml:"ext"`
}

// Extension represents CT_Extension from pml.xsd.
// The XSD defines content as <xsd:any processContents="lax" minOccurs="0" maxOccurs="unbounded"/>.
// Known extension types are parsed into typed fields; unknown extensions use RawContent.
type Extension struct {
	URI string `xml:"uri,attr"`

	// p14 extensions (PowerPoint 2010)
	CreationId        *P14CreationId        `xml:"-"`
	ModId             *P14ModId             `xml:"-"`
	Media             *P14Media             `xml:"-"`
	ShowMediaCtrls    *P14ShowMediaCtrls    `xml:"-"`
	DefaultImageDpi   *P14DefaultImageDpi   `xml:"-"`
	DiscardImageEdit  *P14DiscardImageEdit  `xml:"-"`
	LaserClr          *P14LaserClr          `xml:"-"`

	// p15 extensions (PowerPoint 2012)
	PresenceInfo          *P15PresenceInfo          `xml:"-"`
	SldGuideLst           *P15SldGuideLst           `xml:"-"`
	ChartTrackingRefBased *P15ChartTrackingRefBased `xml:"-"`

	// Fallback for unrecognized extensions (xsd:any)
	RawContent []byte `xml:"-"`
}

// --- p14 extensions (PowerPoint 2010) ---

// P14CreationId represents p14:creationId extension element.
type P14CreationId struct {
	Val uint32 `xml:"val,attr"`
}

// P14ModId represents p14:modId extension element.
type P14ModId struct {
	Val uint32 `xml:"val,attr"`
}

// P14Media represents the p14:media extension element, whose r:embed attribute
// references the embedded media part of a video or audio p:pic.
type P14Media struct {
	Embed string // r:embed relationship ID
	// RawContent preserves child elements (p14:trim, p14:fade, p14:bmkLst)
	// verbatim so trim/fade points and bookmarks survive re-marshaling.
	// Empty for media constructed programmatically.
	RawContent []byte
}

// P14ShowMediaCtrls represents p14:showMediaCtrls extension element.
// Val holds the original xsd:boolean lexical form ("0", "1", "true", "false")
// so it round-trips byte-faithfully; empty means the attribute was absent.
type P14ShowMediaCtrls struct {
	Val string `xml:"val,attr,omitempty"`
}

// P14DefaultImageDpi represents p14:defaultImageDpi extension element.
type P14DefaultImageDpi struct {
	Val *int32 `xml:"val,attr,omitempty"`
}

// P14DiscardImageEdit represents p14:discardImageEditData extension element.
// Val holds the original xsd:boolean lexical form; see P14ShowMediaCtrls.
type P14DiscardImageEdit struct {
	Val string `xml:"val,attr,omitempty"`
}

// P14LaserClr represents p14:laserClr extension element.
// Contains a DML color choice (a:srgbClr, a:schemeClr, etc.).
type P14LaserClr struct {
	dml.ColorChoice
}

// --- p15 extensions (PowerPoint 2012) ---

// P15PresenceInfo represents p15:presenceInfo extension element.
type P15PresenceInfo struct {
	UserId     string `xml:"userId,attr,omitempty"`
	ProviderId string `xml:"providerId,attr,omitempty"`
}

// P15SldGuideLst represents p15:sldGuideLst extension element.
type P15SldGuideLst struct {
	Guide []*P15Guide `xml:"http://schemas.microsoft.com/office/powerpoint/2012/main guide,omitempty"`
}

// P15Guide represents p15:guide element.
type P15Guide struct {
	Id        string    `xml:"id,attr,omitempty"`
	Orient    string    `xml:"orient,attr,omitempty"`
	Pos       string    `xml:"pos,attr,omitempty"`
	UserDrawn string    `xml:"userDrawn,attr,omitempty"`
	Clr       *P15Clr   `xml:"http://schemas.microsoft.com/office/powerpoint/2012/main clr,omitempty"`
}

// P15Clr represents p15:clr element (contains a DML color choice).
type P15Clr struct {
	dml.ColorChoice
}

// P15ChartTrackingRefBased represents p15:chartTrackingRefBased extension element.
// Val holds the original xsd:boolean lexical form; see P14ShowMediaCtrls.
type P15ChartTrackingRefBased struct {
	Val string `xml:"val,attr,omitempty"`
}

// --- Custom UnmarshalXML for Extension ---

const (
	nsP14 = xmlb.NSPowerPoint2010
	nsP15 = xmlb.NSPowerPoint2012
)

func (e *Extension) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Local == "uri" {
			e.URI = attr.Value
		}
	}

	switch e.URI {
	case xmlb.ExtURIPMLCreationId:
		var w struct {
			V P14CreationId `xml:"http://schemas.microsoft.com/office/powerpoint/2010/main creationId"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		e.CreationId = &w.V

	case xmlb.ExtURIPMLModId:
		var w struct {
			V P14ModId `xml:"http://schemas.microsoft.com/office/powerpoint/2010/main modId"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		e.ModId = &w.V

	case xmlb.ExtURIMedia:
		var w struct {
			V struct {
				Embed   string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships embed,attr"`
				Content []byte `xml:",innerxml"`
			} `xml:"http://schemas.microsoft.com/office/powerpoint/2010/main media"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		e.Media = &P14Media{Embed: w.V.Embed, RawContent: w.V.Content}

	case xmlb.ExtURIShowMediaCtrls:
		var w struct {
			V P14ShowMediaCtrls `xml:"http://schemas.microsoft.com/office/powerpoint/2010/main showMediaCtrls"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		if err := validateXSDBoolean("p14:showMediaCtrls", w.V.Val); err != nil {
			return err
		}
		e.ShowMediaCtrls = &w.V

	case xmlb.ExtURIDefaultImageDpi:
		var w struct {
			V P14DefaultImageDpi `xml:"http://schemas.microsoft.com/office/powerpoint/2010/main defaultImageDpi"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		e.DefaultImageDpi = &w.V

	case xmlb.ExtURIDiscardImageEditData:
		var w struct {
			V P14DiscardImageEdit `xml:"http://schemas.microsoft.com/office/powerpoint/2010/main discardImageEditData"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		if err := validateXSDBoolean("p14:discardImageEditData", w.V.Val); err != nil {
			return err
		}
		e.DiscardImageEdit = &w.V

	case xmlb.ExtURILaserClr:
		var w struct {
			V P14LaserClr `xml:"http://schemas.microsoft.com/office/powerpoint/2010/main laserClr"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		e.LaserClr = &w.V

	case xmlb.ExtURIPresenceInfo:
		var w struct {
			V P15PresenceInfo `xml:"http://schemas.microsoft.com/office/powerpoint/2012/main presenceInfo"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		e.PresenceInfo = &w.V

	case xmlb.ExtURISldGuideLst, xmlb.ExtURISldGuideLstMaster, xmlb.ExtURISldGuideLstLayout:
		var w struct {
			V P15SldGuideLst `xml:"http://schemas.microsoft.com/office/powerpoint/2012/main sldGuideLst"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		e.SldGuideLst = &w.V

	case xmlb.ExtURIChartTrackingRefBased:
		var w struct {
			V P15ChartTrackingRefBased `xml:"http://schemas.microsoft.com/office/powerpoint/2012/main chartTrackingRefBased"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		if err := validateXSDBoolean("p15:chartTrackingRefBased", w.V.Val); err != nil {
			return err
		}
		e.ChartTrackingRefBased = &w.V

	default:
		// Unknown extension - preserve raw bytes
		var inner struct {
			Content []byte `xml:",innerxml"`
		}
		if err := d.DecodeElement(&inner, &start); err != nil {
			return err
		}
		e.RawContent = inner.Content
	}

	return nil
}

// --- MarshalToBuilder for Extension ---

func (e *Extension) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName, xmlb.StrAttr("uri", e.URI))

	switch {
	case e.CreationId != nil:
		b.EmptyElementInlineNS(nsP14, xmlb.PrefixPowerPoint2010, "creationId",
			xmlb.UintAttr("val", e.CreationId.Val))

	case e.ModId != nil:
		b.EmptyElementInlineNS(nsP14, xmlb.PrefixPowerPoint2010, "modId",
			xmlb.UintAttr("val", e.ModId.Val))

	case e.Media != nil:
		if len(e.Media.RawContent) > 0 {
			b.StartElementInlineNS(nsP14, xmlb.PrefixPowerPoint2010, "media",
				xmlb.RelAttr("embed", e.Media.Embed))
			b.WriteRaw(e.Media.RawContent)
			b.EndElementInlineNS(xmlb.PrefixPowerPoint2010, "media")
			b.ResetNamespaceDeclaration(nsP14)
		} else {
			b.EmptyElementInlineNS(nsP14, xmlb.PrefixPowerPoint2010, "media",
				xmlb.RelAttr("embed", e.Media.Embed))
		}

	case e.ShowMediaCtrls != nil:
		marshalP14Bool(b, "showMediaCtrls", e.ShowMediaCtrls.Val)

	case e.DefaultImageDpi != nil:
		marshalP14Simple(b, "defaultImageDpi", e.DefaultImageDpi.Val)

	case e.DiscardImageEdit != nil:
		marshalP14Bool(b, "discardImageEditData", e.DiscardImageEdit.Val)

	case e.LaserClr != nil:
		b.StartElementInlineNS(nsP14, xmlb.PrefixPowerPoint2010, "laserClr")
		marshalColorChoice(b, &e.LaserClr.ColorChoice)
		b.EndElementInlineNS(xmlb.PrefixPowerPoint2010, "laserClr")
		b.ResetNamespaceDeclaration(nsP14)

	case e.PresenceInfo != nil:
		var attrs []xmlb.Attr
		if e.PresenceInfo.UserId != "" {
			attrs = append(attrs, xmlb.StrAttr("userId", e.PresenceInfo.UserId))
		}
		if e.PresenceInfo.ProviderId != "" {
			attrs = append(attrs, xmlb.StrAttr("providerId", e.PresenceInfo.ProviderId))
		}
		b.EmptyElementInlineNS(nsP15, xmlb.PrefixPowerPoint2012, "presenceInfo", attrs...)

	case e.SldGuideLst != nil:
		marshalSldGuideLst(b, e.SldGuideLst)

	case e.ChartTrackingRefBased != nil:
		marshalP15Bool(b, "chartTrackingRefBased", e.ChartTrackingRefBased.Val)

	default:
		if len(e.RawContent) > 0 {
			b.WriteRaw(e.RawContent)
		}
	}

	b.EndElement(ns, localName)
}

// marshalP14Simple writes a simple p14 extension element with an optional val attribute.
func marshalP14Simple(b *xmlb.Builder, localName string, val *int32) {
	if val != nil {
		b.EmptyElementInlineNS(nsP14, xmlb.PrefixPowerPoint2010, localName,
			xmlb.Int32Attr("val", *val))
	} else {
		b.EmptyElementInlineNS(nsP14, xmlb.PrefixPowerPoint2010, localName)
	}
}

// marshalP14Bool writes a p14 extension element with an optional xsd:boolean
// val attribute, re-emitting the original lexical form verbatim.
func marshalP14Bool(b *xmlb.Builder, localName, val string) {
	if val != "" {
		b.EmptyElementInlineNS(nsP14, xmlb.PrefixPowerPoint2010, localName,
			xmlb.StrAttr("val", val))
	} else {
		b.EmptyElementInlineNS(nsP14, xmlb.PrefixPowerPoint2010, localName)
	}
}

// marshalP15Bool writes a p15 extension element with an optional xsd:boolean
// val attribute, re-emitting the original lexical form verbatim.
func marshalP15Bool(b *xmlb.Builder, localName, val string) {
	if val != "" {
		b.EmptyElementInlineNS(nsP15, xmlb.PrefixPowerPoint2012, localName,
			xmlb.StrAttr("val", val))
	} else {
		b.EmptyElementInlineNS(nsP15, xmlb.PrefixPowerPoint2012, localName)
	}
}

// validateXSDBoolean checks that val is within the xsd:boolean lexical space
// ("0", "1", "true", "false") or absent (empty string).
func validateXSDBoolean(elem, val string) error {
	switch val {
	case "", "0", "1", "true", "false":
		return nil
	}
	return fmt.Errorf("%s: invalid xsd:boolean value %q for val attribute", elem, val)
}

// marshalColorChoice writes a DML color choice (a:srgbClr, a:schemeClr, etc.).
func marshalColorChoice(b *xmlb.Builder, cc *dml.ColorChoice) {
	nsA := xmlb.NSDrawingML
	if cc.SrgbClr != nil {
		b.MarshalElement(nsA, "srgbClr", cc.SrgbClr)
	} else if cc.SchemeClr != nil {
		b.MarshalElement(nsA, "schemeClr", cc.SchemeClr)
	} else if cc.SysClr != nil {
		b.MarshalElement(nsA, "sysClr", cc.SysClr)
	} else if cc.PrstClr != nil {
		b.MarshalElement(nsA, "prstClr", cc.PrstClr)
	} else if cc.HslClr != nil {
		b.MarshalElement(nsA, "hslClr", cc.HslClr)
	} else if cc.ScrgbClr != nil {
		b.MarshalElement(nsA, "scrgbClr", cc.ScrgbClr)
	}
}

// marshalSldGuideLst writes p15:sldGuideLst element.
func marshalSldGuideLst(b *xmlb.Builder, g *P15SldGuideLst) {
	if len(g.Guide) == 0 {
		b.EmptyElementInlineNS(nsP15, xmlb.PrefixPowerPoint2012, "sldGuideLst")
		return
	}
	b.StartElementInlineNS(nsP15, xmlb.PrefixPowerPoint2012, "sldGuideLst")
	for _, guide := range g.Guide {
		var attrs []xmlb.Attr
		if guide.Id != "" {
			attrs = append(attrs, xmlb.StrAttr("id", guide.Id))
		}
		if guide.Orient != "" {
			attrs = append(attrs, xmlb.StrAttr("orient", guide.Orient))
		}
		if guide.Pos != "" {
			attrs = append(attrs, xmlb.StrAttr("pos", guide.Pos))
		}
		if guide.UserDrawn != "" {
			attrs = append(attrs, xmlb.StrAttr("userDrawn", guide.UserDrawn))
		}
		if guide.Clr != nil {
			b.StartElement(nsP15, "guide", attrs...)
			b.StartElement(nsP15, "clr")
			marshalColorChoice(b, &guide.Clr.ColorChoice)
			b.EndElement(nsP15, "clr")
			b.EndElement(nsP15, "guide")
		} else {
			b.EmptyElement(nsP15, "guide", attrs...)
		}
	}
	b.EndElementInlineNS(xmlb.PrefixPowerPoint2012, "sldGuideLst")
	b.ResetNamespaceDeclaration(nsP15)
}
