package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_Compat represents compatibility settings (w:compat).
// Contains CT_OnOff children for individual compatibility options
// and CT_CompatSetting entries. The children are captured as a map of
// element name -> CT_OnOff.
type CT_Compat struct {
	Options        map[string]*CT_OnOff `xml:"-"`
	CompatSettings []CT_CompatSetting   `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_Compat.
func (c *CT_Compat) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	c.Options = make(map[string]*CT_OnOff)
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "compatSetting" {
				var cs CT_CompatSetting
				if err := d.DecodeElement(&cs, &t); err != nil {
					return err
				}
				c.CompatSettings = append(c.CompatSettings, cs)
			} else {
				o := UnmarshalOnOff(d, &t)
				c.Options[t.Name.Local] = o
			}
		case xml.EndElement:
			return nil
		}
	}
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Compat.
func (c *CT_Compat) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName)
	for name, o := range c.Options {
		if o.Val != nil {
			b.EmptyElement(ns, name, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "val", Value: *o.Val})
		} else {
			b.EmptyElement(ns, name)
		}
	}
	for _, cs := range c.CompatSettings {
		b.MarshalElement(ns, "compatSetting", &cs)
	}
	b.EndElement(ns, localName)
}

// CT_CompatSetting represents a single compatibility setting (w:compatSetting).
type CT_CompatSetting struct {
	Name string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main name,attr,omitempty"`
	URI  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main uri,attr,omitempty"`
	Val  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
}

// CT_ClrSchemeMapping represents color scheme mapping (w:clrSchemeMapping).
type CT_ClrSchemeMapping struct {
	Bg1      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bg1,attr,omitempty"`
	T1       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main t1,attr,omitempty"`
	Bg2      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bg2,attr,omitempty"`
	T2       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main t2,attr,omitempty"`
	Accent1  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main accent1,attr,omitempty"`
	Accent2  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main accent2,attr,omitempty"`
	Accent3  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main accent3,attr,omitempty"`
	Accent4  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main accent4,attr,omitempty"`
	Accent5  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main accent5,attr,omitempty"`
	Accent6  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main accent6,attr,omitempty"`
	Hyperlink string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hyperlink,attr,omitempty"`
	FollowedHyperlink string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main followedHyperlink,attr,omitempty"`
}

// CT_WebSettings represents web settings (w:webSettings).
// Children are various CT_OnOff and other elements; captured as a map.
type CT_WebSettings struct {
	XMLName xml.Name           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main webSettings"`
	Options map[string]*CT_OnOff `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_WebSettings.
func (w *CT_WebSettings) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	w.Options = make(map[string]*CT_OnOff)
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			o := UnmarshalOnOff(d, &t)
			w.Options[t.Name.Local] = o
		case xml.EndElement:
			return nil
		}
	}
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_WebSettings.
func (w *CT_WebSettings) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName)
	for name, o := range w.Options {
		if o.Val != nil {
			b.EmptyElement(ns, name, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "val", Value: *o.Val})
		} else {
			b.EmptyElement(ns, name)
		}
	}
	b.EndElement(ns, localName)
}

// CT_RevisionView represents revision view settings (w:revisionView).
type CT_RevisionView struct {
	Markup         string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main markup,attr,omitempty"`
	Comments       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main comments,attr,omitempty"`
	InsDel         string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main insDel,attr,omitempty"`
	Formatting     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main formatting,attr,omitempty"`
	InkAnnotations string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main inkAnnotations,attr,omitempty"`
}

// CT_DocumentProtection represents document protection (w:documentProtection).
type CT_DocumentProtection struct {
	Edit                string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main edit,attr,omitempty"`
	Enforcement         string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main enforcement,attr,omitempty"`
	Formatting          string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main formatting,attr,omitempty"`
	CryptAlgorithmClass string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cryptAlgorithmClass,attr,omitempty"`
	CryptAlgorithmType  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cryptAlgorithmType,attr,omitempty"`
	CryptAlgorithmSid   string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cryptAlgorithmSid,attr,omitempty"`
	CryptSpinCount      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cryptSpinCount,attr,omitempty"`
	Hash                string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hash,attr,omitempty"`
	Salt                string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main salt,attr,omitempty"`
	HashValue           string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hashValue,attr,omitempty"`
	SaltValue           string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main saltValue,attr,omitempty"`
	SpinCount           string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main spinCount,attr,omitempty"`
	AlgorithmName       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main algorithmName,attr,omitempty"`
	CryptProvider       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cryptProvider,attr,omitempty"`
	CryptProviderType   string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cryptProviderType,attr,omitempty"`
}

// CT_Captions represents caption settings (w:captions).
type CT_Captions struct {
	Caption      []CT_Caption     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main caption,omitempty"`
	AutoCaptions *CT_AutoCaptions `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main autoCaptions,omitempty"`
}

// CT_Caption represents a single caption definition (w:caption).
type CT_Caption struct {
	Name    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main name,attr,omitempty"`
	Pos     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pos,attr,omitempty"`
	ChapNum string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main chapNum,attr,omitempty"`
	Heading string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main heading,attr,omitempty"`
	NumFmt  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numFmt,attr,omitempty"`
	Sep     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sep,attr,omitempty"`
	NoLabel string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main noLabel,attr,omitempty"`
}

// CT_AutoCaptions represents auto-caption settings (w:autoCaptions).
type CT_AutoCaptions struct {
	AutoCaption []CT_AutoCaption `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main autoCaption,omitempty"`
}

// CT_AutoCaption represents a single auto-caption mapping (w:autoCaption).
type CT_AutoCaption struct {
	Name    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main name,attr,omitempty"`
	Caption string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main caption,attr,omitempty"`
}

