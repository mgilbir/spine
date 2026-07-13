package pptx

import (
	"fmt"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// Namespace constants (local copies for convenience)
const (
	nsP = xmlb.NSPresentationML
	nsA = xmlb.NSDrawingML
	nsR = xmlb.NSOfficeDocumentRels
)

// marshalSlide marshals a slide to XML, keeping root-level AlternateContent
// siblings in their parsed positions (C223).
func marshalSlide(slide *oxml.Slide) ([]byte, error) {
	b := xmlb.NewPresentationMLBuilder()
	b.SetSelfClosingSpace(slide.SelfClosingSpace)
	b.SetCollapseEmptyElements(slide.CollapseEmpty)
	b.WriteProlog(slide.Prolog)
	slide.MarshalRootToBuilder(b)
	b.WriteTrailer(slide.Prolog)
	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("pptx: marshal slide: %w", err)
	}
	return b.Bytes(), nil
}

// marshalSlideLayout marshals a slide layout to XML (see marshalSlide).
func marshalSlideLayout(layout *oxml.SlideLayout) ([]byte, error) {
	b := xmlb.NewPresentationMLBuilder()
	b.SetSelfClosingSpace(layout.SelfClosingSpace)
	b.SetCollapseEmptyElements(layout.CollapseEmpty)
	b.WriteProlog(layout.Prolog)
	layout.MarshalRootToBuilder(b)
	b.WriteTrailer(layout.Prolog)
	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("pptx: marshal slide layout: %w", err)
	}
	return b.Bytes(), nil
}

// marshalSlideMaster marshals a slide master to XML (see marshalSlide).
func marshalSlideMaster(master *oxml.SlideMaster) ([]byte, error) {
	b := xmlb.NewPresentationMLBuilder()
	b.SetSelfClosingSpace(master.SelfClosingSpace)
	b.SetCollapseEmptyElements(master.CollapseEmpty)
	b.WriteProlog(master.Prolog)
	master.MarshalRootToBuilder(b)
	b.WriteTrailer(master.Prolog)
	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("pptx: marshal slide master: %w", err)
	}
	return b.Bytes(), nil
}

