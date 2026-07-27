// This file provides PresentationML presentation property types from pml.xsd.
// These types implement the p: namespace presProps elements.

package oxml

import "github.com/mgilbir/spine/common/dml"

// PresentationProperties represents CT_PresentationProperties (p:presentationPr)
type PresentationProperties struct {
	HtmlPubPr    *HtmlPublishProperties   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main htmlPubPr,omitempty"`
	WebPr        *WebProperties           `xml:"http://schemas.openxmlformats.org/presentationml/2006/main webPr,omitempty"`
	PrnPr        *PrintProperties         `xml:"http://schemas.openxmlformats.org/presentationml/2006/main prnPr,omitempty"`
	ShowPr       *ShowProperties          `xml:"http://schemas.openxmlformats.org/presentationml/2006/main showPr,omitempty"`
	ClrMru       *ColorMRU                `xml:"http://schemas.openxmlformats.org/presentationml/2006/main clrMru,omitempty"`
	ExtLst       *ExtensionList              `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
}

// The p:presentationPr booleans below split into two groups per pml.xsd:
// XSD default-FALSE flags stay plain bool (omitempty then round-trips every
// non-default value), while XSD default-TRUE flags must be *bool — parsing an
// explicit "0" into a plain bool and re-marshaling deletes the attribute, and a
// reader then reapplies the default true, silently inverting the setting
// (the C29/C316/C317 rule).

// HtmlPublishProperties represents CT_HtmlPublishProperties (p:htmlPubPr).
// showSpeakerNotes defaults to TRUE.
type HtmlPublishProperties struct {
	ShowSpeakerNotes *bool          `xml:"showSpeakerNotes,attr,omitempty"`
	PubBrowser       string         `xml:"pubBrowser,attr,omitempty"` // v4, v3, v3v4
	Title            string         `xml:"title,attr,omitempty"`
	Id               string         `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
	ExtLst           *ExtensionList `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
}

// WebProperties represents CT_WebProperties (p:webPr).
// showAnimation defaults to false; resizeGraphics, organizeInFolders and
// useLongFilenames default to TRUE.
type WebProperties struct {
	ShowAnimation     bool           `xml:"showAnimation,attr,omitempty"`
	ResizeGraphics    *bool          `xml:"resizeGraphics,attr,omitempty"`
	AllowPng          bool           `xml:"allowPng,attr,omitempty"`
	RelyOnVml         bool           `xml:"relyOnVml,attr,omitempty"`
	OrganizeInFolders *bool          `xml:"organizeInFolders,attr,omitempty"`
	UseLongFilenames  *bool          `xml:"useLongFilenames,attr,omitempty"`
	ImgSz             string         `xml:"imgSz,attr,omitempty"` // screen640x480, screen800x600, screen1024x768, screen1152x882, screen1152x900, screen1280x1024, screen1600x1200, screen1800x1440, screen1920x1200
	Encoding          string         `xml:"encoding,attr,omitempty"`
	Clr               string         `xml:"clr,attr,omitempty"` // none, browser, presentationText, presentationAccent, whiteTextOnBlack, blackTextOnWhite
	ExtLst            *ExtensionList `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
}

// PrintProperties represents CT_PrintProperties (p:prnPr)
type PrintProperties struct {
	PrnWhat         string `xml:"prnWhat,attr,omitempty"`      // slides, handouts1, handouts2, handouts3, handouts4, handouts6, handouts9, notes, outline
	ClrMode         string `xml:"clrMode,attr,omitempty"`      // bw, gray, clr
	HiddenSlides    bool   `xml:"hiddenSlides,attr,omitempty"`
	ScaleToFitPaper bool   `xml:"scaleToFitPaper,attr,omitempty"`
	FrameSlides     bool   `xml:"frameSlides,attr,omitempty"`
	ExtLst          *ExtensionList `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
}

// ShowProperties represents CT_ShowProperties (p:showPr).
// loop and showNarration default to false; showAnimation and useTimings
// default to TRUE.
type ShowProperties struct {
	Loop          bool             `xml:"loop,attr,omitempty"`
	ShowNarration bool             `xml:"showNarration,attr,omitempty"`
	ShowAnimation *bool            `xml:"showAnimation,attr,omitempty"`
	UseTimings    *bool            `xml:"useTimings,attr,omitempty"`
	Present       *ShowInfoPresent `xml:"http://schemas.openxmlformats.org/presentationml/2006/main present,omitempty"`
	Browse       *ShowInfoBrowse  `xml:"http://schemas.openxmlformats.org/presentationml/2006/main browse,omitempty"`
	Kiosk        *ShowInfoKiosk   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main kiosk,omitempty"`
	SldAll       *EmptyElement    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sldAll,omitempty"`
	SldRg        *IndexRange      `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sldRg,omitempty"`
	CustShow     *CustomShowRef   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main custShow,omitempty"`
	PenClr       *dml.ColorChoice `xml:"http://schemas.openxmlformats.org/presentationml/2006/main penClr,omitempty"`
	ExtLst       *ExtensionList      `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
}

// EmptyElement represents CT_Empty
type EmptyElement struct{}

// ShowInfoPresent represents CT_ShowInfoPresent (p:present)
type ShowInfoPresent struct{}

// ShowInfoBrowse represents CT_ShowInfoBrowse (p:browse).
// showScrollbar defaults to TRUE.
type ShowInfoBrowse struct {
	ShowScrollbar *bool `xml:"showScrollbar,attr,omitempty"`
}

// ShowInfoKiosk represents CT_ShowInfoKiosk (p:kiosk)
type ShowInfoKiosk struct {
	Restart uint32 `xml:"restart,attr,omitempty"` // restart time in ms
}

// CustomShowRef represents CT_CustomShowId (p:custShow)
type CustomShowRef struct {
	Id uint32 `xml:"id,attr"`
}

// ColorMRU represents CT_ColorMRU (p:clrMru) - most recently used colors
type ColorMRU struct {
	SrgbClr   []*dml.SrgbClr `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	SchemeClr []*dml.SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
}

// Note: there is no p:sldPr / "slide layout properties" / "slide master
// properties" complex type in pml.xsd — a layout's matchingName/type/preserve/
// userDrawn and a master's preserve are attributes of the part roots themselves
// (SlideLayout, SlideMaster in slide.go), and a slide's transition/timing/hf are
// root children. The three placeholder structs that used to sit here modeled
// nothing, had no production reference, and made two spec round-trip tests pass
// vacuously by standing in for unrelated elements (C527).

// Note: TxStyles is defined in slide.go

// Note: CustomShowList, CustomShow, PhotoAlbum, Kinsoku, ModifyVerifier,
// EmbeddedFontList, EmbeddedFont, EmbeddedFontData, and SmartTags are
// defined in presentation.go
