package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_SectPr represents section properties (w:sectPr).
type CT_SectPr struct {
	RsidR    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsidR,attr,omitempty"`
	RsidRPr  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsidRPr,attr,omitempty"`
	RsidSect string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsidSect,attr,omitempty"`

	HeaderReference []*CT_HdrFtrRef `xml:"-"`
	FooterReference []*CT_HdrFtrRef `xml:"-"`
	FootnoteProperties *CT_FtnProps `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main footnotePr,omitempty"`
	EndnoteProperties  *CT_EdnProps `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main endnotePr,omitempty"`
	Type        *CT_String      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main type,omitempty"`
	PgSz        *CT_PgSz        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pgSz,omitempty"`
	PgMar       *CT_PgMar       `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pgMar,omitempty"`
	PaperSrc    *CT_PaperSrc    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main paperSrc,omitempty"`
	PgBorders   *CT_PgBorders   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pgBorders,omitempty"`
	LnNumType   *CT_LnNumType   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lnNumType,omitempty"`
	PgNumType   *CT_PgNumType   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pgNumType,omitempty"`
	Cols        *CT_Columns     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cols,omitempty"`
	FormProt    *CT_OnOff       `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main formProt,omitempty"`
	VAlign      *CT_String      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main vAlign,omitempty"`
	NoEndnote   *CT_OnOff       `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main noEndnote,omitempty"`
	TitlePg     *CT_OnOff       `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main titlePg,omitempty"`
	TextDirection *CT_String    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main textDirection,omitempty"`
	Bidi        *CT_OnOff       `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bidi,omitempty"`
	RtlGutter   *CT_OnOff       `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rtlGutter,omitempty"`
	DocGrid     *CT_DocGrid     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main docGrid,omitempty"`
	// PrinterSettings (w:printerSettings, a CT_Rel carrying r:id) is preserved
	// raw: the model does not interpret it, but stripping it would orphan the
	// printer-settings part.
	PrinterSettings *CT_RawElement   `xml:"-"`
	SectPrChange    *CT_SectPrChange `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sectPrChange,omitempty"`
}

