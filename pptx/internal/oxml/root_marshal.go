package oxml

import (
	"encoding/xml"
	"strconv"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// Slide-part roots (p:sld, p:sldLayout, p:sldMaster) can carry any number of
// root-level mc:AlternateContent siblings, interleaved with the typed schema
// children (C223). Each AC records an anchor — the local name of the typed
// child that preceded it ("" when it came first) — and marshaling walks the
// schema sequence, emitting each typed child followed by the ACs anchored to
// it. Typed children set or cleared programmatically after parse therefore
// still (dis)appear at their schema position, while parsed ACs keep theirs.

// acDefaultAnchor is where programmatically appended AlternateContent lands:
// after the transition, matching the historical struct field position.
const acDefaultAnchor = "transition"

func acAnchorAt(anchors []string, i int) string {
	if i < len(anchors) {
		return anchors[i]
	}
	return acDefaultAnchor
}

// parseXSDBool parses an xsd:boolean attribute value, ignoring invalid input
// (matching encoding/xml, which leaves the field untouched on parse errors
// only for pointers; invalid values are rare enough to treat as absent).
func parseXSDBool(v string) *bool {
	b, err := strconv.ParseBool(v)
	if err != nil {
		return nil
	}
	return &b
}

// --- Slide ---

// UnmarshalXML parses a p:sld root, tracking the position of every root-level
// mc:AlternateContent relative to the typed children.
func (s *Slide) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	s.XMLName = start.Name
	s.OriginalRootAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	for _, attr := range start.Attr {
		if attr.Name.Space != "" {
			continue
		}
		switch attr.Name.Local {
		case "showMasterSp":
			s.ShowMasterSp = parseXSDBool(attr.Value)
		case "showMasterPhAnim":
			s.ShowMasterPhAnim = parseXSDBool(attr.Value)
		case "show":
			s.Show = parseXSDBool(attr.Value)
		}
	}
	anchor := ""
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Space == xmlb.NSMarkupCompatibility && t.Name.Local == "AlternateContent" {
				ac := &AlternateContent{}
				if err := d.DecodeElement(ac, &t); err != nil {
					return err
				}
				s.AlternateContent = append(s.AlternateContent, ac)
				s.acAnchors = append(s.acAnchors, anchor)
				continue
			}
			switch t.Name.Local {
			case "cSld":
				s.CSld = &CommonSlideData{}
				if err := d.DecodeElement(s.CSld, &t); err != nil {
					return err
				}
			case "clrMapOvr":
				s.ClrMapOvr = &ColorMapOverride{}
				if err := d.DecodeElement(s.ClrMapOvr, &t); err != nil {
					return err
				}
			case "transition":
				s.Transition = &Transition{}
				if err := d.DecodeElement(s.Transition, &t); err != nil {
					return err
				}
			case "timing":
				s.Timing = &Timing{}
				if err := d.DecodeElement(s.Timing, &t); err != nil {
					return err
				}
			case "extLst":
				s.ExtLst = &ExtensionList{}
				if err := d.DecodeElement(s.ExtLst, &t); err != nil {
					return err
				}
			default:
				if err := d.Skip(); err != nil {
					return err
				}
				continue
			}
			anchor = t.Name.Local
		case xml.EndElement:
			return nil
		}
	}
}

// marshalRootChildren writes the slide's children in schema order with the
// AlternateContent siblings restored to their anchored positions.
func (s *Slide) marshalRootChildren(b *xmlb.Builder) {
	emitAC := func(anchor string) {
		for i, ac := range s.AlternateContent {
			if acAnchorAt(s.acAnchors, i) == anchor {
				b.MarshalElement(xmlb.NSMarkupCompatibility, "AlternateContent", ac)
			}
		}
	}
	emitAC("")
	if s.CSld != nil {
		b.MarshalElement(xmlb.NSPresentationML, "cSld", s.CSld)
	}
	emitAC("cSld")
	if s.ClrMapOvr != nil {
		b.MarshalElement(xmlb.NSPresentationML, "clrMapOvr", s.ClrMapOvr)
	}
	emitAC("clrMapOvr")
	if s.Transition != nil {
		b.MarshalElement(xmlb.NSPresentationML, "transition", s.Transition)
	}
	emitAC("transition")
	if s.Timing != nil {
		b.MarshalElement(xmlb.NSPresentationML, "timing", s.Timing)
	}
	emitAC("timing")
	if s.ExtLst != nil {
		b.MarshalElement(xmlb.NSPresentationML, "extLst", s.ExtLst)
	}
	emitAC("extLst")
}

