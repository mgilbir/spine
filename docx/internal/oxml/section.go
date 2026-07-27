package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_SectPr represents section properties (w:sectPr).
type CT_SectPr struct {
	// CapturedAttrs preserves the verbatim source attribute list; replayed
	// on marshal so producer attribute order and unmodeled attributes
	// survive.
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
	RsidR         string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsidR,attr,omitempty"`
	RsidRPr       string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsidRPr,attr,omitempty"`
	RsidSect      string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsidSect,attr,omitempty"`

	HeaderReference []*CT_HdrFtrRef `xml:"-"`
	FooterReference []*CT_HdrFtrRef `xml:"-"`
	// hdrFtrOrder records the interleaved document order of the
	// headerReference/footerReference children: EG_HdrFtrReferences is an
	// ordered choice and Word interleaves the two kinds, so emitting all
	// headers then all footers drifts from the source.
	hdrFtrOrder []hdrFtrOrderRef
	// childSeq records the local names of all children in source order, so a
	// producer that deviates from the schema sequence (w:pgNumType after
	// w:cols) replays its order. Empty for programmatic sections.
	childSeq           []string
	FootnoteProperties *CT_FtnProps  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main footnotePr,omitempty"`
	EndnoteProperties  *CT_EdnProps  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main endnotePr,omitempty"`
	Type               *CT_String    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main type,omitempty"`
	PgSz               *CT_PgSz      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pgSz,omitempty"`
	PgMar              *CT_PgMar     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pgMar,omitempty"`
	PaperSrc           *CT_PaperSrc  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main paperSrc,omitempty"`
	PgBorders          *CT_PgBorders `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pgBorders,omitempty"`
	LnNumType          *CT_LnNumType `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lnNumType,omitempty"`
	PgNumType          *CT_PgNumType `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pgNumType,omitempty"`
	Cols               *CT_Columns   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cols,omitempty"`
	FormProt           *CT_OnOff     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main formProt,omitempty"`
	VAlign             *CT_String    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main vAlign,omitempty"`
	NoEndnote          *CT_OnOff     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main noEndnote,omitempty"`
	TitlePg            *CT_OnOff     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main titlePg,omitempty"`
	TextDirection      *CT_String    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main textDirection,omitempty"`
	Bidi               *CT_OnOff     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bidi,omitempty"`
	RtlGutter          *CT_OnOff     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rtlGutter,omitempty"`
	DocGrid            *CT_DocGrid   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main docGrid,omitempty"`
	// PrinterSettings (w:printerSettings, a CT_Rel carrying r:id) is preserved
	// raw: the model does not interpret it, but stripping it would orphan the
	// printer-settings part.
	PrinterSettings *CT_RawElement   `xml:"-"`
	SectPrChange    *CT_SectPrChange `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sectPrChange,omitempty"`
	// unknownChildren holds sectPr children the model does not type, captured
	// verbatim in document order (they align with the non-modeled entries of
	// childSeq) so a regenerated sectPr does not silently drop them — e.g. the
	// strict-schema or otherwise unmodeled elements some producers emit. Empty
	// for programmatic sections.
	unknownChildren []*CT_RawNamedElement
}