// UnmarshalXML implements custom unmarshaling for CT_SectPr to handle r:id attributes.
func (sp *CT_SectPr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "rsidR":
			sp.RsidR = attr.Value
		case "rsidRPr":
			sp.RsidRPr = attr.Value
		case "rsidSect":
			sp.RsidSect = attr.Value
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
			case "headerReference":
				v := &CT_HdrFtrRef{}
				v.unmarshalAttrs(t.Attr)
				if err := d.Skip(); err != nil {
					return err
				}
				sp.HeaderReference = append(sp.HeaderReference, v)
			case "footerReference":
				v := &CT_HdrFtrRef{}
				v.unmarshalAttrs(t.Attr)
				if err := d.Skip(); err != nil {
					return err
				}
				sp.FooterReference = append(sp.FooterReference, v)
			case "footnotePr":
				sp.FootnoteProperties = &CT_FtnProps{}
				if err := d.DecodeElement(sp.FootnoteProperties, &t); err != nil {
					return err
				}
			case "endnotePr":
				sp.EndnoteProperties = &CT_EdnProps{}
				if err := d.DecodeElement(sp.EndnoteProperties, &t); err != nil {
					return err
				}
			case "type":
				sp.Type = &CT_String{}
				if err := d.DecodeElement(sp.Type, &t); err != nil {
					return err
				}
			case "pgSz":
				sp.PgSz = &CT_PgSz{}
				if err := d.DecodeElement(sp.PgSz, &t); err != nil {
					return err
				}
			case "pgMar":
				sp.PgMar = &CT_PgMar{}
				if err := d.DecodeElement(sp.PgMar, &t); err != nil {
					return err
				}
			case "paperSrc":
				sp.PaperSrc = &CT_PaperSrc{}
				if err := d.DecodeElement(sp.PaperSrc, &t); err != nil {
					return err
				}
			case "pgBorders":
				sp.PgBorders = &CT_PgBorders{}
				if err := d.DecodeElement(sp.PgBorders, &t); err != nil {
					return err
				}
			case "lnNumType":
				sp.LnNumType = &CT_LnNumType{}
				if err := d.DecodeElement(sp.LnNumType, &t); err != nil {
					return err
				}
			case "pgNumType":
				sp.PgNumType = &CT_PgNumType{}
				if err := d.DecodeElement(sp.PgNumType, &t); err != nil {
					return err
				}
			case "cols":
				sp.Cols = &CT_Columns{}
				if err := d.DecodeElement(sp.Cols, &t); err != nil {
					return err
				}
			case "formProt":
				sp.FormProt = UnmarshalOnOff(d, &t)
			case "vAlign":
				sp.VAlign = &CT_String{}
				if err := d.DecodeElement(sp.VAlign, &t); err != nil {
					return err
				}
			case "noEndnote":
				sp.NoEndnote = UnmarshalOnOff(d, &t)
			case "titlePg":
				sp.TitlePg = UnmarshalOnOff(d, &t)
			case "textDirection":
				sp.TextDirection = &CT_String{}
				if err := d.DecodeElement(sp.TextDirection, &t); err != nil {
					return err
				}
			case "bidi":
				sp.Bidi = UnmarshalOnOff(d, &t)
			case "rtlGutter":
				sp.RtlGutter = UnmarshalOnOff(d, &t)
			case "docGrid":
				sp.DocGrid = &CT_DocGrid{}
				if err := d.DecodeElement(sp.DocGrid, &t); err != nil {
					return err
				}
			case "printerSettings":
				sp.PrinterSettings = &CT_RawElement{}
				if err := d.DecodeElement(sp.PrinterSettings, &t); err != nil {
					return err
				}
			case "sectPrChange":
				sp.SectPrChange = &CT_SectPrChange{}
				if err := d.DecodeElement(sp.SectPrChange, &t); err != nil {
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

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_SectPr.
func (sp *CT_SectPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if sp.RsidR != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "rsidR", Value: sp.RsidR})
	}
	if sp.RsidRPr != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "rsidRPr", Value: sp.RsidRPr})
	}
	if sp.RsidSect != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "rsidSect", Value: sp.RsidSect})
	}
	b.StartElement(ns, localName, attrs...)

	for _, h := range sp.HeaderReference {
		h.marshalTo(b, ns, "headerReference")
	}
	for _, f := range sp.FooterReference {
		f.marshalTo(b, ns, "footerReference")
	}

	if sp.FootnoteProperties != nil {
		b.MarshalElement(ns, "footnotePr", sp.FootnoteProperties)
	}
	if sp.EndnoteProperties != nil {
		b.MarshalElement(ns, "endnotePr", sp.EndnoteProperties)
	}
	if sp.Type != nil {
		b.MarshalElement(ns, "type", sp.Type)
	}
	if sp.PgSz != nil {
		b.MarshalElement(ns, "pgSz", sp.PgSz)
	}
	if sp.PgMar != nil {
		b.MarshalElement(ns, "pgMar", sp.PgMar)
	}
	if sp.PaperSrc != nil {
		b.MarshalElement(ns, "paperSrc", sp.PaperSrc)
	}
	if sp.PgBorders != nil {
		b.MarshalElement(ns, "pgBorders", sp.PgBorders)
	}
	if sp.LnNumType != nil {
		b.MarshalElement(ns, "lnNumType", sp.LnNumType)
	}
	if sp.PgNumType != nil {
		b.MarshalElement(ns, "pgNumType", sp.PgNumType)
	}
	if sp.Cols != nil {
		b.MarshalElement(ns, "cols", sp.Cols)
	}
	if sp.FormProt != nil {
		b.MarshalElement(ns, "formProt", sp.FormProt)
	}
	if sp.VAlign != nil {
		b.MarshalElement(ns, "vAlign", sp.VAlign)
	}
	if sp.NoEndnote != nil {
		b.MarshalElement(ns, "noEndnote", sp.NoEndnote)
	}
	if sp.TitlePg != nil {
		b.MarshalElement(ns, "titlePg", sp.TitlePg)
	}
	if sp.TextDirection != nil {
		b.MarshalElement(ns, "textDirection", sp.TextDirection)
	}
	if sp.Bidi != nil {
		b.MarshalElement(ns, "bidi", sp.Bidi)
	}
	if sp.RtlGutter != nil {
		b.MarshalElement(ns, "rtlGutter", sp.RtlGutter)
	}
	if sp.DocGrid != nil {
		b.MarshalElement(ns, "docGrid", sp.DocGrid)
	}
	if sp.PrinterSettings != nil {
		sp.PrinterSettings.MarshalToBuilder(b, ns, "printerSettings")
	}
	if sp.SectPrChange != nil {
		b.MarshalElement(ns, "sectPrChange", sp.SectPrChange)
	}

	b.EndElement(ns, localName)
}

