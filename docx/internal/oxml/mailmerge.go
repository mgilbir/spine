package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_MailMerge represents mail merge settings (w:mailMerge).
type CT_MailMerge struct {
	MainDocumentType *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main mainDocumentType,omitempty"`
	LinkToQuery      *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main linkToQuery,omitempty"`
	DataType         *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main dataType,omitempty"`
	ConnectString    *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main connectString,omitempty"`
	Query            *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main query,omitempty"`
	DataSource       *CT_Rel           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main dataSource,omitempty"`
	HeaderSource     *CT_Rel           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main headerSource,omitempty"`
	DoNotSuppressBlankLines *CT_OnOff  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main doNotSuppressBlankLines,omitempty"`
	Destination      *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main destination,omitempty"`
	AddressFieldName *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main addressFieldName,omitempty"`
	MailSubject      *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main mailSubject,omitempty"`
	MailAsAttachment *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main mailAsAttachment,omitempty"`
	ViewMergedData   *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main viewMergedData,omitempty"`
	ActiveRecord     *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main activeRecord,omitempty"`
	CheckErrors      *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main checkErrors,omitempty"`
	Odso             *CT_Odso          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main odso,omitempty"`
}

// CT_Odso represents Office Data Source Object settings (w:odso).
type CT_Odso struct {
	UdlConnString  *CT_String         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main udl,omitempty"`
	Table          *CT_String         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main table,omitempty"`
	Src            *CT_Rel            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main src,omitempty"`
	ColDelim       *CT_DecimalNumber  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main colDelim,omitempty"`
	Type           *CT_String         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main type,omitempty"`
	FHdr           *CT_OnOff          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main fHdr,omitempty"`
	FieldMapData   []*CT_OdsoFieldMapData `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main fieldMapData,omitempty"`
	RecipientData  []*CT_Rel          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main recipientData,omitempty"`
}

// CT_OdsoFieldMapData represents field map data (w:fieldMapData).
type CT_OdsoFieldMapData struct {
	Type        *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main type,omitempty"`
	Name        *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main name,omitempty"`
	MappedName  *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main mappedName,omitempty"`
	Column      *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main column,omitempty"`
	Lid         *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lid,omitempty"`
	DynamicAddress *CT_OnOff      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main dynamicAddress,omitempty"`
}

// CT_Recipients represents mail merge recipient data (w:recipients).
type CT_Recipients struct {
	RecipientData []*CT_RecipientData `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main recipientData,omitempty"`
}

// CT_RecipientData represents a single recipient record (w:recipientData).
type CT_RecipientData struct {
	Active    *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main active,omitempty"`
	Column    *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main column,omitempty"`
	UniqueTag *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main uniqueTag,omitempty"`
}

// CT_SaveThroughXslt represents XSLT save settings (w:saveThroughXslt).
type CT_SaveThroughXslt struct {
	RID        string `xml:"-"` // r:id attr
	SolutionID string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main solutionID,attr,omitempty"`
}

func (s *CT_SaveThroughXslt) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "solutionID":
			s.SolutionID = attr.Value
		case attr.Name.Local == "id" && (attr.Name.Space == NsRelationships || attr.Name.Space == "r"):
			s.RID = attr.Value
		}
	}
	return d.Skip()
}

func (s *CT_SaveThroughXslt) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if s.RID != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: NsRelationships, Name: "id", Value: s.RID})
	}
	if s.SolutionID != "" {
		attrs = append(attrs, xmlb.StrAttr("solutionID", s.SolutionID))
	}
	b.EmptyElement(ns, localName, attrs...)
}

// CT_Rel represents a relationship reference element (w:attachedTemplate, etc.)
// with an r:id attribute.
type CT_Rel struct {
	RID string `xml:"-"` // r:id attr
}

func (r *CT_Rel) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Local == "id" && (attr.Name.Space == NsRelationships || attr.Name.Space == "r") {
			r.RID = attr.Value
		}
	}
	return d.Skip()
}

func (r *CT_Rel) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if r.RID != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: NsRelationships, Name: "id", Value: r.RID})
	}
	b.EmptyElement(ns, localName, attrs...)
}
