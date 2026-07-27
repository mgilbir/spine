// This file provides PresentationML view property types from pml.xsd.
// These types implement the p: namespace view elements.

package oxml

import "github.com/mgilbir/spine/common/dml"

// ViewProperties represents CT_ViewProperties (p:viewPr).
// showComments defaults to TRUE, so it is a *bool: parsing an explicit "0" into
// a plain bool and re-marshaling would delete the attribute and let a reader
// reapply the default true (the C29/C316/C317 rule, C526).
type ViewProperties struct {
	LastView       string                  `xml:"lastView,attr,omitempty"` // sldView, sldMasterView, notesView, handoutView, notesMasterView, outlineView, sldSorterView, sldThumbnailView
	ShowComments   *bool                   `xml:"showComments,attr,omitempty"`
	NormalViewPr   *NormalViewProperties   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main normalViewPr,omitempty"`
	SlideViewPr    *SlideViewProperties    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main slideViewPr,omitempty"`
	OutlineViewPr  *OutlineViewProperties  `xml:"http://schemas.openxmlformats.org/presentationml/2006/main outlineViewPr,omitempty"`
	NotesTextViewPr *NotesTextViewProperties `xml:"http://schemas.openxmlformats.org/presentationml/2006/main notesTextViewPr,omitempty"`
	SorterViewPr   *SorterViewProperties   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sorterViewPr,omitempty"`
	NotesViewPr    *NotesViewProperties    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main notesViewPr,omitempty"`
	GridSpacing    *dml.OffXML             `xml:"http://schemas.openxmlformats.org/presentationml/2006/main gridSpacing,omitempty"`
	ExtLst         *ExtensionList             `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
}

// NormalViewProperties represents CT_NormalViewProperties (p:normalViewPr).
// showOutlineIcons defaults to TRUE (see ViewProperties).
type NormalViewProperties struct {
	ShowOutlineIcons *bool  `xml:"showOutlineIcons,attr,omitempty"`
	SnapVertSplitter bool   `xml:"snapVertSplitter,attr,omitempty"`
	VertBarState     string `xml:"vertBarState,attr,omitempty"` // minimized, restored, maximized
	HorzBarState     string `xml:"horzBarState,attr,omitempty"` // minimized, restored, maximized
	PreferSingleView bool   `xml:"preferSingleView,attr,omitempty"`
	RestoredLeft     *NormalViewPortion `xml:"http://schemas.openxmlformats.org/presentationml/2006/main restoredLeft,omitempty"`
	RestoredTop      *NormalViewPortion `xml:"http://schemas.openxmlformats.org/presentationml/2006/main restoredTop,omitempty"`
}

// NormalViewPortion represents CT_NormalViewPortion (p:restoredLeft, p:restoredTop).
// autoAdjust defaults to TRUE (see ViewProperties).
type NormalViewPortion struct {
	Sz       int32 `xml:"sz,attr,omitempty"`       // size in percentage
	AutoAdjust *bool `xml:"autoAdjust,attr,omitempty"`
}

// SlideViewProperties represents CT_SlideViewProperties (p:slideViewPr)
type SlideViewProperties struct {
	CSldViewPr *CommonSlideViewProperties `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cSldViewPr,omitempty"`
}

// CommonSlideViewProperties represents CT_CommonSlideViewProperties (p:cSldViewPr).
// snapToGrid defaults to TRUE (see ViewProperties); snapToObjects and showGuides
// default to false.
type CommonSlideViewProperties struct {
	SnapToGrid   *bool             `xml:"snapToGrid,attr,omitempty"`
	SnapToObjects bool             `xml:"snapToObjects,attr,omitempty"`
	ShowGuides   bool              `xml:"showGuides,attr,omitempty"`
	CViewPr      *CommonViewProperties `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cViewPr,omitempty"`
	GuideLst     *GuideList        `xml:"http://schemas.openxmlformats.org/presentationml/2006/main guideLst,omitempty"`
}

// CommonViewProperties represents CT_CommonViewProperties (p:cViewPr)
type CommonViewProperties struct {
	VarScale  bool     `xml:"varScale,attr,omitempty"`
	Scale     *ScalePoint `xml:"http://schemas.openxmlformats.org/presentationml/2006/main scale,omitempty"`
	Origin    *dml.OffXML `xml:"http://schemas.openxmlformats.org/presentationml/2006/main origin,omitempty"`
}

// ScalePoint represents CT_Scale2D (a:scale)
type ScalePoint struct {
	Sx *Ratio `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sx,omitempty"`
	Sy *Ratio `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sy,omitempty"`
}

// Ratio represents CT_Ratio (a:sx, a:sy)
type Ratio struct {
	N int32 `xml:"n,attr"` // numerator
	D int32 `xml:"d,attr"` // denominator
}

// GuideList represents CT_GuideList (p:guideLst)
type GuideList struct {
	Guide []*Guide `xml:"http://schemas.openxmlformats.org/presentationml/2006/main guide,omitempty"`
}

// Guide represents CT_Guide (p:guide)
type Guide struct {
	Orient string `xml:"orient,attr,omitempty"` // horz, vert
	Pos    int32  `xml:"pos,attr,omitempty"`    // position in EMUs/8
}

// OutlineViewProperties represents CT_OutlineViewProperties (p:outlineViewPr)
type OutlineViewProperties struct {
	CViewPr *CommonViewProperties `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cViewPr,omitempty"`
	SldLst  *OutlineViewSlideList `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sldLst,omitempty"`
}

// OutlineViewSlideList represents CT_OutlineViewSlideList (p:sldLst)
type OutlineViewSlideList struct {
	Sld []*OutlineViewSlideEntry `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sld,omitempty"`
}

// OutlineViewSlideEntry represents CT_OutlineViewSlideEntry (p:sld)
type OutlineViewSlideEntry struct {
	Id       string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
	Collapse bool   `xml:"collapse,attr,omitempty"`
}

// NotesTextViewProperties represents CT_NotesTextViewProperties (p:notesTextViewPr)
type NotesTextViewProperties struct {
	CViewPr *CommonViewProperties `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cViewPr,omitempty"`
}

// SorterViewProperties represents CT_SlideSorterViewProperties (p:sorterViewPr).
// showFormatting defaults to TRUE (see ViewProperties).
type SorterViewProperties struct {
	CViewPr    *CommonViewProperties `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cViewPr,omitempty"`
	ShowFormatting *bool `xml:"showFormatting,attr,omitempty"`
}

// NotesViewProperties represents CT_NotesViewProperties (p:notesViewPr)
type NotesViewProperties struct {
	CSldViewPr *CommonSlideViewProperties `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cSldViewPr,omitempty"`
}
