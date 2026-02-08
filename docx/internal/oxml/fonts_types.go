package oxml

// CT_Font represents a font definition in the font table (w:font).
type CT_Font struct {
	Name     string     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main name,attr,omitempty"`
	AltName  *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main altName,omitempty"`
	Panose1  *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main panose1,omitempty"`
	Charset  *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main charset,omitempty"`
	Family   *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main family,omitempty"`
	Pitch    *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pitch,omitempty"`
	Sig      *CT_FontSig `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sig,omitempty"`
	NotTrueType *CT_OnOff `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main notTrueType,omitempty"`
}

// CT_FontSig represents a font signature (w:sig).
type CT_FontSig struct {
	Usb0 string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main usb0,attr,omitempty"`
	Usb1 string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main usb1,attr,omitempty"`
	Usb2 string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main usb2,attr,omitempty"`
	Usb3 string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main usb3,attr,omitempty"`
	Csb0 string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main csb0,attr,omitempty"`
	Csb1 string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main csb1,attr,omitempty"`
}
