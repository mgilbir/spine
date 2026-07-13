// Package oxml provides internal XML types for WordprocessingML (WML) documents.
package oxml

import "encoding/xml"

// CT_Empty represents a WML empty element (e.g., <w:noProof/>).
type CT_Empty struct{}

// CT_OnOff represents an on/off toggle element (e.g., <w:b/>, <w:b w:val="0"/>).
// When the element is present without a val attribute, the value is true.
type CT_OnOff struct {
	Val *string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
}

// IsOn returns whether the toggle is on. Present without val means true,
// val="" means true, val="1"/"true"/"on" means true.
func (o *CT_OnOff) IsOn() bool {
	if o == nil {
		return false
	}
	if o.Val == nil {
		return true // present without val = on
	}
	v := *o.Val
	return v == "" || v == "1" || v == "true" || v == "on"
}

// CT_String represents an element with a w:val string attribute.
type CT_String struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
}

// CT_DecimalNumber represents an element with a w:val integer attribute.
type CT_DecimalNumber struct {
	Val int `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
}

// CT_UnsignedDecimalNumber represents an element with a w:val unsigned integer attribute.
type CT_UnsignedDecimalNumber struct {
	Val uint64 `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
}

// CT_TwipsMeasure represents a measurement in twips (1/20 of a point).
type CT_TwipsMeasure struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
}

// CT_HpsMeasure represents a measurement in half-points.
type CT_HpsMeasure struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
}

// CT_SignedTwipsMeasure represents a signed measurement in twips.
type CT_SignedTwipsMeasure struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
}

// CT_SignedHpsMeasure represents a signed half-point measurement.
type CT_SignedHpsMeasure struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
}

// CT_LongHexNumber represents a 4-byte hex number (e.g., rsidR).
type CT_LongHexNumber struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
}

// CT_TextScale represents text scaling percentage.
type CT_TextScale struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
}

// CT_Highlight represents a text highlight color.
type CT_Highlight struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
}

// CT_Color represents a color value with optional theme color.
type CT_Color struct {
	Val        string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	ThemeColor string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeColor,attr,omitempty"`
	ThemeTint  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeTint,attr,omitempty"`
	ThemeShade string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeShade,attr,omitempty"`
}

// CT_Border represents a border definition.
type CT_Border struct {
	Val        string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	Color      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main color,attr,omitempty"`
	ThemeColor string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeColor,attr,omitempty"`
	ThemeTint  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeTint,attr,omitempty"`
	ThemeShade string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeShade,attr,omitempty"`
	Sz         string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sz,attr,omitempty"`
	Space      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main space,attr,omitempty"`
	Shadow     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main shadow,attr,omitempty"`
	Frame      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main frame,attr,omitempty"`
}

// CT_Shd represents shading properties.
type CT_Shd struct {
	Val            string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	Color          string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main color,attr,omitempty"`
	Fill           string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main fill,attr,omitempty"`
	ThemeFill      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeFill,attr,omitempty"`
	ThemeFillTint  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeFillTint,attr,omitempty"`
	ThemeFillShade string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeFillShade,attr,omitempty"`
	ThemeColor     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeColor,attr,omitempty"`
	ThemeTint      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeTint,attr,omitempty"`
	ThemeShade     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeShade,attr,omitempty"`
}

// CT_Underline represents underline formatting.
type CT_Underline struct {
	Val        string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
	Color      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main color,attr,omitempty"`
	ThemeColor string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeColor,attr,omitempty"`
}

// CT_Lang represents language identification.
type CT_Lang struct {
	Val      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
	EastAsia string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main eastAsia,attr,omitempty"`
	Bidi     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bidi,attr,omitempty"`
}

// CT_Fonts represents font specifications.
type CT_Fonts struct {
	Ascii         string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main ascii,attr,omitempty"`
	HAnsi         string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hAnsi,attr,omitempty"`
	EastAsia      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main eastAsia,attr,omitempty"`
	Cs            string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cs,attr,omitempty"`
	AsciiTheme    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main asciiTheme,attr,omitempty"`
	HAnsiTheme    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hAnsiTheme,attr,omitempty"`
	EastAsiaTheme string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main eastAsiaTheme,attr,omitempty"`
	CsTheme       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cstheme,attr,omitempty"`
}

// CT_FitText represents text fitting properties.
type CT_FitText struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	ID  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr,omitempty"`
}

// CT_Em represents emphasis mark type.
type CT_Em struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
}

// CT_VerticalAlignRun represents vertical character alignment.
type CT_VerticalAlignRun struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
}

// CT_TblWidth represents table width/measurement.
type CT_TblWidth struct {
	W    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main w,attr,omitempty"`
	Type string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main type,attr,omitempty"`
}

