package oxml

// CT_Ptab represents a positional tab character (w:ptab).
type CT_Ptab struct {
	Alignment  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main alignment,attr,omitempty"`
	RelativeTo string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main relativeTo,attr,omitempty"`
	Leader     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main leader,attr,omitempty"`
}

// CT_Legacy represents legacy numbering properties (w:legacy).
type CT_Legacy struct {
	Legacy       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main legacy,attr,omitempty"`
	LegacySpace  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main legacySpace,attr,omitempty"`
	LegacyIndent string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main legacyIndent,attr,omitempty"`
}

// CT_DirContentRun represents a direction content run (w:dir). The content
// paths do not decode into this type — w:dir is raw-preserved via isRawPChild so
// its runs survive verbatim — but it documents the model's spec coverage of the
// element (see spec_test.go).
type CT_DirContentRun struct {
	Val string  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
	R   []*CT_R `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main r,omitempty"`
	P   []*CT_P `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main p,omitempty"`
}

// CT_BdoContentRun represents a bidirectional override run (w:bdo). Like
// CT_DirContentRun it is a spec-coverage stand-in; w:bdo is raw-preserved.
type CT_BdoContentRun struct {
	Val string  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
	R   []*CT_R `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main r,omitempty"`
	P   []*CT_P `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main p,omitempty"`
}

// CT_NumPicBullet represents a picture numbering bullet (w:numPicBullet).
type CT_NumPicBullet struct {
	NumPicBulletId string      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numPicBulletId,attr,omitempty"`
	Drawing        *CT_Drawing `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main drawing,omitempty"`
}

// CT_FontsList represents a font table (w:fonts).
type CT_FontsList struct {
	Font []*CT_Font `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main font,omitempty"`
}

// CT_Pitch represents font pitch (w:pitch) - alias for CT_String.
type CT_Pitch = CT_String

// CT_Div represents an HTML div element reference (w:div).
type CT_Div struct {
	Id         string     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr,omitempty"`
	MarLeft    *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main marLeft,omitempty"`
	MarRight   *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main marRight,omitempty"`
	MarTop     *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main marTop,omitempty"`
	MarBottom  *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main marBottom,omitempty"`
	DivBdr     *CT_DivBdr `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main divBdr,omitempty"`
	DivsChild  []*CT_Div  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main divsChild,omitempty"`
	BlockQuote *CT_OnOff  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main blockQuote,omitempty"`
	BodyDiv    *CT_OnOff  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bodyDiv,omitempty"`
}

// CT_DivBdr represents div borders.
type CT_DivBdr struct {
	Top    *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main top,omitempty"`
	Left   *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main left,omitempty"`
	Bottom *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bottom,omitempty"`
	Right  *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main right,omitempty"`
}
