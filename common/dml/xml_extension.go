// Package dml provides DrawingML XML extension types from dml-main.xsd.
// These types handle forward/backward compatibility with Office document versions.
package dml

import "encoding/xml"

// ExtLst represents CT_OfficeArtExtensionList (a:extLst)
// Extension list for future compatibility
type ExtLst struct {
	Ext []*Ext `xml:"http://schemas.openxmlformats.org/drawingml/2006/main ext,omitempty"`
}

// Ext represents CT_OfficeArtExtension (a:ext)
// Single extension element with URI identifier
type Ext struct {
	URI      string   `xml:"uri,attr"`
	InnerXML []byte   `xml:",innerxml"`
}

// CompatExt represents compatibility extension wrapper
type CompatExt struct {
	SpId string `xml:"spId,attr,omitempty"`
}

// NonVisualDrawingPropsExtension represents extension for non-visual drawing properties
type NonVisualDrawingPropsExtension struct {
	CreationId *CreationId `xml:"http://schemas.microsoft.com/office/drawing/2014/main creationId,omitempty"`
}

// CreationId represents a16:creationId extension element
type CreationId struct {
	Id string `xml:"id,attr,omitempty"`
}

// Custom unmarshal for Ext to capture raw XML
func (e *Ext) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Local == "uri" {
			e.URI = attr.Value
		}
	}

	// Read inner XML content
	var inner struct {
		Content []byte `xml:",innerxml"`
	}
	if err := d.DecodeElement(&inner, &start); err != nil {
		return err
	}
	e.InnerXML = inner.Content
	return nil
}