func (s *Slide) rootAttrs() []xmlb.Attr {
	var attrs []xmlb.Attr
	if s.ShowMasterSp != nil {
		attrs = append(attrs, xmlb.BoolAttr("showMasterSp", *s.ShowMasterSp))
	}
	if s.ShowMasterPhAnim != nil {
		attrs = append(attrs, xmlb.BoolAttr("showMasterPhAnim", *s.ShowMasterPhAnim))
	}
	if s.Show != nil {
		attrs = append(attrs, xmlb.BoolAttr("show", *s.Show))
	}
	return attrs
}

// MarshalToBuilder implements xmlb.BuilderMarshaler (non-root contexts).
func (s *Slide) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName, s.rootAttrs()...)
	s.marshalRootChildren(b)
	b.EndElement(ns, localName)
}

// MarshalRootToBuilder writes the p:sld root element with the standard
// PresentationML namespace declarations.
func (s *Slide) MarshalRootToBuilder(b *xmlb.Builder) {
	if s.OriginalRootAttrs != nil {
		b.StartElementWithRootAttrs(xmlb.NSPresentationML, "sld", s.OriginalRootAttrs)
	} else {
		b.StartElementWithNS(xmlb.NSPresentationML, "sld", xmlb.PresentationMLNamespaces(), s.rootAttrs()...)
	}
	s.marshalRootChildren(b)
	b.EndElement(xmlb.NSPresentationML, "sld")
}

// --- SlideLayout ---

// UnmarshalXML parses a p:sldLayout root (see Slide.UnmarshalXML).
func (sl *SlideLayout) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	sl.XMLName = start.Name
	sl.OriginalRootAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	for _, attr := range start.Attr {
		if attr.Name.Space != "" {
			continue
		}
		switch attr.Name.Local {
		case "showMasterSp":
			sl.ShowMasterSp = parseXSDBool(attr.Value)
		case "showMasterPhAnim":
			sl.ShowMasterPhAnim = parseXSDBool(attr.Value)
		case "type":
			sl.Type = attr.Value
		case "preserve":
			if v := parseXSDBool(attr.Value); v != nil {
				sl.Preserve = *v
			}
		case "userDrawn":
			if v := parseXSDBool(attr.Value); v != nil {
				sl.UserDrawn = *v
			}
		case "matchingName":
			sl.MatchingName = attr.Value
			sl.MatchingNamePresent = true
		}
	}
	anchor := ""
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Space == xmlb.NSMarkupCompatibility && t.Name.Local == "AlternateContent" {
				ac := &AlternateContent{}
				if err := d.DecodeElement(ac, &t); err != nil {
					return err
				}
				sl.AlternateContent = append(sl.AlternateContent, ac)
				sl.acAnchors = append(sl.acAnchors, anchor)
				continue
			}
			switch t.Name.Local {
			case "cSld":
				sl.CSld = &CommonSlideData{}
				if err := d.DecodeElement(sl.CSld, &t); err != nil {
					return err
				}
			case "clrMapOvr":
				sl.ClrMapOvr = &ColorMapOverride{}
				if err := d.DecodeElement(sl.ClrMapOvr, &t); err != nil {
					return err
				}
			case "transition":
				sl.Transition = &Transition{}
				if err := d.DecodeElement(sl.Transition, &t); err != nil {
					return err
				}
			case "timing":
				sl.Timing = &Timing{}
				if err := d.DecodeElement(sl.Timing, &t); err != nil {
					return err
				}
			case "hf":
				sl.Hf = &HeaderFooter{}
				if err := d.DecodeElement(sl.Hf, &t); err != nil {
					return err
				}
			case "extLst":
				sl.ExtLst = &ExtensionList{}
				if err := d.DecodeElement(sl.ExtLst, &t); err != nil {
					return err
				}
			default:
				if err := d.Skip(); err != nil {
					return err
				}
				continue
			}
			anchor = t.Name.Local
		case xml.EndElement:
			return nil
		}
	}
}

