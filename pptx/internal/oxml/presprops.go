// Package oxml provides PresentationML presentation property types from pml.xsd.
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
	ExtLst       *dml.ExtLst              `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
}

// HtmlPublishProperties represents CT_HtmlPublishProperties (p:htmlPubPr)
type HtmlPublishProperties struct {
	ShowSpeakerNotes bool    `xml:"showSpeakerNotes,attr,omitempty"`
	PubBrowser       string  `xml:"pubBrowser,attr,omitempty"` // v4, v3, v3v4
	Title            string  `xml:"title,attr,omitempty"`
	Id               string  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
	ExtLst           *dml.ExtLst `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
}

// WebProperties represents CT_WebProperties (p:webPr)
type WebProperties struct {
	ShowAnimation     bool   `xml:"showAnimation,attr,omitempty"`
	ResizeGraphics    bool   `xml:"resizeGraphics,attr,omitempty"`
	AllowPng          bool   `xml:"allowPng,attr,omitempty"`
	RelyOnVml         bool   `xml:"relyOnVml,attr,omitempty"`
	OrganizeInFolders bool   `xml:"organizeInFolders,attr,omitempty"`
	UseLongFilenames  bool   `xml:"useLongFilenames,attr,omitempty"`
	ImgSz             string `xml:"imgSz,attr,omitempty"` // screen640x480, screen800x600, screen1024x768, screen1152x882, screen1152x900, screen1280x1024, screen1600x1200, screen1800x1440, screen1920x1200
	Encoding          string `xml:"encoding,attr,omitempty"`
	Clr               string `xml:"clr,attr,omitempty"` // none, browser, presentationText, presentationAccent, whiteTextOnBlack, blackTextOnWhite
	ExtLst            *dml.ExtLst `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
}

// PrintProperties represents CT_PrintProperties (p:prnPr)
type PrintProperties struct {
	PrnWhat         string `xml:"prnWhat,attr,omitempty"`      // slides, handouts1, handouts2, handouts3, handouts4, handouts6, handouts9, notes, outline
	ClrMode         string `xml:"clrMode,attr,omitempty"`      // bw, gray, clr
	HiddenSlides    bool   `xml:"hiddenSlides,attr,omitempty"`
	ScaleToFitPaper bool   `xml:"scaleToFitPaper,attr,omitempty"`
	FrameSlides     bool   `xml:"frameSlides,attr,omitempty"`
	ExtLst          *dml.ExtLst `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
}

// ShowProperties represents CT_ShowProperties (p:showPr)
type ShowProperties struct {
	Loop         bool   `xml:"loop,attr,omitempty"`
	ShowNarration bool  `xml:"showNarration,attr,omitempty"`
	ShowAnimation bool  `xml:"showAnimation,attr,omitempty"`
	UseTimings   bool   `xml:"useTimings,attr,omitempty"`
	Present      *ShowInfoPresent `xml:"http://schemas.openxmlformats.org/presentationml/2006/main present,omitempty"`
	Browse       *ShowInfoBrowse  `xml:"http://schemas.openxmlformats.org/presentationml/2006/main browse,omitempty"`
	Kiosk        *ShowInfoKiosk   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main kiosk,omitempty"`
	SldAll       *EmptyElement    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sldAll,omitempty"`
	SldRg        *IndexRange      `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sldRg,omitempty"`
	CustShow     *CustomShowRef   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main custShow,omitempty"`
	PenClr       *dml.ColorChoice `xml:"http://schemas.openxmlformats.org/presentationml/2006/main penClr,omitempty"`
	ExtLst       *dml.ExtLst      `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
}

// EmptyElement represents CT_Empty
type EmptyElement struct{}

// ShowInfoPresent represents CT_ShowInfoPresent (p:present)
type ShowInfoPresent struct{}

// ShowInfoBrowse represents CT_ShowInfoBrowse (p:browse)
type ShowInfoBrowse struct {
	ShowScrollbar bool `xml:"showScrollbar,attr,omitempty"`
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

// --- Slide Properties ---

// SlideProperties represents CT_SlideProperties (p:sldPr) - for individual slide
type SlideProperties struct {
	Transition *Transition   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main transition,omitempty"`
	Timing     *Timing       `xml:"http://schemas.openxmlformats.org/presentationml/2006/main timing,omitempty"`
	Hf         *HeaderFooter `xml:"http://schemas.openxmlformats.org/presentationml/2006/main hf,omitempty"`
}

// --- Slide Layout Properties ---

// SlideLayoutProperties represents additional slide layout properties
type SlideLayoutProperties struct {
	MatchingName string `xml:"matchingName,attr,omitempty"`
	Type         string `xml:"type,attr,omitempty"` // title, obj, secHead, twoObj, twoTxTwoObj, etc.
	Preserve     bool   `xml:"preserve,attr,omitempty"`
	UserDrawn    bool   `xml:"userDrawn,attr,omitempty"`
}

// --- Slide Master Properties ---

// SlideMasterProperties represents additional slide master properties
type SlideMasterProperties struct {
	Preserve bool `xml:"preserve,attr,omitempty"`
}

// Note: TxStyles is defined in slide.go

// Note: CustomShowList, CustomShow, PhotoAlbum, Kinsoku, ModifyVerifier,
// EmbeddedFontList, EmbeddedFont, EmbeddedFontData, and SmartTags are
// defined in presentation.go
