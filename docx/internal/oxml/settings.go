package oxml

import "encoding/xml"

// CT_Settings is the root element of the settings part (w:settings).
// Preserves raw content for round-trip fidelity while providing access to key fields.
type CT_Settings struct {
	XMLName    xml.Name `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main settings"`
	RawContent []byte   `xml:",innerxml"`
}
