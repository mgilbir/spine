package oxml

import (
	"encoding/xml"

	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// ExtensionList contains PresentationML extension elements (p:extLst).
// It models both CT_ExtensionList and CT_ExtensionListModify from pml.xsd:
// Mod carries the latter's optional mod attribute and stays nil elsewhere.
type ExtensionList struct {
	Mod *bool       `xml:"mod,attr,omitempty"`
	Ext []Extension `xml:"ext"`
}

// Extension represents CT_Extension from pml.xsd.
// The XSD defines content as <xsd:any processContents="lax" minOccurs="0" maxOccurs="unbounded"/>.
// Known extension types are parsed into typed fields; unknown extensions use RawContent.
type Extension struct {
	URI string `xml:"uri,attr"`

	// p14 extensions (PowerPoint 2010)
	CreationId       *P14CreationId       `xml:"-"`
	ModId            *P14ModId            `xml:"-"`
	Media            *P14Media            `xml:"-"`
	ShowMediaCtrls   *P14ShowMediaCtrls   `xml:"-"`
	DefaultImageDpi  *P14DefaultImageDpi  `xml:"-"`
	DiscardImageEdit *P14DiscardImageEdit `xml:"-"`
	LaserClr         *P14LaserClr         `xml:"-"`
	SectionLst       *P14SectionLst       `xml:"-"`

	// p15 extensions (PowerPoint 2012)
	PresenceInfo          *P15PresenceInfo          `xml:"-"`
	SldGuideLst           *P15SldGuideLst           `xml:"-"`
	ChartTrackingRefBased *P15ChartTrackingRefBased `xml:"-"`

	// Fallback for unrecognized extensions (xsd:any)
	RawContent []byte `xml:"-"`

	// InlineNSDecls preserves xmlns declarations carried on the ext element
	// itself (e.g. <p:ext uri="..." xmlns:foo="urn:foo">). They are re-emitted
	// for unknown-URI extensions so prefixes used by RawContent stay bound.
	InlineNSDecls []xmlb.NSDecl `xml:"-"`
}

// --- p14 extensions (PowerPoint 2010) ---

// P14CreationId represents p14:creationId extension element.
type P14CreationId struct {
	Val uint32 `xml:"val,attr"`
	// CapturedAttrs preserves the verbatim source attribute list (xmlns
	// declarations interleaved with attributes, in source order); nil for
	// values built programmatically.
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
}

// UnmarshalXML captures the element's verbatim attribute list (leaf element).
func (v *P14CreationId) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias P14CreationId
	return d.DecodeElement((*alias)(v), &start)
}

