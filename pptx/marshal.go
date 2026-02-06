package pptx

import (
	"bytes"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// Namespace constants (local copies for convenience)
const (
	nsP = xmlb.NSPresentationML
	nsA = xmlb.NSDrawingML
	nsR = xmlb.NSPresentationRels
)

// marshalSlide marshals a slide to XML using the reflection-based marshaler.
func marshalSlide(slide *oxml.Slide) []byte {
	b := xmlb.NewPresentationMLBuilder()
	b.WriteHeader()
	b.MarshalRoot(nsP, "sld", slide, xmlb.PresentationMLNamespaces())
	result := b.Bytes()

	// Inject mc:AlternateContent raw XML if present.
	// It goes between clrMapOvr and timing (or before timing, or before </p:sld>).
	if slide.AlternateContent != nil && len(slide.AlternateContent.RawXML) > 0 {
		result = injectAlternateContent(result, slide.AlternateContent.RawXML)
	}

	return result
}

// injectAlternateContent inserts raw mc:AlternateContent XML into the slide output.
// The mc:AlternateContent element appears between clrMapOvr and timing per the XSD.
func injectAlternateContent(slideXML []byte, mcXML []byte) []byte {
	// Try to insert before <p:timing>
	if idx := bytes.Index(slideXML, []byte("<p:timing")); idx >= 0 {
		result := make([]byte, 0, len(slideXML)+len(mcXML))
		result = append(result, slideXML[:idx]...)
		result = append(result, mcXML...)
		result = append(result, slideXML[idx:]...)
		return result
	}
	// Try to insert before <p:extLst>
	if idx := bytes.Index(slideXML, []byte("<p:extLst")); idx >= 0 {
		result := make([]byte, 0, len(slideXML)+len(mcXML))
		result = append(result, slideXML[:idx]...)
		result = append(result, mcXML...)
		result = append(result, slideXML[idx:]...)
		return result
	}
	// Insert before </p:sld>
	if idx := bytes.Index(slideXML, []byte("</p:sld>")); idx >= 0 {
		result := make([]byte, 0, len(slideXML)+len(mcXML))
		result = append(result, slideXML[:idx]...)
		result = append(result, mcXML...)
		result = append(result, slideXML[idx:]...)
		return result
	}
	return slideXML
}

// marshalSlideLayout marshals a slide layout to XML using the reflection-based marshaler.
func marshalSlideLayout(layout *oxml.SlideLayout) []byte {
	b := xmlb.NewPresentationMLBuilder()
	b.WriteHeader()
	b.MarshalRoot(nsP, "sldLayout", layout, xmlb.PresentationMLNamespaces())
	return b.Bytes()
}

// marshalSlideMaster marshals a slide master to XML using the reflection-based marshaler.
func marshalSlideMaster(master *oxml.SlideMaster) []byte {
	b := xmlb.NewPresentationMLBuilder()
	b.WriteHeader()
	b.MarshalRoot(nsP, "sldMaster", master, xmlb.PresentationMLNamespaces())
	return b.Bytes()
}

// marshalPresentation marshals the presentation.xml using proper namespace prefixes.
func marshalPresentationXML(pres *oxml.Presentation) []byte {
	b := xmlb.NewPresentationMLBuilder()
	b.WriteHeader()

	// Build presentation attributes
	var presAttrs []xmlb.Attr

	// Add presentation-level attributes from parsed data
	if pres.SaveSubsetFonts != nil && *pres.SaveSubsetFonts {
		presAttrs = append(presAttrs, xmlb.StrAttr("saveSubsetFonts", "1"))
	}
	if pres.AutoCompressPictures != nil {
		if *pres.AutoCompressPictures {
			presAttrs = append(presAttrs, xmlb.StrAttr("autoCompressPictures", "1"))
		} else {
			presAttrs = append(presAttrs, xmlb.StrAttr("autoCompressPictures", "0"))
		}
	}
	if pres.EmbedTrueTypeFonts != nil && *pres.EmbedTrueTypeFonts {
		presAttrs = append(presAttrs, xmlb.StrAttr("embedTrueTypeFonts", "1"))
	}

	// Start root element with namespace declarations and attributes
	b.StartElementWithNS(nsP, "presentation", xmlb.PresentationMLNamespaces(), presAttrs...)

	// sldMasterIdLst
	if pres.SlideMasterIDs != nil && len(pres.SlideMasterIDs.SlideMasterID) > 0 {
		b.StartElement(nsP, "sldMasterIdLst")
		for _, master := range pres.SlideMasterIDs.SlideMasterID {
			b.EmptyElement(nsP, "sldMasterId",
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
			b.EmptyElement(nsP, "notesMasterId", xmlb.RelAttr("id", nm.RID))
		}
		b.EndElement(nsP, "notesMasterIdLst")
	}

	// handoutMasterIdLst
	if pres.HandoutMasterIDs != nil && len(pres.HandoutMasterIDs.HandoutMasterID) > 0 {
		b.StartElement(nsP, "handoutMasterIdLst")
		for _, hm := range pres.HandoutMasterIDs.HandoutMasterID {
			b.EmptyElement(nsP, "handoutMasterId", xmlb.RelAttr("id", hm.RID))
		}
		b.EndElement(nsP, "handoutMasterIdLst")
	}

	// sldIdLst
	if pres.SlideIDs != nil && len(pres.SlideIDs.SlideID) > 0 {
		b.StartElement(nsP, "sldIdLst")
		for _, slide := range pres.SlideIDs.SlideID {
			b.EmptyElement(nsP, "sldId",
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
		b.EmptyElement(nsP, "sldSz", sldSzAttrs...)
	}

	// notesSz
	if pres.NotesSize != nil {
		b.EmptyElement(nsP, "notesSz",
			xmlb.IntAttr("cx", pres.NotesSize.Cx),
			xmlb.IntAttr("cy", pres.NotesSize.Cy),
		)
	}

	// defaultTextStyle - use parsed data if available, otherwise default
	if pres.DefaultTextStyle != nil {
		marshalParsedDefaultTextStyle(b, pres.DefaultTextStyle)
	} else {
		marshalDefaultTextStyle(b)
	}

	b.EndElement(nsP, "presentation")
	return b.Bytes()
}

// marshalParsedDefaultTextStyle writes the defaultTextStyle element from parsed data.
func marshalParsedDefaultTextStyle(b *xmlb.Builder, style *oxml.TextListStyle) {
	b.StartElement(nsP, "defaultTextStyle")

	// defPPr
	if style.DefPPr != nil {
		marshalParsedParagraphProps(b, "defPPr", style.DefPPr)
	}

	// Level 1-9 paragraph properties
	levels := []*oxml.TextParagraphProperties{
		style.Lvl1PPr, style.Lvl2PPr, style.Lvl3PPr, style.Lvl4PPr, style.Lvl5PPr,
		style.Lvl6PPr, style.Lvl7PPr, style.Lvl8PPr, style.Lvl9PPr,
	}
	names := []string{"lvl1pPr", "lvl2pPr", "lvl3pPr", "lvl4pPr", "lvl5pPr", "lvl6pPr", "lvl7pPr", "lvl8pPr", "lvl9pPr"}
	for i, level := range levels {
		if level != nil {
			marshalParsedParagraphProps(b, names[i], level)
		}
	}

	b.EndElement(nsP, "defaultTextStyle")
}

// marshalParsedParagraphProps writes a paragraph properties element from parsed data.
func marshalParsedParagraphProps(b *xmlb.Builder, name string, props *oxml.TextParagraphProperties) {
	var attrs []xmlb.Attr
	if props.MarL != nil {
		attrs = append(attrs, xmlb.IntAttr("marL", *props.MarL))
	}
	if props.Algn != "" {
		attrs = append(attrs, xmlb.StrAttr("algn", props.Algn))
	}
	if props.DefTabSz != nil {
		attrs = append(attrs, xmlb.IntAttr("defTabSz", *props.DefTabSz))
	}
	if props.Rtl != nil {
		if *props.Rtl {
			attrs = append(attrs, xmlb.StrAttr("rtl", "1"))
		} else {
			attrs = append(attrs, xmlb.StrAttr("rtl", "0"))
		}
	}
	if props.EaLnBrk != nil {
		if *props.EaLnBrk {
			attrs = append(attrs, xmlb.StrAttr("eaLnBrk", "1"))
		} else {
			attrs = append(attrs, xmlb.StrAttr("eaLnBrk", "0"))
		}
	}
	if props.LatinLnBrk != nil {
		if *props.LatinLnBrk {
			attrs = append(attrs, xmlb.StrAttr("latinLnBrk", "1"))
		} else {
			attrs = append(attrs, xmlb.StrAttr("latinLnBrk", "0"))
		}
	}
	if props.HangingPunct != nil {
		if *props.HangingPunct {
			attrs = append(attrs, xmlb.StrAttr("hangingPunct", "1"))
		} else {
			attrs = append(attrs, xmlb.StrAttr("hangingPunct", "0"))
		}
	}

	b.StartElement(nsA, name, attrs...)

	// defRPr
	if props.DefRPr != nil {
		marshalParsedCharacterProps(b, "defRPr", props.DefRPr)
	}

	b.EndElement(nsA, name)
}

// marshalParsedCharacterProps writes character properties from parsed data.
func marshalParsedCharacterProps(b *xmlb.Builder, name string, props *oxml.TextCharacterProperties) {
	var attrs []xmlb.Attr
	if props.Lang != "" {
		attrs = append(attrs, xmlb.StrAttr("lang", props.Lang))
	}
	if props.Sz != nil {
		attrs = append(attrs, xmlb.Int32Attr("sz", *props.Sz))
	}
	if props.Kern != nil {
		attrs = append(attrs, xmlb.Int32Attr("kern", *props.Kern))
	}

	// Check if we have any child elements
	hasSolidFill := props.SolidFill != nil
	hasLatin := props.Latin != nil
	hasEa := props.Ea != nil
	hasCs := props.Cs != nil
	hasChildren := hasSolidFill || hasLatin || hasEa || hasCs

	if hasChildren {
		b.StartElement(nsA, name, attrs...)

		if hasSolidFill && props.SolidFill.SchemeClr != nil {
			b.StartElement(nsA, "solidFill")
			b.EmptyElement(nsA, "schemeClr", xmlb.StrAttr("val", props.SolidFill.SchemeClr.Val))
			b.EndElement(nsA, "solidFill")
		}

		if hasLatin {
			b.EmptyElement(nsA, "latin", xmlb.StrAttr("typeface", props.Latin.Typeface))
		}
		if hasEa {
			b.EmptyElement(nsA, "ea", xmlb.StrAttr("typeface", props.Ea.Typeface))
		}
		if hasCs {
			b.EmptyElement(nsA, "cs", xmlb.StrAttr("typeface", props.Cs.Typeface))
		}

		b.EndElement(nsA, name)
	} else {
		b.EmptyElement(nsA, name, attrs...)
	}
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