// UnmarshalXML implements custom unmarshaling for CT_SectPr to handle r:id attributes.
func (sp *CT_SectPr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	sp.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
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
			sp.childSeq = append(sp.childSeq, t.Name.Local)
			switch t.Name.Local {
			case "headerReference":
				v := &CT_HdrFtrRef{}
				v.unmarshalAttrs(t.Attr)
				v.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
				if err := d.Skip(); err != nil {
					return err
				}
				sp.hdrFtrOrder = append(sp.hdrFtrOrder, hdrFtrOrderRef{footer: false, index: len(sp.HeaderReference)})
				sp.HeaderReference = append(sp.HeaderReference, v)
			case "footerReference":
				v := &CT_HdrFtrRef{}
				v.unmarshalAttrs(t.Attr)
				v.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
				if err := d.Skip(); err != nil {
					return err
				}
				sp.hdrFtrOrder = append(sp.hdrFtrOrder, hdrFtrOrderRef{footer: true, index: len(sp.FooterReference)})
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
				rn := &CT_RawNamedElement{}
				if err := d.DecodeElement(rn, &t); err != nil {
					return err
				}
				sp.unknownChildren = append(sp.unknownChildren, rn)
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
	if sp.CapturedAttrs != nil {
		attrs = b.ReplayCapturedAttrs(sp.CapturedAttrs, attrs)
	}
	b.StartElement(ns, localName, attrs...)

	emit := map[string]func(){
		"footnotePr": func() {
			if sp.FootnoteProperties != nil {
				b.MarshalElement(ns, "footnotePr", sp.FootnoteProperties)
			}
		},
		"endnotePr": func() {
			if sp.EndnoteProperties != nil {
				b.MarshalElement(ns, "endnotePr", sp.EndnoteProperties)
			}
		},
		"type": func() {
			if sp.Type != nil {
				b.MarshalElement(ns, "type", sp.Type)
			}
		},
		"pgSz": func() {
			if sp.PgSz != nil {
				b.MarshalElement(ns, "pgSz", sp.PgSz)
			}
		},
		"pgMar": func() {
			if sp.PgMar != nil {
				b.MarshalElement(ns, "pgMar", sp.PgMar)
			}
		},
		"paperSrc": func() {
			if sp.PaperSrc != nil {
				b.MarshalElement(ns, "paperSrc", sp.PaperSrc)
			}
		},
		"pgBorders": func() {
			if sp.PgBorders != nil {
				b.MarshalElement(ns, "pgBorders", sp.PgBorders)
			}
		},
		"lnNumType": func() {
			if sp.LnNumType != nil {
				b.MarshalElement(ns, "lnNumType", sp.LnNumType)
			}
		},
		"pgNumType": func() {
			if sp.PgNumType != nil {
				b.MarshalElement(ns, "pgNumType", sp.PgNumType)
			}
		},
		"cols": func() {
			if sp.Cols != nil {
				b.MarshalElement(ns, "cols", sp.Cols)
			}
		},
		"formProt": func() {
			if sp.FormProt != nil {
				b.MarshalElement(ns, "formProt", sp.FormProt)
			}
		},
		"vAlign": func() {
			if sp.VAlign != nil {
				b.MarshalElement(ns, "vAlign", sp.VAlign)
			}
		},
		"noEndnote": func() {
			if sp.NoEndnote != nil {
				b.MarshalElement(ns, "noEndnote", sp.NoEndnote)
			}
		},
		"titlePg": func() {
			if sp.TitlePg != nil {
				b.MarshalElement(ns, "titlePg", sp.TitlePg)
			}
		},
		"textDirection": func() {
			if sp.TextDirection != nil {
				b.MarshalElement(ns, "textDirection", sp.TextDirection)
			}
		},
		"bidi": func() {
			if sp.Bidi != nil {
				b.MarshalElement(ns, "bidi", sp.Bidi)
			}
		},
		"rtlGutter": func() {
			if sp.RtlGutter != nil {
				b.MarshalElement(ns, "rtlGutter", sp.RtlGutter)
			}
		},
		"docGrid": func() {
			if sp.DocGrid != nil {
				b.MarshalElement(ns, "docGrid", sp.DocGrid)
			}
		},
		"printerSettings": func() {
			if sp.PrinterSettings != nil {
				sp.PrinterSettings.MarshalToBuilder(b, ns, "printerSettings")
			}
		},
		"sectPrChange": func() {
			if sp.SectPrChange != nil {
				b.MarshalElement(ns, "sectPrChange", sp.SectPrChange)
			}
		},
	}
	// canonical is the schema emission order used for children the captured
	// sequence does not cover (programmatic sections, post-parse additions).
	canonical := []string{"footnotePr", "endnotePr", "type", "pgSz", "pgMar",
		"paperSrc", "pgBorders", "lnNumType", "pgNumType", "cols", "formProt",
		"vAlign", "noEndnote", "titlePg", "textDirection", "bidi", "rtlGutter",
		"docGrid", "printerSettings", "sectPrChange"}

	hdrFtrDone := false
	emitted := make(map[string]bool)
	unkIdx := 0
	for _, name := range sp.childSeq {
		if name == "headerReference" || name == "footerReference" {
			if !hdrFtrDone {
				sp.marshalHdrFtrReferences(b, ns)
				hdrFtrDone = true
			}
			continue
		}
		if f, ok := emit[name]; ok {
			if !emitted[name] {
				emitted[name] = true
				f()
			}
			continue
		}
		// Unmodeled child: replay the next captured raw element verbatim in
		// its source position (childSeq order matches unknownChildren order).
		if unkIdx < len(sp.unknownChildren) {
			sp.unknownChildren[unkIdx].MarshalNamed(b, ns)
			unkIdx++
		}
	}
	if !hdrFtrDone {
		sp.marshalHdrFtrReferences(b, ns)
	}
	for _, name := range canonical {
		if !emitted[name] {
			emit[name]()
		}
	}
	// Drain any captured children not covered by childSeq (defensive; for a
	// parsed section childSeq already accounts for every child).
	for ; unkIdx < len(sp.unknownChildren); unkIdx++ {
		sp.unknownChildren[unkIdx].MarshalNamed(b, ns)
	}

	b.EndElement(ns, localName)
}

// CT_HdrFtrRef represents a header/footer reference with r:id.
type CT_HdrFtrRef struct {
	Type string `xml:"-"` // w:type attr
	RID  string `xml:"-"` // r:id attr
	// CapturedAttrs preserves the verbatim source attribute list; replayed
	// on marshal (producers disagree on r:id vs w:type order).
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
	// CapturedEmptyTag records the source's empty-element style (some
	// producers write <w:headerReference ...></w:headerReference>).
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

func (h *CT_HdrFtrRef) unmarshalAttrs(attrs []xml.Attr) {
	h.CapturedAttrs = xmlb.CaptureAttrs(attrs)
	for _, attr := range attrs {
		switch {
		case attr.Name.Local == "type":
			h.Type = attr.Value
		case attr.Name.Local == "id" && attr.Name.Space == NsRelationships:
			h.RID = attr.Value
		case attr.Name.Local == "id" && attr.Name.Space == "r":
			// Lenience for an undeclared r: prefix: Go's decoder leaves the
			// literal prefix "r" as the namespace, never yields Local=="r:id".
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
	if h.CapturedAttrs != nil {
		attrs = b.ReplayCapturedAttrs(h.CapturedAttrs, attrs)
	}
	b.EmptyElementStyled(h.CapturedEmptyTag, ns, localName, attrs...)
}

// CT_PgSz represents page size.
type CT_PgSz struct {
	W             string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main w,attr,omitempty"`
	H             string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main h,attr,omitempty"`
	Orient        string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main orient,attr,omitempty"`
	Code          string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main code,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (ps *CT_PgSz) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	ps.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	ps.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_PgSz
	return d.DecodeElement((*alias)(ps), &start)
}

// CT_PgMar represents page margins.
type CT_PgMar struct {
	Top           string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main top,attr,omitempty"`
	Right         string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main right,attr,omitempty"`
	Bottom        string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bottom,attr,omitempty"`
	Left          string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main left,attr,omitempty"`
	Header        string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main header,attr,omitempty"`
	Footer        string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main footer,attr,omitempty"`
	Gutter        string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main gutter,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (pm *CT_PgMar) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	pm.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	pm.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_PgMar
	return d.DecodeElement((*alias)(pm), &start)
}

// CT_PgBorders represents page borders.
type CT_PgBorders struct {
	OffsetFrom    string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main offsetFrom,attr,omitempty"`
	Top           *CT_Border      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main top,omitempty"`
	Left          *CT_Border      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main left,omitempty"`
	Bottom        *CT_Border      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bottom,omitempty"`
	Right         *CT_Border      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main right,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (pb *CT_PgBorders) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	pb.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_PgBorders
	return d.DecodeElement((*alias)(pb), &start)
}

// CT_PgNumType represents page numbering settings.
type CT_PgNumType struct {
	Fmt       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main fmt,attr,omitempty"`
	Start     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main start,attr,omitempty"`
	ChapStyle string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main chapStyle,attr,omitempty"`
	ChapSep   string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main chapSep,attr,omitempty"`
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's empty-tag style before decoding
// through the struct tags.
func (v *CT_PgNumType) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	type alias CT_PgNumType
	return d.DecodeElement((*alias)(v), &start)
}

// CT_PaperSrc represents paper source settings.
type CT_PaperSrc struct {
	First string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main first,attr,omitempty"`
	Other string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main other,attr,omitempty"`
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's empty-tag style before decoding
// through the struct tags.
func (v *CT_PaperSrc) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	type alias CT_PaperSrc
	return d.DecodeElement((*alias)(v), &start)
}

// CT_LnNumType represents line numbering settings.
type CT_LnNumType struct {
	CountBy  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main countBy,attr,omitempty"`
	Start    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main start,attr,omitempty"`
	Distance string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main distance,attr,omitempty"`
	Restart  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main restart,attr,omitempty"`
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's empty-tag style before decoding
// through the struct tags.
func (v *CT_LnNumType) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	type alias CT_LnNumType
	return d.DecodeElement((*alias)(v), &start)
}

// CT_FtnProps represents footnote properties.
type CT_FtnProps struct {
	Pos        *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pos,omitempty"`
	NumFmt     *CT_NumFmt        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numFmt,omitempty"`
	NumStart   *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numStart,omitempty"`
	NumRestart *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numRestart,omitempty"`
}

// CT_EdnProps represents endnote properties.
type CT_EdnProps struct {
	Pos        *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pos,omitempty"`
	NumFmt     *CT_NumFmt        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numFmt,omitempty"`
	NumStart   *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numStart,omitempty"`
	NumRestart *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numRestart,omitempty"`
}

// CT_NumFmt represents a number format.
type CT_NumFmt struct {
	Val    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	Format string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main format,attr,omitempty"`
}

// hdrFtrOrderRef locates one entry of the interleaved header/footer
// reference sequence: which slice it lives in and at what index.
type hdrFtrOrderRef struct {
	footer bool
	index  int
}

// marshalHdrFtrReferences writes the headerReference/footerReference children
// in their captured document order; references added after parse (or on a
// programmatically built section) follow, headers first.
func (sp *CT_SectPr) marshalHdrFtrReferences(b *xmlb.Builder, ns string) {
	writtenHdr := 0
	writtenFtr := 0
	for _, ref := range sp.hdrFtrOrder {
		if ref.footer {
			if ref.index < len(sp.FooterReference) {
				sp.FooterReference[ref.index].marshalTo(b, ns, "footerReference")
				writtenFtr++
			}
			continue
		}
		if ref.index < len(sp.HeaderReference) {
			sp.HeaderReference[ref.index].marshalTo(b, ns, "headerReference")
			writtenHdr++
		}
	}
	for _, h := range sp.HeaderReference[min(writtenHdr, len(sp.HeaderReference)):] {
		h.marshalTo(b, ns, "headerReference")
	}
	for _, f := range sp.FooterReference[min(writtenFtr, len(sp.FooterReference)):] {
		f.marshalTo(b, ns, "footerReference")
	}
}