// P14ModId represents p14:modId extension element.
type P14ModId struct {
	Val           uint32          `xml:"val,attr"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // see P14CreationId.CapturedAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (leaf element).
func (v *P14ModId) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias P14ModId
	return d.DecodeElement((*alias)(v), &start)
}

// P14Media represents the p14:media extension element, whose r:embed attribute
// references the embedded media part of a video or audio p:pic.
type P14Media struct {
	Embed string // r:embed relationship ID
	Link  string // r:link relationship ID (externally linked media)
	// CapturedAttrs preserves the verbatim source attribute list (r:link,
	// xmlns="" resets, declaration order); replayed on marshal.
	CapturedAttrs []xmlb.RootAttr
	// RawContent preserves child elements (p14:trim, p14:fade, p14:bmkLst)
	// verbatim so trim/fade points and bookmarks survive re-marshaling.
	// Empty for media constructed programmatically.
	RawContent []byte
}

// UnmarshalXML captures the media element's verbatim attribute list and raw
// content.
func (m *P14Media) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	m.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	for _, attr := range start.Attr {
		if attr.Name.Space != xmlb.NSOfficeDocumentRels {
			continue
		}
		switch attr.Name.Local {
		case "embed":
			m.Embed = attr.Value
		case "link":
			m.Link = attr.Value
		}
	}
	var inner struct {
		Content []byte `xml:",innerxml"`
	}
	if err := d.DecodeElement(&inner, &start); err != nil {
		return err
	}
	m.RawContent = inner.Content
	return nil
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

// P14SectionLst represents the p14:sectionLst extension element, which holds
// the slide sections shown in PowerPoint's thumbnail pane. Each section names
// an ordered list of member slide ids (p14:sldId, referencing p:sldId/@id).
//
// For byte-faithful round-tripping, the source element's exact bytes are kept
// in raw and replayed verbatim on save; the typed Section model is regenerated
// only after a mutation marks the list Dirty. A programmatically built list
// (raw nil) always regenerates.
type P14SectionLst struct {
	Section []*P14Section
	raw     []byte
	dirty   bool
}

// Dirty reports whether the section list was mutated since it was parsed and so
// must be regenerated (rather than replayed verbatim) on save.
func (sl *P14SectionLst) Dirty() bool { return sl.dirty }

// MarkDirty flags the list for regeneration on the next save.
func (sl *P14SectionLst) MarkDirty() { sl.dirty = true }

// P14Section represents a single p14:section (a named group of slides).
type P14Section struct {
	Name string
	// ID is the section GUID in PowerPoint's brace-wrapped uppercase form,
	// e.g. "{124617AE-E5F0-462F-B980-77B306D58FBF}".
	ID string
	// SldId lists the member slide ids in display order. Each value matches a
	// p:sldId/@id in the presentation's sldIdLst.
	SldId []uint32
}

// parseSectionLst decodes the verbatim p14:sectionLst bytes into the typed
// model, keeping the raw bytes for byte-faithful replay of an unmodified list.
func parseSectionLst(raw []byte) (*P14SectionLst, error) {
	var w struct {
		Section []struct {
			Name     string `xml:"name,attr"`
			ID       string `xml:"id,attr"`
			SldIdLst struct {
				SldId []struct {
					ID uint32 `xml:"id,attr"`
				} `xml:"http://schemas.microsoft.com/office/powerpoint/2010/main sldId"`
			} `xml:"http://schemas.microsoft.com/office/powerpoint/2010/main sldIdLst"`
		} `xml:"http://schemas.microsoft.com/office/powerpoint/2010/main section"`
	}
	if err := xml.Unmarshal(raw, &w); err != nil {
		return nil, err
	}
	sl := &P14SectionLst{raw: raw}
	for _, s := range w.Section {
		sec := &P14Section{Name: s.Name, ID: s.ID}
		for _, sid := range s.SldIdLst.SldId {
			sec.SldId = append(sec.SldId, sid.ID)
		}
		sl.Section = append(sl.Section, sec)
	}
	return sl, nil
}

// --- p15 extensions (PowerPoint 2012) ---

// P15PresenceInfo represents p15:presenceInfo extension element.
type P15PresenceInfo struct {
	UserId     string `xml:"userId,attr,omitempty"`
	ProviderId string `xml:"providerId,attr,omitempty"`
}

// P15SldGuideLst represents p15:sldGuideLst extension element.
type P15SldGuideLst struct {
	Guide         []*P15Guide     `xml:"http://schemas.microsoft.com/office/powerpoint/2012/main guide,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // see P14CreationId.CapturedAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (some producers
// carry xmlns="" alongside xmlns:p15) before decoding the guide children.
func (v *P15SldGuideLst) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias P15SldGuideLst
	return d.DecodeElement((*alias)(v), &start)
}