// CT_DocVars represents document variables (w:docVars).
type CT_DocVars struct {
	DocVar []CT_DocVar `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main docVar,omitempty"`
}

// CT_DocVar represents a single document variable (w:docVar).
type CT_DocVar struct {
	Name string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main name,attr,omitempty"`
	Val  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
}

// CT_Rsids represents revision save IDs (w:rsids).
type CT_Rsids struct {
	RsidRoot *CT_LongHexNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsidRoot,omitempty"`
	Rsid     []CT_LongHexNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsid,omitempty"`
}

// CT_ProofState represents proofing state (w:proofState).
type CT_ProofState struct {
	Spelling string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main spelling,attr,omitempty"`
	Grammar  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main grammar,attr,omitempty"`
}

// CT_ReadModeInkLockDown represents read mode ink lock down settings (w:readModeInkLockDown).
type CT_ReadModeInkLockDown struct {
	W        string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main w,attr,omitempty"`
	H        string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main h,attr,omitempty"`
	FontSz   string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main fontSz,attr,omitempty"`
	ActualPg string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main actualPg,attr,omitempty"`
}

// CT_Zoom represents zoom settings (w:zoom).
type CT_Zoom struct {
	Val     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
	Percent string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main percent,attr,omitempty"`
}

// CT_WriteProtection represents write protection (w:writeProtection).
type CT_WriteProtection struct {
	Recommended         string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main recommended,attr,omitempty"`
	HashValue           string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hashValue,attr,omitempty"`
	SaltValue           string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main saltValue,attr,omitempty"`
	SpinCount           string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main spinCount,attr,omitempty"`
	AlgorithmName       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main algorithmName,attr,omitempty"`
	CryptAlgorithmClass string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cryptAlgorithmClass,attr,omitempty"`
	CryptAlgorithmType  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cryptAlgorithmType,attr,omitempty"`
	CryptAlgorithmSid   string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cryptAlgorithmSid,attr,omitempty"`
	CryptSpinCount      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cryptSpinCount,attr,omitempty"`
	Hash                string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hash,attr,omitempty"`
	Salt                string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main salt,attr,omitempty"`
	CryptProvider       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cryptProvider,attr,omitempty"`
	CryptProviderType   string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cryptProviderType,attr,omitempty"`
}

// CT_ActiveWritingStyle represents an active writing style (w:activeWritingStyle).
type CT_ActiveWritingStyle struct {
	AppName  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main appName,attr,omitempty"`
	Lang     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lang,attr,omitempty"`
	VendorID string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main vendorID,attr,omitempty"`
	DllVersion string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main dllVersion,attr,omitempty"`
	NlCheck  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main nlCheck,attr,omitempty"`
	CheckStyle string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main checkStyle,attr,omitempty"`
}

// CT_ThemeFontLang represents theme font languages (w:themeFontLang).
type CT_ThemeFontLang struct {
	Val      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
	EastAsia string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main eastAsia,attr,omitempty"`
	Bidi     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bidi,attr,omitempty"`
}

// CT_ShapeDefaults represents default shape properties (w:shapeDefaults, w:hdrShapeDefaults).
type CT_ShapeDefaults struct{}

// CT_SmartTagType represents a smart tag type (w:smartTagType).
type CT_SmartTagType struct {
	NamespaceURI string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main namespaceuri,attr,omitempty"`
	Name         string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main name,attr,omitempty"`
	URL          string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main url,attr,omitempty"`
}

// CT_SmartTagPr represents smart tag properties (w:smartTagPr).
type CT_SmartTagPr struct {
	Attr []*CT_SmartTagAttr `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main attr,omitempty"`
}

// CT_SmartTagAttr represents a smart tag attribute.
type CT_SmartTagAttr struct {
	Name string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main name,attr,omitempty"`
	Val  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
	URI  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main uri,attr,omitempty"`
}

// CT_SdtDate represents a structured document tag date element (w:date).
type CT_SdtDate struct {
	FullDate   string     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main fullDate,attr,omitempty"`
	DateFormat *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main dateFormat,omitempty"`
	Lid        *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lid,omitempty"`
	StoreMappedDataAs *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main storeMappedDataAs,omitempty"`
	Calendar   *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main calendar,omitempty"`
}

// CT_StylePaneFormatFilter represents style pane format filter (w:stylePaneFormatFilter).
type CT_StylePaneFormatFilter struct {
	Val                   string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
	AllStyles             string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main allStyles,attr,omitempty"`
	CustomStyles          string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main customStyles,attr,omitempty"`
	LatentStyles          string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main latentStyles,attr,omitempty"`
	StylesInUse           string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main stylesInUse,attr,omitempty"`
	HeadingStyles         string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main headingStyles,attr,omitempty"`
	NumberingStyles       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numberingStyles,attr,omitempty"`
	TableStyles           string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tableStyles,attr,omitempty"`
	DirectFormattingOnRuns       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main directFormattingOnRuns,attr,omitempty"`
	DirectFormattingOnParagraphs string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main directFormattingOnParagraphs,attr,omitempty"`
	DirectFormattingOnNumbering  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main directFormattingOnNumbering,attr,omitempty"`
	DirectFormattingOnTables     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main directFormattingOnTables,attr,omitempty"`
	ClearFormatting              string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main clearFormatting,attr,omitempty"`
	Top3HeadingStyles            string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main top3HeadingStyles,attr,omitempty"`
	VisibleStyles                string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main visibleStyles,attr,omitempty"`
	AlternateStyleNames          string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main alternateStyleNames,attr,omitempty"`
}