// marshalPresentation marshals the presentation.xml using proper namespace prefixes.
// synthesizeDefaults controls whether a defaultTextStyle is fabricated when the
// model has none: true for decks created programmatically (PowerPoint expects
// one in new files), false for opened decks (a deck that never had a
// defaultTextStyle must not gain invented document-wide text defaults on save).
func marshalPresentationXML(pres *oxml.Presentation, synthesizeDefaults bool) ([]byte, error) {
	b := xmlb.NewPresentationMLBuilder()
	b.SetSelfClosingSpace(pres.SelfClosingSpace)
	b.SetCollapseEmptyElements(pres.CollapseEmpty)
	b.WriteProlog(pres.Prolog)

	// Build presentation attributes in PowerPoint's emission order (the XSD
	// attribute order): serverZoom, firstSlideNum, showSpecialPlsOnTitleSld,
	// rtl, removePersonalInfoOnSave, compatMode, strictFirstAndLastChars,
	// embedTrueTypeFonts, saveSubsetFonts, autoCompressPictures,
	// bookmarkIdSeed, conformance.
	var presAttrs []xmlb.Attr

	appendBool := func(name string, v *bool) {
		if v == nil {
			return
		}
		val := "0"
		if *v {
			val = "1"
		}
		presAttrs = append(presAttrs, xmlb.StrAttr(name, val))
	}
	if pres.ServerZoom != "" {
		presAttrs = append(presAttrs, xmlb.StrAttr("serverZoom", pres.ServerZoom))
	}
	if pres.FirstSlideNum != nil {
		presAttrs = append(presAttrs, xmlb.IntAttr("firstSlideNum", int64(*pres.FirstSlideNum)))
	}
	appendBool("showSpecialPlsOnTitleSld", pres.ShowSpecialPlsOnTitleSld)
	appendBool("rtl", pres.Rtl)
	appendBool("removePersonalInfoOnSave", pres.RemovePersonalInfoOnSave)
	appendBool("compatMode", pres.CompatMode)
	appendBool("strictFirstAndLastChars", pres.StrictFirstAndLastChars)
	if pres.EmbedTrueTypeFonts != nil && *pres.EmbedTrueTypeFonts {
		presAttrs = append(presAttrs, xmlb.StrAttr("embedTrueTypeFonts", "1"))
	}
	if pres.SaveSubsetFonts != nil && *pres.SaveSubsetFonts {
		presAttrs = append(presAttrs, xmlb.StrAttr("saveSubsetFonts", "1"))
	}
	appendBool("autoCompressPictures", pres.AutoCompressPictures)
	if pres.BookmarkIdSeed != nil {
		presAttrs = append(presAttrs, xmlb.UintAttr("bookmarkIdSeed", *pres.BookmarkIdSeed))
	}
	if pres.Conformance != "" {
		presAttrs = append(presAttrs, xmlb.StrAttr("conformance", pres.Conformance))
	}

	// Start root element with namespace declarations and attributes. A parsed
	// deck replays the source's verbatim attribute list (declarations beyond
	// a/r/p, producer attribute order, mc:Ignorable and friends); the
	// XSD-order emission above serves programmatically built decks.
	if pres.OriginalRootAttrs != nil {
		b.StartElementWithRootAttrs(nsP, "presentation", pres.OriginalRootAttrs)
	} else {
		b.StartElementWithNS(nsP, "presentation", xmlb.PresentationMLNamespaces(), presAttrs...)
	}

	// idEntry writes an sldId-family list entry, emitting the optional extLst
	// child when the parsed entry carried one (C225).
	idEntry := func(name string, extLst *oxml.ExtensionList, attrs ...xmlb.Attr) {
		if extLst == nil {
			b.EmptyElement(nsP, name, attrs...)
			return
		}
		b.StartElement(nsP, name, attrs...)
		b.MarshalElement(nsP, "extLst", extLst)
		b.EndElement(nsP, name)
	}

	// sldMasterIdLst
	if pres.SlideMasterIDs != nil && len(pres.SlideMasterIDs.SlideMasterID) > 0 {
		b.StartElement(nsP, "sldMasterIdLst")
		for _, master := range pres.SlideMasterIDs.SlideMasterID {
			idEntry("sldMasterId", master.ExtLst,
				xmlb.UintAttr("id", master.ID),
				xmlb.RelAttr("id", master.RID),
			)
		}
		b.EndElement(nsP, "sldMasterIdLst")
	}

	// notesMasterIdLst
	if pres.NotesMasterIDs != nil && len(pres.NotesMasterIDs.NotesMasterID) > 0 {
		b.StartElement(nsP, "notesMasterIdLst")
		for _, nm := range pres.NotesMasterIDs.NotesMasterID {
			idEntry("notesMasterId", nm.ExtLst, xmlb.RelAttr("id", nm.RID))
		}
		b.EndElement(nsP, "notesMasterIdLst")
	}

	// handoutMasterIdLst
	if pres.HandoutMasterIDs != nil && len(pres.HandoutMasterIDs.HandoutMasterID) > 0 {
		b.StartElement(nsP, "handoutMasterIdLst")
		for _, hm := range pres.HandoutMasterIDs.HandoutMasterID {
			idEntry("handoutMasterId", hm.ExtLst, xmlb.RelAttr("id", hm.RID))
		}
		b.EndElement(nsP, "handoutMasterIdLst")
	}

	// sldIdLst
	if pres.SlideIDs != nil && len(pres.SlideIDs.SlideID) > 0 {
		b.StartElement(nsP, "sldIdLst")
		for _, slide := range pres.SlideIDs.SlideID {
			idEntry("sldId", slide.ExtLst,
				xmlb.UintAttr("id", slide.ID),
				xmlb.RelAttr("id", slide.RID),
			)
		}
		b.EndElement(nsP, "sldIdLst")
	}

	// sldSz - include type if present
	if pres.SlideSize != nil {
		sldSzAttrs := []xmlb.Attr{
			xmlb.IntAttr("cx", pres.SlideSize.Cx),
			xmlb.IntAttr("cy", pres.SlideSize.Cy),
		}
		if pres.SlideSize.Type != "" {
			sldSzAttrs = append(sldSzAttrs, xmlb.StrAttr("type", pres.SlideSize.Type))
		}
		if pres.SlideSize.CapturedAttrs != nil {
			sldSzAttrs = b.ReplayCapturedAttrs(pres.SlideSize.CapturedAttrs, sldSzAttrs)
		}
		b.EmptyElement(nsP, "sldSz", sldSzAttrs...)
	}

	// notesSz
	if pres.NotesSize != nil {
		notesSzAttrs := []xmlb.Attr{
			xmlb.IntAttr("cx", pres.NotesSize.Cx),
			xmlb.IntAttr("cy", pres.NotesSize.Cy),
		}
		if pres.NotesSize.CapturedAttrs != nil {
			notesSzAttrs = b.ReplayCapturedAttrs(pres.NotesSize.CapturedAttrs, notesSzAttrs)
		}
		b.EmptyElement(nsP, "notesSz", notesSzAttrs...)
	}

	// smartTags .. kinsoku: parsed-but-previously-dropped children, emitted in
	// their schema position (between notesSz and defaultTextStyle) when present.
	if pres.SmartTags != nil {
		b.MarshalElement(nsP, "smartTags", pres.SmartTags)
	}
	if pres.EmbeddedFontLst != nil {
		b.MarshalElement(nsP, "embeddedFontLst", pres.EmbeddedFontLst)
	}
	if pres.CustShowLst != nil {
		b.MarshalElement(nsP, "custShowLst", pres.CustShowLst)
	}
	if pres.PhotoAlbum != nil {
		b.MarshalElement(nsP, "photoAlbum", pres.PhotoAlbum)
	}
	if pres.CustDataLst != nil {
		b.MarshalElement(nsP, "custDataLst", pres.CustDataLst)
	}
	if pres.Kinsoku != nil {
		b.MarshalElement(nsP, "kinsoku", pres.Kinsoku)
	}

	// defaultTextStyle - use parsed data if available; fabricate one only for
	// newly created decks. The parsed style is the full dml CT_TextListStyle,
	// so bullets, line spacing, tabs, and every fill/color kind survive the
	// regeneration (C91: a hand-rolled writer previously dropped srgbClr
	// fills, tint/shade transforms, and all bullet/spacing children).
	if pres.DefaultTextStyle != nil {
		b.MarshalElement(nsP, "defaultTextStyle", pres.DefaultTextStyle)
	} else if synthesizeDefaults {
		marshalDefaultTextStyle(b)
	}

	// modifyVerifier (password-to-modify) — parsed but previously dropped.
	if pres.ModifyVerifier != nil {
		b.MarshalElement(nsP, "modifyVerifier", pres.ModifyVerifier)
	}

	// extLst
	if pres.ExtLst != nil && len(pres.ExtLst.Ext) > 0 {
		b.StartElement(nsP, "extLst")
		for i := range pres.ExtLst.Ext {
			pres.ExtLst.Ext[i].MarshalToBuilder(b, nsP, "ext")
		}
		b.EndElement(nsP, "extLst")
	}

	b.EndElement(nsP, "presentation")
	b.WriteTrailer(pres.Prolog)
	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("pptx: marshal presentation.xml: %w", err)
	}
	return b.Bytes(), nil
}