// P15Guide represents p15:guide element.
type P15Guide struct {
	Id        string  `xml:"id,attr,omitempty"`
	Orient    string  `xml:"orient,attr,omitempty"`
	Pos       string  `xml:"pos,attr,omitempty"`
	UserDrawn string  `xml:"userDrawn,attr,omitempty"`
	Clr       *P15Clr `xml:"http://schemas.microsoft.com/office/powerpoint/2012/main clr,omitempty"`
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
	var nsDecls []xmlb.NSDecl
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "uri" && attr.Name.Space == "":
			e.URI = attr.Value
		case attr.Name.Space == "xmlns":
			nsDecls = append(nsDecls, xmlb.NSDecl{Prefix: attr.Name.Local, URI: attr.Value})
		case attr.Name.Space == "" && attr.Name.Local == "xmlns":
			nsDecls = append(nsDecls, xmlb.NSDecl{URI: attr.Value})
		}
	}
	// Keep ext-level declarations for typed content too: marshal replays them
	// on the p:ext element so a source that declared the extension prefix
	// there (instead of on the child) round-trips.
	e.InlineNSDecls = nsDecls

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
			V P14Media `xml:"http://schemas.microsoft.com/office/powerpoint/2010/main media"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		e.Media = &w.V

	case xmlb.ExtURIShowMediaCtrls:
		var w struct {
			V P14ShowMediaCtrls `xml:"http://schemas.microsoft.com/office/powerpoint/2010/main showMediaCtrls"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		// Lenient: an out-of-lexical-space xsd:boolean (e.g. a wild "N") must not
		// abort Open. Val is preserved verbatim and re-emitted as-is (C355).
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
		// Lenient xsd:boolean; see ShowMediaCtrls above (C355).
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
		// Lenient xsd:boolean; see ShowMediaCtrls above (C355).
		e.ChartTrackingRefBased = &w.V

	case xmlb.ExtURISectionLst:
		// Capture the verbatim sectionLst bytes (for byte-faithful replay of
		// an unmodified list) and parse the typed model from them.
		var inner struct {
			Content []byte `xml:",innerxml"`
		}
		if err := d.DecodeElement(&inner, &start); err != nil {
			return err
		}
		sl, err := parseSectionLst(inner.Content)
		if err != nil {
			return err
		}
		e.SectionLst = sl

	default:
		// Unknown extension - preserve raw bytes along with any xmlns
		// declarations the ext element carried, so re-emitted content
		// keeps its prefixes bound.
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
	// Captured ext-level declarations are replayed for typed content too: a
	// source that declared the extension prefix on the p:ext element (rather
	// than on the child) must get it back in the same place. The typed child
	// only synthesizes its own declaration when it carries no capture.
	attrs := xmlb.NSDeclAttrs([]xmlb.Attr{xmlb.StrAttr("uri", e.URI)}, e.InlineNSDecls)
	b.StartElement(ns, localName, attrs...)

	switch {
	case e.CreationId != nil:
		if raw := e.CreationId.CapturedAttrs; raw != nil {
			b.EmptyElementLiteral(xmlb.RawAttrPrefix(raw, nsP14, xmlb.PrefixPowerPoint2010), "creationId",
				xmlb.RawAttrListOverride(raw, map[string]string{"val": xmlb.UintAttr("val", e.CreationId.Val).Value})...)
			break
		}
		b.EmptyElementInlineNS(nsP14, xmlb.PrefixPowerPoint2010, "creationId",
			xmlb.UintAttr("val", e.CreationId.Val))

	case e.ModId != nil:
		if raw := e.ModId.CapturedAttrs; raw != nil {
			b.EmptyElementLiteral(xmlb.RawAttrPrefix(raw, nsP14, xmlb.PrefixPowerPoint2010), "modId",
				xmlb.RawAttrListOverride(raw, map[string]string{"val": xmlb.UintAttr("val", e.ModId.Val).Value})...)
			break
		}
		b.EmptyElementInlineNS(nsP14, xmlb.PrefixPowerPoint2010, "modId",
			xmlb.UintAttr("val", e.ModId.Val))

	case e.Media != nil:
		if raw := e.Media.CapturedAttrs; raw != nil {
			// Verbatim replay: keeps r:link references, xmlns="" resets, and
			// the source's declaration placement; a mutated r:embed value
			// still wins via the override.
			override := map[string]string{}
			if e.Media.Embed != "" {
				override["r:embed"] = e.Media.Embed
			}
			if e.Media.Link != "" {
				override["r:link"] = e.Media.Link
			}
			prefix := xmlb.RawAttrPrefix(raw, nsP14, xmlb.PrefixPowerPoint2010)
			if len(e.Media.RawContent) > 0 {
				b.StartElementLiteral(prefix, "media", nil, xmlb.RawAttrListOverride(raw, override)...)
				b.WriteRaw(e.Media.RawContent)
				b.EndElementLiteral(prefix, "media")
			} else {
				b.EmptyElementLiteral(prefix, "media", xmlb.RawAttrListOverride(raw, override)...)
			}
			break
		}
		// Programmatic media (no captured attrs): emit r:embed and, when set,
		// r:link (externally linked media) — the latter was previously dropped
		// (C355).
		mediaAttrs := []xmlb.Attr{xmlb.RelAttr("embed", e.Media.Embed)}
		if e.Media.Link != "" {
			mediaAttrs = append(mediaAttrs, xmlb.RelAttr("link", e.Media.Link))
		}
		if len(e.Media.RawContent) > 0 {
			b.StartElementInlineNS(nsP14, xmlb.PrefixPowerPoint2010, "media", mediaAttrs...)
			b.WriteRaw(e.Media.RawContent)
			b.EndElementInlineNS(xmlb.PrefixPowerPoint2010, "media")
			b.ResetNamespaceDeclaration(nsP14)
		} else {
			b.EmptyElementInlineNS(nsP14, xmlb.PrefixPowerPoint2010, "media", mediaAttrs...)
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

	case e.SectionLst != nil:
		marshalSectionLst(b, e.SectionLst)

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
	if raw := g.CapturedAttrs; raw != nil {
		prefix := xmlb.RawAttrPrefix(raw, nsP15, xmlb.PrefixPowerPoint2012)
		if len(g.Guide) == 0 {
			b.EmptyElementLiteral(prefix, "sldGuideLst", xmlb.RawAttrList(raw)...)
			return
		}
		b.StartElementLiteral(prefix, "sldGuideLst",
			[]xmlb.NSDecl{{Prefix: prefix, URI: nsP15}}, xmlb.RawAttrList(raw)...)
		marshalSldGuides(b, g)
		b.EndElementLiteral(prefix, "sldGuideLst")
		return
	}
	if len(g.Guide) == 0 {
		b.EmptyElementInlineNS(nsP15, xmlb.PrefixPowerPoint2012, "sldGuideLst")
		return
	}
	b.StartElementInlineNS(nsP15, xmlb.PrefixPowerPoint2012, "sldGuideLst")
	marshalSldGuides(b, g)
	b.EndElementInlineNS(xmlb.PrefixPowerPoint2012, "sldGuideLst")
	b.ResetNamespaceDeclaration(nsP15)
}

// marshalSectionLst writes the p14:sectionLst extension. An unmodified list
// replays its source bytes verbatim (byte-identical round-trip); a mutated or
// programmatically built list regenerates the canonical PowerPoint form.
func marshalSectionLst(b *xmlb.Builder, sl *P14SectionLst) {
	if !sl.dirty && len(sl.raw) > 0 {
		b.WriteRaw(sl.raw)
		return
	}
	prefix := xmlb.PrefixPowerPoint2010
	if len(sl.Section) == 0 {
		b.EmptyElementInlineNS(nsP14, prefix, "sectionLst")
		return
	}
	b.StartElementInlineNS(nsP14, prefix, "sectionLst")
	for _, s := range sl.Section {
		b.StartElement(nsP14, "section",
			xmlb.StrAttr("name", s.Name), xmlb.StrAttr("id", s.ID))
		if len(s.SldId) == 0 {
			b.EmptyElement(nsP14, "sldIdLst")
		} else {
			b.StartElement(nsP14, "sldIdLst")
			for _, id := range s.SldId {
				b.EmptyElement(nsP14, "sldId", xmlb.UintAttr("id", id))
			}
			b.EndElement(nsP14, "sldIdLst")
		}
		b.EndElement(nsP14, "section")
	}
	b.EndElementInlineNS(prefix, "sectionLst")
	b.ResetNamespaceDeclaration(nsP14)
}

// marshalSldGuides writes the p15:guide children of a sldGuideLst.
func marshalSldGuides(b *xmlb.Builder, g *P15SldGuideLst) {
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
}