func (sl *SlideLayout) marshalRootChildren(b *xmlb.Builder) {
	emitAC := func(anchor string) {
		for i, ac := range sl.AlternateContent {
			if acAnchorAt(sl.acAnchors, i) == anchor {
				b.MarshalElement(xmlb.NSMarkupCompatibility, "AlternateContent", ac)
			}
		}
	}
	emitAC("")
	if sl.CSld != nil {
		b.MarshalElement(xmlb.NSPresentationML, "cSld", sl.CSld)
	}
	emitAC("cSld")
	if sl.ClrMapOvr != nil {
		b.MarshalElement(xmlb.NSPresentationML, "clrMapOvr", sl.ClrMapOvr)
	}
	emitAC("clrMapOvr")
	if sl.Transition != nil {
		b.MarshalElement(xmlb.NSPresentationML, "transition", sl.Transition)
	}
	emitAC("transition")
	if sl.Timing != nil {
		b.MarshalElement(xmlb.NSPresentationML, "timing", sl.Timing)
	}
	emitAC("timing")
	if sl.Hf != nil {
		b.MarshalElement(xmlb.NSPresentationML, "hf", sl.Hf)
	}
	emitAC("hf")
	if sl.ExtLst != nil {
		b.MarshalElement(xmlb.NSPresentationML, "extLst", sl.ExtLst)
	}
	emitAC("extLst")
}

func (sl *SlideLayout) rootAttrs() []xmlb.Attr {
	var attrs []xmlb.Attr
	if sl.ShowMasterSp != nil {
		attrs = append(attrs, xmlb.BoolAttr("showMasterSp", *sl.ShowMasterSp))
	}
	if sl.ShowMasterPhAnim != nil {
		attrs = append(attrs, xmlb.BoolAttr("showMasterPhAnim", *sl.ShowMasterPhAnim))
	}
	// PowerPoint's attribute order: matchingName, type, preserve, userDrawn.
	// An explicit matchingName="" in the source is re-emitted.
	if sl.MatchingName != "" || sl.MatchingNamePresent {
		attrs = append(attrs, xmlb.StrAttr("matchingName", sl.MatchingName))
	}
	if sl.Type != "" {
		attrs = append(attrs, xmlb.StrAttr("type", sl.Type))
	}
	if sl.Preserve {
		attrs = append(attrs, xmlb.BoolAttr("preserve", true))
	}
	if sl.UserDrawn {
		attrs = append(attrs, xmlb.BoolAttr("userDrawn", true))
	}
	return attrs
}

// MarshalToBuilder implements xmlb.BuilderMarshaler (non-root contexts).
func (sl *SlideLayout) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName, sl.rootAttrs()...)
	sl.marshalRootChildren(b)
	b.EndElement(ns, localName)
}

// MarshalRootToBuilder writes the p:sldLayout root element with the standard
// PresentationML namespace declarations.
func (sl *SlideLayout) MarshalRootToBuilder(b *xmlb.Builder) {
	if sl.OriginalRootAttrs != nil {
		b.StartElementWithRootAttrs(xmlb.NSPresentationML, "sldLayout", sl.OriginalRootAttrs)
	} else {
		b.StartElementWithNS(xmlb.NSPresentationML, "sldLayout", xmlb.PresentationMLNamespaces(), sl.rootAttrs()...)
	}
	sl.marshalRootChildren(b)
	b.EndElement(xmlb.NSPresentationML, "sldLayout")
}

// --- SlideMaster ---