// marshalDefaultTextStyle writes the defaultTextStyle element with all 9 levels.
func marshalDefaultTextStyle(b *xmlb.Builder) {
	b.StartElement(nsP, "defaultTextStyle")

	// defPPr
	b.StartElement(nsA, "defPPr")
	b.EmptyElement(nsA, "defRPr", xmlb.StrAttr("lang", "en-US"))
	b.EndElement(nsA, "defPPr")

	// Level 1-9 paragraph properties
	margins := []int64{0, 457200, 914400, 1371600, 1828800, 2286000, 2743200, 3200400, 3657600}
	for i := 1; i <= 9; i++ {
		marshalDefaultLevelPPr(b, i, margins[i-1])
	}

	b.EndElement(nsP, "defaultTextStyle")
}

// marshalDefaultLevelPPr writes a level paragraph properties element.
func marshalDefaultLevelPPr(b *xmlb.Builder, level int, marL int64) {
	levelName := "lvl" + string(rune('0'+level)) + "pPr"
	b.StartElement(nsA, levelName,
		xmlb.IntAttr("marL", marL),
		xmlb.StrAttr("algn", "l"),
		xmlb.IntAttr("defTabSz", 914400),
		xmlb.BoolAttr("rtl", false),
		xmlb.BoolAttr("eaLnBrk", true),
		xmlb.BoolAttr("latinLnBrk", false),
		xmlb.BoolAttr("hangingPunct", true),
	)

	// defRPr with font settings
	b.StartElement(nsA, "defRPr",
		xmlb.IntAttr("sz", 1800),
		xmlb.IntAttr("kern", 1200),
	)

	// solidFill with scheme color
	b.StartElement(nsA, "solidFill")
	b.EmptyElement(nsA, "schemeClr", xmlb.StrAttr("val", "tx1"))
	b.EndElement(nsA, "solidFill")

	// Font typefaces
	b.EmptyElement(nsA, "latin", xmlb.StrAttr("typeface", "+mn-lt"))
	b.EmptyElement(nsA, "ea", xmlb.StrAttr("typeface", "+mn-ea"))
	b.EmptyElement(nsA, "cs", xmlb.StrAttr("typeface", "+mn-cs"))

	b.EndElement(nsA, "defRPr")
	b.EndElement(nsA, levelName)
}