// CT_HdrFtrRef represents a header/footer reference with r:id.
type CT_HdrFtrRef struct {
	Type string `xml:"-"` // w:type attr
	RID  string `xml:"-"` // r:id attr
}

func (h *CT_HdrFtrRef) unmarshalAttrs(attrs []xml.Attr) {
	for _, attr := range attrs {
		switch {
		case attr.Name.Local == "type":
			h.Type = attr.Value
		case attr.Name.Local == "id" && attr.Name.Space == NsRelationships:
			h.RID = attr.Value
		case attr.Name.Local == "r:id":
			h.RID = attr.Value
		}
	}
}

func (h *CT_HdrFtrRef) marshalTo(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if h.Type != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "type", Value: h.Type})
	}
	if h.RID != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: NsRelationships, Name: "id", Value: h.RID})
	}
	b.EmptyElement(ns, localName, attrs...)
}

// CT_PgSz represents page size.
type CT_PgSz struct {
	W      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main w,attr,omitempty"`
	H      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main h,attr,omitempty"`
	Orient string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main orient,attr,omitempty"`
	Code   string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main code,attr,omitempty"`
}

// CT_PgMar represents page margins.
type CT_PgMar struct {
	Top    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main top,attr,omitempty"`
	Right  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main right,attr,omitempty"`
	Bottom string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bottom,attr,omitempty"`
	Left   string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main left,attr,omitempty"`
	Header string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main header,attr,omitempty"`
	Footer string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main footer,attr,omitempty"`
	Gutter string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main gutter,attr,omitempty"`
}

// CT_PgBorders represents page borders.
type CT_PgBorders struct {
	OffsetFrom string     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main offsetFrom,attr,omitempty"`
	Top        *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main top,omitempty"`
	Left       *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main left,omitempty"`
	Bottom     *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bottom,omitempty"`
	Right      *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main right,omitempty"`
}

// CT_PgNumType represents page numbering settings.
type CT_PgNumType struct {
	Fmt       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main fmt,attr,omitempty"`
	Start     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main start,attr,omitempty"`
	ChapStyle string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main chapStyle,attr,omitempty"`
	ChapSep   string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main chapSep,attr,omitempty"`
}

// CT_PaperSrc represents paper source settings.
type CT_PaperSrc struct {
	First string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main first,attr,omitempty"`
	Other string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main other,attr,omitempty"`
}

// CT_LnNumType represents line numbering settings.
type CT_LnNumType struct {
	CountBy  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main countBy,attr,omitempty"`
	Start    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main start,attr,omitempty"`
	Distance string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main distance,attr,omitempty"`
	Restart  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main restart,attr,omitempty"`
}

// CT_FtnProps represents footnote properties.
type CT_FtnProps struct {
	Pos    *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pos,omitempty"`
	NumFmt *CT_NumFmt `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numFmt,omitempty"`
	NumStart *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numStart,omitempty"`
	NumRestart *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numRestart,omitempty"`
}

// CT_EdnProps represents endnote properties.
type CT_EdnProps struct {
	Pos    *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pos,omitempty"`
	NumFmt *CT_NumFmt `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numFmt,omitempty"`
	NumStart *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numStart,omitempty"`
	NumRestart *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numRestart,omitempty"`
}

// CT_NumFmt represents a number format.
type CT_NumFmt struct {
	Val    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	Format string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main format,attr,omitempty"`
}
