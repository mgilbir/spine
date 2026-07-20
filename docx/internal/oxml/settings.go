package oxml

import (
	"bytes"
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_Settings is the root element of the settings part (w:settings).
//
// It follows the house round-trip pattern: the root attributes and every
// child element of a parsed part are preserved verbatim and in order, so a
// zero-modification save stays byte-identical, while flags the API needs to
// toggle (w:evenAndOddHeaders) can be inserted at their schema position.
type CT_Settings struct {
	XMLName xml.Name `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main settings"`
	// OriginalNSDecls and Ignorable preserve the root attributes of a parsed
	// part so regeneration keeps its namespace declarations.
	OriginalNSDecls []xmlb.NSDecl `xml:"-"`
	Ignorable       string        `xml:"-"`
	// Children holds every child element verbatim, in document order.
	Children []*CT_RawNamedElement `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_Settings.
func (s *CT_Settings) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	s.XMLName = start.Name
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Space == "xmlns":
			s.OriginalNSDecls = append(s.OriginalNSDecls, xmlb.NSDecl{Prefix: attr.Name.Local, URI: attr.Value})
		case attr.Name.Space == "" && attr.Name.Local == "xmlns":
			s.OriginalNSDecls = append(s.OriginalNSDecls, xmlb.NSDecl{Prefix: "", URI: attr.Value})
		case attr.Name.Local == "Ignorable":
			s.Ignorable = attr.Value
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			v := &CT_RawNamedElement{}
			if err := d.DecodeElement(v, &t); err != nil {
				return err
			}
			s.Children = append(s.Children, v)
		case xml.EndElement:
			return nil
		}
	}
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Settings.
func (s *CT_Settings) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName)
	s.MarshalContent(b, ns)
	b.EndElement(ns, localName)
}

// MarshalContent writes the settings children verbatim, in order.
func (s *CT_Settings) MarshalContent(b *xmlb.Builder, ns string) {
	for _, c := range s.Children {
		c.MarshalNamed(b, ns)
	}
}

// settingsElementOrder maps each CT_Settings child element to its position in
// the schema sequence (ECMA-376 §17.15.1.78), used to insert new flags at a
// schema-valid position among the preserved children.
var settingsElementOrder = func() map[string]int {
	names := []string{
		"writeProtection", "view", "zoom", "removePersonalInformation",
		"removeDateAndTime", "doNotDisplayPageBoundaries",
		"displayBackgroundShape", "printPostScriptOverText",
		"printFractionalCharacterWidth", "printFormsData",
		"embedTrueTypeFonts", "embedSystemFonts", "saveSubsetFonts",
		"saveFormsData", "mirrorMargins", "alignBordersAndEdges",
		"bordersDoNotSurroundHeader", "bordersDoNotSurroundFooter",
		"gutterAtTop", "hideSpellingErrors", "hideGrammaticalErrors",
		"activeWritingStyle", "proofState", "formsDesign", "attachedTemplate",
		"linkStyles", "stylePaneFormatFilter", "stylePaneSortMethod",
		"documentType", "mailMerge", "revisionView", "trackRevisions",
		"doNotTrackMoves", "doNotTrackFormatting", "documentProtection",
		"autoFormatOverride", "styleLockTheme", "styleLockQFSet",
		"defaultTabStop", "autoHyphenation", "consecutiveHyphenLimit",
		"hyphenationZone", "doNotHyphenateCaps", "showEnvelope",
		"summaryLength", "clickAndTypeStyle", "defaultTableStyle",
		"evenAndOddHeaders", "bookFoldRevPrinting", "bookFoldPrinting",
		"bookFoldPrintingSheets", "drawingGridHorizontalSpacing",
		"drawingGridVerticalSpacing", "displayHorizontalDrawingGridEvery",
		"displayVerticalDrawingGridEvery",
		"doNotUseMarginsForDrawingGridOrigin", "drawingGridHorizontalOrigin",
		"drawingGridVerticalOrigin", "doNotShadeFormData",
		"noPunctuationKerning", "characterSpacingControl", "printTwoOnOne",
		"strictFirstAndLastChars", "noLineBreaksAfter", "noLineBreaksBefore",
		"savePreviewPicture", "doNotValidateAgainstSchema", "saveInvalidXml",
		"ignoreMixedContent", "alwaysShowPlaceholderText",
		"doNotDemarcateInvalidXml", "saveXmlDataOnly", "useXSLTWhenSaving",
		"saveThroughXslt", "showXMLTags", "alwaysMergeEmptyNamespace",
		"updateFields", "footnotePr", "endnotePr", "compat", "docVars",
		"rsids", "mathPr", "attachedSchema", "themeFontLang",
		"clrSchemeMapping", "doNotIncludeSubdocsInStats",
		"doNotAutoCompressPictures", "forceUpgrade", "captions",
		"readModeInkLockDown", "smartTagType", "schemaLibrary",
		"doNotEmbedSmartTags", "decimalSymbol", "listSeparator",
	}
	m := make(map[string]int, len(names))
	for i, n := range names {
		m[n] = i
	}
	return m
}()

// EnsureEvenAndOddHeaders inserts <w:evenAndOddHeaders/> at its schema
// position unless the element is already present. It reports whether the
// settings were changed. Children whose schema position is unknown (extension
// elements) never act as an insertion anchor, so a Word-ordered part keeps
// its layout with the flag inserted before the first later-in-schema child.
func (s *CT_Settings) EnsureEvenAndOddHeaders() bool {
	for _, c := range s.Children {
		if c.Local == "evenAndOddHeaders" {
			return false
		}
	}
	target := settingsElementOrder["evenAndOddHeaders"]
	insertAt := len(s.Children)
	for i, c := range s.Children {
		if pos, ok := settingsElementOrder[c.Local]; ok && pos > target {
			insertAt = i
			break
		}
	}
	el := &CT_RawNamedElement{Local: "evenAndOddHeaders"}
	s.Children = append(s.Children, nil)
	copy(s.Children[insertAt+1:], s.Children[insertAt:])
	s.Children[insertAt] = el
	return true
}

// Child returns the first settings child with the given local name, or nil.
func (s *CT_Settings) Child(local string) *CT_RawNamedElement {
	for _, c := range s.Children {
		if c.Local == local {
			return c
		}
	}
	return nil
}

// SetChild inserts, or replaces in place, a settings child with the given local
// name (in the WordprocessingML namespace) carrying attrs. A new element is
// placed at its schema-valid position among the preserved children, using the
// same insertion rule as EnsureEvenAndOddHeaders. The element is written empty
// (self-closing), which suits the flag-style settings this package authors.
func (s *CT_Settings) SetChild(local string, attrs []xml.Attr) {
	el := &CT_RawNamedElement{Local: local, Space: NsWml}
	el.Attrs = attrs
	s.setChildElement(el)
}

// SetRawChild inserts, or replaces in place, a settings child with the given
// local name (in the WordprocessingML namespace) carrying content as its raw
// inner XML. A new element is placed at its schema-valid position among the
// preserved children (the same insertion rule as SetChild). Empty content is
// written as a self-closing element.
func (s *CT_Settings) SetRawChild(local string, content []byte) {
	el := &CT_RawNamedElement{Local: local, Space: NsWml}
	el.RawContent = content
	s.setChildElement(el)
}

// setChildElement replaces the first settings child sharing el's local name in
// place, or inserts el at its schema-valid position among the preserved
// children (the same insertion rule as EnsureEvenAndOddHeaders).
func (s *CT_Settings) setChildElement(el *CT_RawNamedElement) {
	for i, c := range s.Children {
		if c.Local == el.Local {
			s.Children[i] = el
			return
		}
	}
	target, known := settingsElementOrder[el.Local]
	insertAt := len(s.Children)
	if known {
		for i, c := range s.Children {
			if pos, ok := settingsElementOrder[c.Local]; ok && pos > target {
				insertAt = i
				break
			}
		}
	}
	s.Children = append(s.Children, nil)
	copy(s.Children[insertAt+1:], s.Children[insertAt:])
	s.Children[insertAt] = el
}

// DocVars parses the document variables (w:docVars/w:docVar) from the preserved
// settings children, in document order. It returns nil when no w:docVars child
// is present. The children are stored verbatim; parsing here matches on local
// names so an unbound prefix in the raw fragment is tolerated.
func (s *CT_Settings) DocVars() []CT_DocVar {
	c := s.Child("docVars")
	if c == nil {
		return nil
	}
	return parseDocVars(c.RawContent)
}

// parseDocVars extracts the w:docVar name/value pairs from a raw w:docVars inner
// fragment.
func parseDocVars(raw []byte) []CT_DocVar {
	if len(raw) == 0 {
		return nil
	}
	dec := xml.NewDecoder(bytes.NewReader(raw))
	var vars []CT_DocVar
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "docVar" {
			continue
		}
		var dv CT_DocVar
		for _, a := range se.Attr {
			switch a.Name.Local {
			case "name":
				dv.Name = a.Value
			case "val":
				dv.Val = a.Value
			}
		}
		vars = append(vars, dv)
	}
	return vars
}

// SetDocVars replaces the document variables (w:docVars). An empty slice removes
// the w:docVars child. The element is regenerated from the given pairs, so after
// an edit it is schema-valid rather than byte-identical to a parsed original.
func (s *CT_Settings) SetDocVars(vars []CT_DocVar) {
	if len(vars) == 0 {
		s.RemoveChild("docVars")
		return
	}
	var buf bytes.Buffer
	for _, v := range vars {
		buf.WriteString(`<w:docVar w:name="`)
		buf.WriteString(xmlb.EscapeAttrValue(v.Name))
		buf.WriteString(`" w:val="`)
		buf.WriteString(xmlb.EscapeAttrValue(v.Val))
		buf.WriteString(`"/>`)
	}
	el := &CT_RawNamedElement{Local: "docVars", Space: NsWml}
	el.RawContent = buf.Bytes()
	s.setChildElement(el)
}

// RemoveChild deletes every settings child with the given local name and
// reports whether any were removed.
func (s *CT_Settings) RemoveChild(local string) bool {
	kept := s.Children[:0]
	removed := false
	for _, c := range s.Children {
		if c.Local == local {
			removed = true
			continue
		}
		kept = append(kept, c)
	}
	s.Children = kept
	return removed
}

// Attr returns the value of the named attribute (matched by local name), or the
// empty string when absent.
func (rn *CT_RawNamedElement) Attr(local string) string {
	for _, a := range rn.Attrs {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}