// CT_PBdr represents paragraph borders.
type CT_PBdr struct {
	Top     *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main top,omitempty"`
	Left    *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main left,omitempty"`
	Bottom  *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bottom,omitempty"`
	Right   *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main right,omitempty"`
	Between *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main between,omitempty"`
	Bar     *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bar,omitempty"`
}

// CT_TblBorders represents table borders.
type CT_TblBorders struct {
	Top     *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main top,omitempty"`
	Left    *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main left,omitempty"`
	Bottom  *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bottom,omitempty"`
	Right   *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main right,omitempty"`
	InsideH *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main insideH,omitempty"`
	InsideV *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main insideV,omitempty"`
}

// CT_TcBorders represents table cell borders.
type CT_TcBorders struct {
	Top     *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main top,omitempty"`
	Left    *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main left,omitempty"`
	Bottom  *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bottom,omitempty"`
	Right   *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main right,omitempty"`
	InsideH *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main insideH,omitempty"`
	InsideV *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main insideV,omitempty"`
	Tl2Br   *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tl2br,omitempty"`
	Tr2Bl   *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tr2bl,omitempty"`
}

// CT_Spacing represents paragraph spacing.
type CT_Spacing struct {
	Before            string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main before,attr,omitempty"`
	BeforeLines       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main beforeLines,attr,omitempty"`
	BeforeAutospacing string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main beforeAutospacing,attr,omitempty"`
	After             string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main after,attr,omitempty"`
	AfterLines        string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main afterLines,attr,omitempty"`
	AfterAutospacing  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main afterAutospacing,attr,omitempty"`
	Line              string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main line,attr,omitempty"`
	LineRule          string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lineRule,attr,omitempty"`
}

// CT_Ind represents paragraph indentation.
type CT_Ind struct {
	Left           string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main left,attr,omitempty"`
	LeftChars      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main leftChars,attr,omitempty"`
	Right          string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main right,attr,omitempty"`
	RightChars     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rightChars,attr,omitempty"`
	Hanging        string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hanging,attr,omitempty"`
	HangingChars   string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hangingChars,attr,omitempty"`
	FirstLine      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main firstLine,attr,omitempty"`
	FirstLineChars string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main firstLineChars,attr,omitempty"`
}

// CT_Jc represents paragraph justification.
type CT_Jc struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
}

// CT_NumPr represents numbering properties.
type CT_NumPr struct {
	Ilvl  *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main ilvl,omitempty"`
	NumId *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numId,omitempty"`
}

// CT_Tabs represents a set of tab stops.
type CT_Tabs struct {
	Tab []CT_TabStop `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tab"`
}

// CT_TabStop represents a single tab stop.
type CT_TabStop struct {
	Val    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	Pos    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pos,attr"`
	Leader string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main leader,attr,omitempty"`
}

// CT_FramePr represents frame properties.
type CT_FramePr struct {
	DropCap    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main dropCap,attr,omitempty"`
	Lines      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lines,attr,omitempty"`
	W          string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main w,attr,omitempty"`
	H          string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main h,attr,omitempty"`
	VSpace     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main vSpace,attr,omitempty"`
	HSpace     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hSpace,attr,omitempty"`
	Wrap       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main wrap,attr,omitempty"`
	HAnchor    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hAnchor,attr,omitempty"`
	VAnchor    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main vAnchor,attr,omitempty"`
	X          string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main x,attr,omitempty"`
	XAlign     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main xAlign,attr,omitempty"`
	Y          string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main y,attr,omitempty"`
	YAlign     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main yAlign,attr,omitempty"`
	HRule      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hRule,attr,omitempty"`
	AnchorLock string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main anchorLock,attr,omitempty"`
}

// CT_DocGrid represents the document grid.
type CT_DocGrid struct {
	Type      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main type,attr,omitempty"`
	LinePitch string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main linePitch,attr,omitempty"`
	CharSpace string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main charSpace,attr,omitempty"`
}

// CT_Columns represents column definitions.
type CT_Columns struct {
	EqualWidth string      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main equalWidth,attr,omitempty"`
	Space      string      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main space,attr,omitempty"`
	Num        string      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main num,attr,omitempty"`
	Sep        string      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sep,attr,omitempty"`
	Col        []CT_Column `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main col,omitempty"`
}

// CT_Column represents a single column definition.
type CT_Column struct {
	W     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main w,attr,omitempty"`
	Space string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main space,attr,omitempty"`
}

// NsWml is the WML namespace constant used throughout this package.
const NsWml = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"

// NsRelationships is the relationship namespace.
const NsRelationships = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"

// UnmarshalOnOff handles the common WML pattern of decoding an on/off element.
// Returns a pointer to CT_OnOff or nil if the element is not found at the current position.
func UnmarshalOnOff(d *xml.Decoder, start *xml.StartElement) *CT_OnOff {
	o := &CT_OnOff{}
	for _, attr := range start.Attr {
		if attr.Name.Local == "val" {
			val := attr.Value
			o.Val = &val
		}
	}
	_ = d.Skip()
	return o
}
