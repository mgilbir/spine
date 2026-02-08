package oxml

// CT_RubyPr represents ruby (phonetic guide) properties (w:rubyPr).
type CT_RubyPr struct {
	RubyAlign  *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rubyAlign,omitempty"`
	Hps        *CT_HpsMeasure    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hps,omitempty"`
	HpsRaise   *CT_HpsMeasure    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hpsRaise,omitempty"`
	HpsBaseText *CT_HpsMeasure   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hpsBaseText,omitempty"`
	Lid        *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lid,omitempty"`
	Dirty      *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main dirty,omitempty"`
}

// CT_RubyContent represents ruby base or ruby text content (w:rubyBase, w:rt).
type CT_RubyContent struct {
	R []*CT_R `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main r,omitempty"`
	P []*CT_P `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main p,omitempty"`
}