// UnmarshalXML parses a p:sldMaster root (see Slide.UnmarshalXML).
func (sm *SlideMaster) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	sm.XMLName = start.Name
	sm.OriginalRootAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	for _, attr := range start.Attr {
		if attr.Name.Space == "" && attr.Name.Local == "preserve" {
			if v := parseXSDBool(attr.Value); v != nil {
				sm.Preserve = *v
			}
		}
	}
	anchor := ""
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Space == xmlb.NSMarkupCompatibility && t.Name.Local == "AlternateContent" {
				ac := &AlternateContent{}
				if err := d.DecodeElement(ac, &t); err != nil {
					return err
				}
				sm.AlternateContent = append(sm.AlternateContent, ac)
				sm.acAnchors = append(sm.acAnchors, anchor)
				continue
			}
			switch t.Name.Local {
			case "cSld":
				sm.CSld = &CommonSlideData{}
				if err := d.DecodeElement(sm.CSld, &t); err != nil {
					return err
				}
			case "clrMap":
				sm.ClrMap = &ColorMap{}
				if err := d.DecodeElement(sm.ClrMap, &t); err != nil {
					return err
				}
			case "sldLayoutIdLst":
				sm.SlideLayoutIDs = &SlideLayoutIDs{}
				if err := d.DecodeElement(sm.SlideLayoutIDs, &t); err != nil {
					return err
				}
			case "transition":
				sm.Transition = &Transition{}
				if err := d.DecodeElement(sm.Transition, &t); err != nil {
					return err
				}
			case "timing":
				sm.Timing = &Timing{}
				if err := d.DecodeElement(sm.Timing, &t); err != nil {
					return err
				}
			case "hf":
				sm.Hf = &HeaderFooter{}
				if err := d.DecodeElement(sm.Hf, &t); err != nil {
					return err
				}
			case "txStyles":
				sm.TxStyles = &TxStyles{}
				if err := d.DecodeElement(sm.TxStyles, &t); err != nil {
					return err
				}
			case "extLst":
				sm.ExtLst = &ExtensionList{}
				if err := d.DecodeElement(sm.ExtLst, &t); err != nil {
					return err
				}
			default:
				if err := d.Skip(); err != nil {
					return err
				}
				continue
			}
			anchor = t.Name.Local
		case xml.EndElement:
			return nil
		}
	}
}

func (sm *SlideMaster) marshalRootChildren(b *xmlb.Builder) {
	emitAC := func(anchor string) {
		for i, ac := range sm.AlternateContent {
			if acAnchorAt(sm.acAnchors, i) == anchor {
				b.MarshalElement(xmlb.NSMarkupCompatibility, "AlternateContent", ac)
			}
		}
	}
	emitAC("")
	if sm.CSld != nil {
		b.MarshalElement(xmlb.NSPresentationML, "cSld", sm.CSld)
	}
	emitAC("cSld")
	if sm.ClrMap != nil {
		b.MarshalElement(xmlb.NSPresentationML, "clrMap", sm.ClrMap)
	}
	emitAC("clrMap")
	if sm.SlideLayoutIDs != nil {
		b.MarshalElement(xmlb.NSPresentationML, "sldLayoutIdLst", sm.SlideLayoutIDs)
	}
	emitAC("sldLayoutIdLst")
	if sm.Transition != nil {
		b.MarshalElement(xmlb.NSPresentationML, "transition", sm.Transition)
	}
	emitAC("transition")
	if sm.Timing != nil {
		b.MarshalElement(xmlb.NSPresentationML, "timing", sm.Timing)
	}
	emitAC("timing")
	if sm.Hf != nil {
		b.MarshalElement(xmlb.NSPresentationML, "hf", sm.Hf)
	}
	emitAC("hf")
	if sm.TxStyles != nil {
		b.MarshalElement(xmlb.NSPresentationML, "txStyles", sm.TxStyles)
	}
	emitAC("txStyles")
	if sm.ExtLst != nil {
		b.MarshalElement(xmlb.NSPresentationML, "extLst", sm.ExtLst)
	}
	emitAC("extLst")
}

func (sm *SlideMaster) rootAttrs() []xmlb.Attr {
	var attrs []xmlb.Attr
	if sm.Preserve {
		attrs = append(attrs, xmlb.BoolAttr("preserve", true))
	}
	return attrs
}

// MarshalToBuilder implements xmlb.BuilderMarshaler (non-root contexts).
func (sm *SlideMaster) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName, sm.rootAttrs()...)
	sm.marshalRootChildren(b)
	b.EndElement(ns, localName)
}

// MarshalRootToBuilder writes the p:sldMaster root element with the standard
// PresentationML namespace declarations.
func (sm *SlideMaster) MarshalRootToBuilder(b *xmlb.Builder) {
	if sm.OriginalRootAttrs != nil {
		b.StartElementWithRootAttrs(xmlb.NSPresentationML, "sldMaster", sm.OriginalRootAttrs)
	} else {
		b.StartElementWithNS(xmlb.NSPresentationML, "sldMaster", xmlb.PresentationMLNamespaces(), sm.rootAttrs()...)
	}
	sm.marshalRootChildren(b)
	b.EndElement(xmlb.NSPresentationML, "sldMaster")
}
