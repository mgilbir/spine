package oxml

import "encoding/xml"

// CT_Numbering is the root element of the numbering definitions part.
type CT_Numbering struct {
	XMLName     xml.Name          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numbering"`
	AbstractNum []*CT_AbstractNum `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main abstractNum"`
	Num         []*CT_Num         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main num"`
	NumIdMacAtCleanup *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numIdMacAtCleanup,omitempty"`
}

// CT_AbstractNum represents an abstract numbering definition.
type CT_AbstractNum struct {
	AbstractNumId string     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main abstractNumId,attr"`
	Nsid          *CT_LongHexNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main nsid,omitempty"`
	MultiLevelType *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main multiLevelType,omitempty"`
	Tmpl          *CT_LongHexNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tmpl,omitempty"`
	Name          *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main name,omitempty"`
	StyleLink     *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main styleLink,omitempty"`
	NumStyleLink  *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numStyleLink,omitempty"`
	Lvl           []*CT_Lvl  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lvl"`
}

// CT_Lvl represents a single numbering level.
type CT_Lvl struct {
	Ilvl          string         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main ilvl,attr"`
	Tplc          string         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tplc,attr,omitempty"`
	Tentative     string         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tentative,attr,omitempty"`
	Start         *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main start,omitempty"`
	NumFmt        *CT_NumFmt     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numFmt,omitempty"`
	LvlRestart    *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lvlRestart,omitempty"`
	PStyle        *CT_String     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pStyle,omitempty"`
	IsLgl         *CT_OnOff      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main isLgl,omitempty"`
	Suff          *CT_String     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main suff,omitempty"`
	LvlText       *CT_LvlText    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lvlText,omitempty"`
	LvlPicBulletId *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lvlPicBulletId,omitempty"`
	LvlJc         *CT_Jc         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lvlJc,omitempty"`
	PPr           *CT_PPr        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pPr,omitempty"`
	RPr           *CT_RPr        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rPr,omitempty"`
}

// CT_LvlText represents the level text template.
type CT_LvlText struct {
	Val  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	Null string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main null,attr,omitempty"`
}

// CT_Num represents a numbering definition instance.
type CT_Num struct {
	NumId         string        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numId,attr"`
	AbstractNumId *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main abstractNumId"`
	LvlOverride   []*CT_NumLvl  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lvlOverride,omitempty"`
}

// CT_NumLvl represents a level override in a numbering instance.
type CT_NumLvl struct {
	Ilvl      string            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main ilvl,attr"`
	StartOverride *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main startOverride,omitempty"`
	Lvl       *CT_Lvl           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lvl,omitempty"`
}
