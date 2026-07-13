// This file contains the XML schema types for XLSX documents.

package oxml

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"

	coxml "github.com/mgilbir/spine/common/oxml"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// SpreadsheetML namespace constants
const (
	nsSML = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
	nsR   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
)

// Workbook child element type constants for child ordering.
const (
	wbChildFileVersion      = "fileVersion"
	wbChildWorkbookPr       = "workbookPr"
	wbChildAlternateContent = "AlternateContent"
	wbChildBookViews        = "bookViews"
	wbChildSheets           = "sheets"
	wbChildDefinedNames     = "definedNames"
	wbChildCalcPr           = "calcPr"
	wbChildExtLst           = "extLst"
)

// WbUnknownChild represents an unknown child element captured as raw bytes.
type WbUnknownChild struct {
	Data []byte // raw XML bytes including the element itself
}

// CT_Workbook is the root element of workbook.xml.
type CT_Workbook struct {
	XMLName          xml.Name                `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main workbook"`
	Conformance      string                  `xml:"conformance,attr,omitempty"`
	Ignorable        string                  `xml:"-"` // mc:Ignorable attribute value
	FileVersion      *CT_FileVersion         `xml:"fileVersion,omitempty"`
	WorkbookPr       *CT_WorkbookPr          `xml:"workbookPr,omitempty"`
	AlternateContent *coxml.AlternateContent `xml:"-"` // mc:AlternateContent child element
	BookViews        *CT_BookViews           `xml:"bookViews,omitempty"`
	Sheets           CT_Sheets               `xml:"sheets"`
	DefinedNames     *CT_DefinedNames        `xml:"definedNames,omitempty"`
	CalcPr           *CT_CalcPr              `xml:"calcPr,omitempty"`
	ExtLst           *CT_ExtensionList       `xml:"extLst,omitempty"`
	// UnknownChildren stores extension child elements (like xr:revisionPtr)
	// that we don't have typed structs for, indexed for child ordering.
	UnknownChildren []WbUnknownChild `xml:"-"`
	// ChildOrder preserves the order of child elements for round-trip.
	// Each entry is either a known element name (e.g., "fileVersion") or
	// "unknown:N" where N is the index into UnknownChildren.
	ChildOrder []string `xml:"-"`
	// OriginalRootAttrs preserves all root element attributes (namespace declarations
	// and regular attributes like mc:Ignorable) in their original order for round-trip.
	OriginalRootAttrs []xmlb.RootAttr `xml:"-"`
	// OriginalNSDecls preserves namespace declarations (fallback for new workbooks).
	OriginalNSDecls []xmlb.NSDecl `xml:"-"`
	// Prolog preserves the source part's XML declaration and surrounding
	// whitespace for byte-faithful regeneration. Set from raw part data
	// during loading.
	Prolog xmlb.Prolog `xml:"-"`
	// SelfClosingSpace controls whether self-closing elements use " />" (true) or "/>" (false).
	SelfClosingSpace bool `xml:"-"`
	// RawSource holds the raw part bytes being unmarshaled. When set before
	// xml.Unmarshal, unknown children are captured as verbatim source slices
	// (via decoder offsets) instead of being re-encoded from tokens, which
	// preserves producer quirks tokens cannot express (" />" self-closing
	// space, expanded empty elements like <xr:revisionPtr ...></xr:revisionPtr>).
	RawSource []byte `xml:"-"`
	// ElemSeparator is whitespace inserted between sibling elements (e.g., " " for spaced format).
	ElemSeparator string `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_Workbook.
func (wb *CT_Workbook) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	wb.XMLName = start.Name

	// Capture all root element attributes in order for round-trip preservation.
	for _, attr := range start.Attr {
		if attr.Name.Local == "conformance" && attr.Name.Space == "" {
			wb.Conformance = attr.Value
		}
		if attr.Name.Local == "Ignorable" {
			wb.Ignorable = attr.Value
		}

		// Build ordered RootAttr list
		if attr.Name.Space == "xmlns" {
			// Prefixed namespace: xmlns:prefix="URI"
			wb.OriginalRootAttrs = append(wb.OriginalRootAttrs, xmlb.RootAttr{
				IsNS: true, Prefix: attr.Name.Local, Value: attr.Value,
			})
		} else if attr.Name.Space == "" && attr.Name.Local == "xmlns" {
			// Default namespace: xmlns="URI"
			wb.OriginalRootAttrs = append(wb.OriginalRootAttrs, xmlb.RootAttr{
				IsNS: true, Prefix: "", Value: attr.Value,
			})
		} else {
			// Regular attribute (conformance, mc:Ignorable, etc.)
			prefix := ""
			localName := attr.Name.Local
			// Go's xml decoder puts the full namespace URI in Space for namespaced attrs.
			// We need the prefix. Check for known namespace URIs.
			switch attr.Name.Space {
			case xmlb.NSMarkupCompatibility:
				prefix = xmlb.PrefixMarkupCompatibility
			case nsR:
				prefix = "r"
			case xmlb.NSXML:
				// Reserved prefix, never declared: xml:space etc.
				prefix = "xml"
			case "":
				// no prefix
			default:
				// For unknown namespaces, try to find the prefix from already-seen xmlns decls
				for _, ra := range wb.OriginalRootAttrs {
					if ra.IsNS && ra.Value == attr.Name.Space {
						prefix = ra.Prefix
						break
					}
				}
			}
			wb.OriginalRootAttrs = append(wb.OriginalRootAttrs, xmlb.RootAttr{
				IsNS: false, Prefix: prefix, LocalName: localName, Value: attr.Value,
			})
		}
	}

	// Build namespace prefix map from root attributes for resolving URIs in unknown children.
	nsPrefixMap := make(map[string]string)
	for _, ra := range wb.OriginalRootAttrs {
		if ra.IsNS && ra.Prefix != "" {
			nsPrefixMap[ra.Value] = ra.Prefix
		}
	}

	for {
		tokStart := d.InputOffset()
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.CharData:
			// Capture inter-element whitespace for round-trip formatting.
			if wb.ElemSeparator == "" && len(t) > 0 {
				isWS := true
				for _, b := range t {
					if b != ' ' && b != '\t' && b != '\n' && b != '\r' {
						isWS = false
						break
					}
				}
				if isWS {
					wb.ElemSeparator = string(t)
				}
			}
		case xml.StartElement:
			switch t.Name.Local {
			case "fileVersion":
				wb.FileVersion = &CT_FileVersion{}
				if err := d.DecodeElement(wb.FileVersion, &t); err != nil {
					return err
				}
				wb.ChildOrder = append(wb.ChildOrder, wbChildFileVersion)
			case "workbookPr":
				wb.WorkbookPr = &CT_WorkbookPr{}
				if err := d.DecodeElement(wb.WorkbookPr, &t); err != nil {
					return err
				}
				wb.ChildOrder = append(wb.ChildOrder, wbChildWorkbookPr)
			case "AlternateContent":
				wb.AlternateContent = &coxml.AlternateContent{}
				if err := d.DecodeElement(wb.AlternateContent, &t); err != nil {
					return err
				}
				wb.ChildOrder = append(wb.ChildOrder, wbChildAlternateContent)
			case "bookViews":
				wb.BookViews = &CT_BookViews{}
				if err := d.DecodeElement(wb.BookViews, &t); err != nil {
					return err
				}
				wb.ChildOrder = append(wb.ChildOrder, wbChildBookViews)
			case "sheets":
				if err := d.DecodeElement(&wb.Sheets, &t); err != nil {
					return err
				}
				wb.ChildOrder = append(wb.ChildOrder, wbChildSheets)
			case "definedNames":
				wb.DefinedNames = &CT_DefinedNames{}
				if err := d.DecodeElement(wb.DefinedNames, &t); err != nil {
					return err
				}
				wb.ChildOrder = append(wb.ChildOrder, wbChildDefinedNames)
			case "calcPr":
				wb.CalcPr = &CT_CalcPr{}
				if err := d.DecodeElement(wb.CalcPr, &t); err != nil {
					return err
				}
				wb.ChildOrder = append(wb.ChildOrder, wbChildCalcPr)
			case "extLst":
				wb.ExtLst = &CT_ExtensionList{}
				if err := d.DecodeElement(wb.ExtLst, &t); err != nil {
					return err
				}
				wb.ChildOrder = append(wb.ChildOrder, wbChildExtLst)
			default:
				// Capture unknown child elements as raw bytes
				var raw struct {
					Content []byte `xml:",innerxml"`
				}
				if err := d.DecodeElement(&raw, &t); err != nil {
					return err
				}
				// Prefer the verbatim source slice (start of the element's
				// start tag through the end of its end tag): tokens cannot
				// distinguish <x/> from <x></x> or reproduce " />" styles.
				var data []byte
				if end := d.InputOffset(); wb.RawSource != nil &&
					tokStart >= 0 && end > tokStart && end <= int64(len(wb.RawSource)) {
					data = append([]byte(nil), wb.RawSource[tokStart:end]...)
				}
				if len(data) == 0 || data[0] != '<' {
					data = encodeUnknownElement(t, raw.Content, nsPrefixMap)
				}
				idx := len(wb.UnknownChildren)
				wb.UnknownChildren = append(wb.UnknownChildren, WbUnknownChild{Data: data})
				wb.ChildOrder = append(wb.ChildOrder, fmt.Sprintf("unknown:%d", idx))
			}
		case xml.EndElement:
			return nil
		}
	}
}

// encodeUnknownElement reconstructs the raw XML for an unknown element from its
// start element and inner content. nsPrefixMap maps namespace URIs to their prefixes
// (from root element namespace declarations).
func encodeUnknownElement(start xml.StartElement, innerContent []byte, nsPrefixMap map[string]string) []byte {
	// Prefixes declared inline on the element itself (xmlns:foo="urn:foo") must
	// resolve too, or the element and its attributes would be re-emitted
	// unprefixed and silently move into the default namespace (C201). The map
	// is copied on write so the caller's root-attr map is not polluted for
	// sibling elements. Inner content is raw bytes, so nested declarations and
	// prefixes are preserved verbatim and need no handling here.
	prefixes := nsPrefixMap
	cloned := false
	for _, attr := range start.Attr {
		var prefix string
		switch {
		case attr.Name.Space == "xmlns":
			prefix = attr.Name.Local
		case attr.Name.Space == "" && attr.Name.Local == "xmlns":
			prefix = "" // default namespace declaration
		default:
			continue
		}
		if !cloned {
			m := make(map[string]string, len(nsPrefixMap)+1)
			for k, v := range nsPrefixMap {
				m[k] = v
			}
			prefixes = m
			cloned = true
		}
		prefixes[attr.Value] = prefix
	}

	var buf []byte
	buf = append(buf, '<')

	// Write qualified name using namespace prefix
	if start.Name.Space != "" {
		if prefix, ok := prefixes[start.Name.Space]; ok && prefix != "" {
			buf = append(buf, prefix...)
			buf = append(buf, ':')
		}
	}
	buf = append(buf, start.Name.Local...)

	// Write attributes
	for _, attr := range start.Attr {
		buf = append(buf, ' ')
		if attr.Name.Space == "xmlns" {
			buf = append(buf, "xmlns:"...)
		} else if attr.Name.Space == xmlb.NSXML {
			// Reserved prefix, never declared: xml:space etc.
			buf = append(buf, "xml:"...)
		} else if attr.Name.Space != "" {
			if prefix, ok := prefixes[attr.Name.Space]; ok && prefix != "" {
				buf = append(buf, prefix...)
				buf = append(buf, ':')
			}
		}
		buf = append(buf, attr.Name.Local...)
		buf = append(buf, `="`...)
		buf = append(buf, xmlb.EscapeAttrValue(attr.Value)...)
		buf = append(buf, '"')
	}

	if len(innerContent) == 0 {
		buf = append(buf, "/>"...)
	} else {
		buf = append(buf, '>')
		buf = append(buf, innerContent...)
		buf = append(buf, "</"...)
		if start.Name.Space != "" {
			if prefix, ok := prefixes[start.Name.Space]; ok && prefix != "" {
				buf = append(buf, prefix...)
				buf = append(buf, ':')
			}
		}
		buf = append(buf, start.Name.Local...)
		buf = append(buf, '>')
	}
	return buf
}

// workbookSchemaSeq is the CT_Workbook child element sequence from the
// SpreadsheetML schema (ECMA-376 sml.xsd). It includes elements this package
// has no typed model for (fileSharing, pivotCaches, ...) so that captured
// unknown children can still be ranked when computing a schema-ordered
// insertion point.
var workbookSchemaSeq = []string{
	"fileVersion", "fileSharing", "workbookPr", "workbookProtection",
	"bookViews", "sheets", "functionGroups", "externalReferences",
	"definedNames", "calcPr", "oleSize", "customWorkbookViews",
	"pivotCaches", "smartTagPr", "smartTagTypes", "webPublishing",
	"fileRecoveryPr", "webPublishObjects", "extLst",
}

// workbookChildRank maps a CT_Workbook child local name to its index in
// workbookSchemaSeq.
var workbookChildRank = func() map[string]int {
	m := make(map[string]int, len(workbookSchemaSeq))
	for i, n := range workbookSchemaSeq {
		m[n] = i
	}
	return m
}()

// EnsureChildOrder records name in wb.ChildOrder at its CT_Workbook schema
// position if it is not already present. Mutators must call this when they
// populate a child element kind the parsed workbook did not contain:
// marshaling of opened workbooks is ChildOrder-gated, so a populated child
// without an entry would be silently dropped (C12). It is a no-op when
// ChildOrder is empty (new workbooks marshal in fixed schema order) or when
// name is already listed.
//
// The insertion point is immediately before the first existing entry whose
// schema rank exceeds name's rank. "unknown:N" entries are ranked by the
// captured element's local name when it is a standard workbook child we lack
// a typed model for; otherwise (e.g. xr:revisionPtr, mc:AlternateContent)
// they impose no constraint and keep their position relative to the
// surrounding known children. This mirrors CT_Worksheet.EnsureChildOrder.
func (wb *CT_Workbook) EnsureChildOrder(name string) {
	if len(wb.ChildOrder) == 0 {
		return
	}
	rank, ok := workbookChildRank[name]
	if !ok {
		return
	}
	for _, entry := range wb.ChildOrder {
		if entry == name {
			return
		}
	}
	insert := len(wb.ChildOrder)
	for i, entry := range wb.ChildOrder {
		if r, ok := wb.childOrderEntryRank(entry); ok && r > rank {
			insert = i
			break
		}
	}
	wb.ChildOrder = append(wb.ChildOrder, "")
	copy(wb.ChildOrder[insert+1:], wb.ChildOrder[insert:])
	wb.ChildOrder[insert] = name
}

// childOrderEntryRank resolves the schema rank of a ChildOrder entry. Known
// element names are looked up directly; "unknown:N" entries are ranked by the
// local name of the captured element when it is a standard workbook child.
func (wb *CT_Workbook) childOrderEntryRank(entry string) (int, bool) {
	if idx, isUnknown := strings.CutPrefix(entry, "unknown:"); isUnknown {
		n, err := strconv.Atoi(idx)
		if err != nil || n < 0 || n >= len(wb.UnknownChildren) {
			return 0, false
		}
		r, ok := workbookChildRank[unknownElementLocalName(wb.UnknownChildren[n].Data)]
		return r, ok
	}
	r, ok := workbookChildRank[entry]
	return r, ok
}

// CT_FileVersion represents the fileVersion element.
type CT_FileVersion struct {
	AppName      string `xml:"appName,attr,omitempty"`
	LastEdited   string `xml:"lastEdited,attr,omitempty"`
	LowestEdited string `xml:"lowestEdited,attr,omitempty"`
	RupBuild     string `xml:"rupBuild,attr,omitempty"`
}

// CT_WorkbookPr represents the workbookPr element.
type CT_WorkbookPr struct {
	Date1904                   *BoolLex `xml:"date1904,attr,omitempty"`
	ShowObjects                string   `xml:"showObjects,attr,omitempty"`
	ShowBorderUnselectedTables *BoolLex `xml:"showBorderUnselectedTables,attr,omitempty"`
	FilterPrivacy              *BoolLex `xml:"filterPrivacy,attr,omitempty"`
	PromptedSolutions          *BoolLex `xml:"promptedSolutions,attr,omitempty"`
	ShowInkAnnotation          *BoolLex `xml:"showInkAnnotation,attr,omitempty"`
	BackupFile                 *BoolLex `xml:"backupFile,attr,omitempty"`
	SaveExternalLinkValues     *BoolLex `xml:"saveExternalLinkValues,attr,omitempty"`
	UpdateLinks                string   `xml:"updateLinks,attr,omitempty"`
	CodeName                   string   `xml:"codeName,attr,omitempty"`
	HidePivotFieldList         *BoolLex `xml:"hidePivotFieldList,attr,omitempty"`
	ShowPivotChartFilter       *BoolLex `xml:"showPivotChartFilter,attr,omitempty"`
	AllowRefreshQuery          *BoolLex `xml:"allowRefreshQuery,attr,omitempty"`
	CheckCompatibility         *BoolLex `xml:"checkCompatibility,attr,omitempty"`
	AutoCompressPictures       *BoolLex `xml:"autoCompressPictures,attr,omitempty"`
	DefaultThemeVersion        *uint32  `xml:"defaultThemeVersion,attr,omitempty"`
}

// CT_BookViews represents the bookViews element.
type CT_BookViews struct {
	WorkbookView []CT_BookView `xml:"workbookView"`
}

// CT_BookView represents a workbookView element. Has custom UnmarshalXML/MarshalToBuilder
// to handle extension attributes (e.g., xr2:uid) for round-trip preservation.
type CT_BookView struct {
	Visibility             string      `xml:"-"`
	Minimized              *BoolLex    `xml:"-"`
	ShowHorizontalScroll   *BoolLex    `xml:"-"`
	ShowVerticalScroll     *BoolLex    `xml:"-"`
	ShowSheetTabs          *BoolLex    `xml:"-"`
	XWindow                *int32      `xml:"-"`
	YWindow                *int32      `xml:"-"`
	WindowWidth            *uint32     `xml:"-"`
	WindowHeight           *uint32     `xml:"-"`
	TabRatio               *uint32     `xml:"-"`
	FirstSheet             *uint32     `xml:"-"`
	ActiveTab              *uint32     `xml:"-"`
	AutoFilterDateGrouping *BoolLex    `xml:"-"`
	ExtAttrs               []xmlb.Attr `xml:"-"` // extension attrs (e.g., xr2:uid)
	// attrOrder records the source order of the known attributes above:
	// Excel writes them in XSD order but Apache POI writes them
	// alphabetically, so a fixed emission order cannot serve both.
	attrOrder []string
}

// bookViewAttrOrder lists the modeled workbookView attribute names in
// canonical (XSD) emission order.
var bookViewAttrOrder = []string{
	"visibility", "minimized", "showHorizontalScroll", "showVerticalScroll",
	"showSheetTabs", "xWindow", "yWindow", "windowWidth", "windowHeight",
	"tabRatio", "firstSheet", "activeTab", "autoFilterDateGrouping",
}

// bookViewAttrNames is the set of modeled workbookView attribute names.
var bookViewAttrNames = func() map[string]bool {
	m := make(map[string]bool, len(bookViewAttrOrder))
	for _, n := range bookViewAttrOrder {
		m[n] = true
	}
	return m
}()

// UnmarshalXML implements custom unmarshaling for CT_BookView.
func (bv *CT_BookView) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Space == "" && bookViewAttrNames[attr.Name.Local] {
			bv.attrOrder = append(bv.attrOrder, attr.Name.Local)
		}
		switch attr.Name.Local {
		case "visibility":
			bv.Visibility = attr.Value
		case "minimized":
			v := &BoolLex{}
			if err := v.UnmarshalXMLAttr(attr); err != nil {
				return err
			}
			bv.Minimized = v
		case "showHorizontalScroll":
			v := &BoolLex{}
			if err := v.UnmarshalXMLAttr(attr); err != nil {
				return err
			}
			bv.ShowHorizontalScroll = v
		case "showVerticalScroll":
			v := &BoolLex{}
			if err := v.UnmarshalXMLAttr(attr); err != nil {
				return err
			}
			bv.ShowVerticalScroll = v
		case "showSheetTabs":
			v := &BoolLex{}
			if err := v.UnmarshalXMLAttr(attr); err != nil {
				return err
			}
			bv.ShowSheetTabs = v
		case "xWindow":
			if n, err := strconv.ParseInt(attr.Value, 10, 32); err == nil {
				v := int32(n)
				bv.XWindow = &v
			}
		case "yWindow":
			if n, err := strconv.ParseInt(attr.Value, 10, 32); err == nil {
				v := int32(n)
				bv.YWindow = &v
			}
		case "windowWidth":
			if n, err := strconv.ParseUint(attr.Value, 10, 32); err == nil {
				v := uint32(n)
				bv.WindowWidth = &v
			}
		case "windowHeight":
			if n, err := strconv.ParseUint(attr.Value, 10, 32); err == nil {
				v := uint32(n)
				bv.WindowHeight = &v
			}
		case "tabRatio":
			if n, err := strconv.ParseUint(attr.Value, 10, 32); err == nil {
				v := uint32(n)
				bv.TabRatio = &v
			}
		case "firstSheet":
			if n, err := strconv.ParseUint(attr.Value, 10, 32); err == nil {
				v := uint32(n)
				bv.FirstSheet = &v
			}
		case "activeTab":
			if n, err := strconv.ParseUint(attr.Value, 10, 32); err == nil {
				v := uint32(n)
				bv.ActiveTab = &v
			}
		case "autoFilterDateGrouping":
			v := &BoolLex{}
			if err := v.UnmarshalXMLAttr(attr); err != nil {
				return err
			}
			bv.AutoFilterDateGrouping = v
		default:
			if attr.Name.Space == "xmlns" || (attr.Name.Space == "" && attr.Name.Local == "xmlns") {
				continue // skip namespace declarations
			}
			// Extension attribute (e.g., xr2:uid) - store with namespace URI
			bv.ExtAttrs = append(bv.ExtAttrs, xmlb.Attr{
				Namespace: attr.Name.Space,
				Name:      attr.Name.Local,
				Value:     attr.Value,
			})
		}
	}
	return d.Skip()
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_BookView.
// Attributes are written in the captured source order (Excel uses XSD order,
// POI alphabetical); values built programmatically use the fixed XSD order.
func (bv *CT_BookView) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	if len(bv.attrOrder) > 0 {
		attrs := make([]xmlb.Attr, 0, len(bv.attrOrder)+len(bv.ExtAttrs))
		seen := make(map[string]bool, len(bv.attrOrder))
		for _, name := range bv.attrOrder {
			seen[name] = true
			if a, ok := bv.namedAttr(name); ok {
				attrs = append(attrs, a)
			}
		}
		// Fields set after parse (mutation API) follow in canonical order.
		for _, name := range bookViewAttrOrder {
			if seen[name] {
				continue
			}
			if a, ok := bv.namedAttr(name); ok {
				attrs = append(attrs, a)
			}
		}
		attrs = append(attrs, bv.ExtAttrs...)
		b.EmptyElement(ns, localName, attrs...)
		return
	}
	var attrs []xmlb.Attr
	if bv.Visibility != "" {
		attrs = append(attrs, xmlb.StrAttr("visibility", bv.Visibility))
	}
	if bv.Minimized != nil {
		attrs = append(attrs, xmlb.StrAttr("minimized", bv.Minimized.AttrValue()))
	}
	if bv.ShowHorizontalScroll != nil {
		attrs = append(attrs, xmlb.StrAttr("showHorizontalScroll", bv.ShowHorizontalScroll.AttrValue()))
	}
	if bv.ShowVerticalScroll != nil {
		attrs = append(attrs, xmlb.StrAttr("showVerticalScroll", bv.ShowVerticalScroll.AttrValue()))
	}
	if bv.ShowSheetTabs != nil {
		attrs = append(attrs, xmlb.StrAttr("showSheetTabs", bv.ShowSheetTabs.AttrValue()))
	}
	if bv.XWindow != nil {
		attrs = append(attrs, xmlb.Int32Attr("xWindow", *bv.XWindow))
	}
	if bv.YWindow != nil {
		attrs = append(attrs, xmlb.Int32Attr("yWindow", *bv.YWindow))
	}
	if bv.WindowWidth != nil {
		attrs = append(attrs, xmlb.UintAttr("windowWidth", *bv.WindowWidth))
	}
	if bv.WindowHeight != nil {
		attrs = append(attrs, xmlb.UintAttr("windowHeight", *bv.WindowHeight))
	}
	if bv.TabRatio != nil {
		attrs = append(attrs, xmlb.UintAttr("tabRatio", *bv.TabRatio))
	}
	if bv.FirstSheet != nil {
		attrs = append(attrs, xmlb.UintAttr("firstSheet", *bv.FirstSheet))
	}
	if bv.ActiveTab != nil {
		attrs = append(attrs, xmlb.UintAttr("activeTab", *bv.ActiveTab))
	}
	if bv.AutoFilterDateGrouping != nil {
		attrs = append(attrs, xmlb.StrAttr("autoFilterDateGrouping", bv.AutoFilterDateGrouping.AttrValue()))
	}
	// Append extension attributes (e.g., xr2:uid)
	attrs = append(attrs, bv.ExtAttrs...)
	b.EmptyElement(ns, localName, attrs...)
}

// namedAttr returns the Attr for one modeled workbookView attribute, or
// ok=false when the field is unset.
func (bv *CT_BookView) namedAttr(name string) (xmlb.Attr, bool) {
	switch name {
	case "visibility":
		if bv.Visibility != "" {
			return xmlb.StrAttr("visibility", bv.Visibility), true
		}
	case "minimized":
		if bv.Minimized != nil {
			return xmlb.StrAttr("minimized", bv.Minimized.AttrValue()), true
		}
	case "showHorizontalScroll":
		if bv.ShowHorizontalScroll != nil {
			return xmlb.StrAttr("showHorizontalScroll", bv.ShowHorizontalScroll.AttrValue()), true
		}
	case "showVerticalScroll":
		if bv.ShowVerticalScroll != nil {
			return xmlb.StrAttr("showVerticalScroll", bv.ShowVerticalScroll.AttrValue()), true
		}
	case "showSheetTabs":
		if bv.ShowSheetTabs != nil {
			return xmlb.StrAttr("showSheetTabs", bv.ShowSheetTabs.AttrValue()), true
		}
	case "xWindow":
		if bv.XWindow != nil {
			return xmlb.Int32Attr("xWindow", *bv.XWindow), true
		}
	case "yWindow":
		if bv.YWindow != nil {
			return xmlb.Int32Attr("yWindow", *bv.YWindow), true
		}
	case "windowWidth":
		if bv.WindowWidth != nil {
			return xmlb.UintAttr("windowWidth", *bv.WindowWidth), true
		}
	case "windowHeight":
		if bv.WindowHeight != nil {
			return xmlb.UintAttr("windowHeight", *bv.WindowHeight), true
		}
	case "tabRatio":
		if bv.TabRatio != nil {
			return xmlb.UintAttr("tabRatio", *bv.TabRatio), true
		}
	case "firstSheet":
		if bv.FirstSheet != nil {
			return xmlb.UintAttr("firstSheet", *bv.FirstSheet), true
		}
	case "activeTab":
		if bv.ActiveTab != nil {
			return xmlb.UintAttr("activeTab", *bv.ActiveTab), true
		}
	case "autoFilterDateGrouping":
		if bv.AutoFilterDateGrouping != nil {
			return xmlb.StrAttr("autoFilterDateGrouping", bv.AutoFilterDateGrouping.AttrValue()), true
		}
	}
	return xmlb.Attr{}, false
}

// CT_Sheets represents the sheets element.
type CT_Sheets struct {
	Sheet []CT_Sheet `xml:"sheet"`
}

// CT_Sheet represents a sheet element. It has an r:id attribute that requires
// custom UnmarshalXML and MarshalToBuilder.
type CT_Sheet struct {
	Name    string `xml:"-"`
	SheetId uint32 `xml:"-"`
	State   string `xml:"-"`
	RID     string `xml:"-"`
	// InlineNSDecls preserves xmlns declarations carried on the sheet
	// element itself. Some producers (System.IO.Packaging) declare the
	// relationships namespace inline (<sheet ... r:id="..." xmlns:r="..."/>)
	// instead of on the workbook root; dropping the declaration leaves r:id
	// referencing an undeclared prefix.
	InlineNSDecls []xmlb.NSDecl `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_Sheet.
// Handles the r:id attribute which uses the relationships namespace.
func (s *CT_Sheet) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Space == "xmlns":
			s.InlineNSDecls = append(s.InlineNSDecls, xmlb.NSDecl{Prefix: attr.Name.Local, URI: attr.Value})
		case attr.Name.Space == "" && attr.Name.Local == "xmlns":
			s.InlineNSDecls = append(s.InlineNSDecls, xmlb.NSDecl{Prefix: "", URI: attr.Value})
		case attr.Name.Local == "name":
			s.Name = attr.Value
		case attr.Name.Local == "sheetId":
			if n, err := strconv.ParseUint(attr.Value, 10, 32); err == nil {
				s.SheetId = uint32(n)
			}
		case attr.Name.Local == "state":
			s.State = attr.Value
		case attr.Name.Local == "id" && attr.Name.Space == nsR:
			s.RID = attr.Value
		case attr.Name.Local == "r:id":
			s.RID = attr.Value
		}
	}
	return d.Skip()
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Sheet.
func (s *CT_Sheet) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	attrs := []xmlb.Attr{
		xmlb.StrAttr("name", s.Name),
		xmlb.UintAttr("sheetId", s.SheetId),
	}
	if s.State != "" {
		attrs = append(attrs, xmlb.StrAttr("state", s.State))
	}
	attrs = append(attrs, xmlb.RelAttr("id", s.RID))
	// Replay inline xmlns declarations after the regular attributes,
	// matching the position producers that use them write them in.
	for _, decl := range s.InlineNSDecls {
		name := "xmlns"
		if decl.Prefix != "" {
			name = "xmlns:" + decl.Prefix
		}
		attrs = append(attrs, xmlb.Attr{Name: name, Value: decl.URI})
	}
	b.EmptyElement(ns, localName, attrs...)
}

// CT_DefinedNames represents the definedNames element.
type CT_DefinedNames struct {
	DefinedName []CT_DefinedName `xml:"definedName"`
}

// CT_DefinedName represents a definedName element. It has chardata value plus
// attributes, requiring custom UnmarshalXML and MarshalToBuilder.
type CT_DefinedName struct {
	Name              string  `xml:"-"`
	Comment           string  `xml:"-"`
	CustomMenu        string  `xml:"-"`
	Description       string  `xml:"-"`
	Help              string  `xml:"-"`
	StatusBar         string  `xml:"-"`
	LocalSheetId      *uint32 `xml:"-"`
	Hidden            *bool   `xml:"-"`
	Function          *bool   `xml:"-"`
	VbProcedure       *bool   `xml:"-"`
	Xlm               *bool   `xml:"-"`
	FunctionGroupId   *uint32 `xml:"-"`
	ShortcutKey       string  `xml:"-"`
	PublishToServer   *bool   `xml:"-"`
	WorkbookParameter *bool   `xml:"-"`
	Value             string  `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_DefinedName.
func (dn *CT_DefinedName) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "name":
			dn.Name = attr.Value
		case "comment":
			dn.Comment = attr.Value
		case "customMenu":
			dn.CustomMenu = attr.Value
		case "description":
			dn.Description = attr.Value
		case "help":
			dn.Help = attr.Value
		case "statusBar":
			dn.StatusBar = attr.Value
		case "localSheetId":
			if n, err := strconv.ParseUint(attr.Value, 10, 32); err == nil {
				v := uint32(n)
				dn.LocalSheetId = &v
			}
		case "hidden":
			b := attr.Value == "1" || attr.Value == "true"
			dn.Hidden = &b
		case "function":
			b := attr.Value == "1" || attr.Value == "true"
			dn.Function = &b
		case "vbProcedure":
			b := attr.Value == "1" || attr.Value == "true"
			dn.VbProcedure = &b
		case "xlm":
			b := attr.Value == "1" || attr.Value == "true"
			dn.Xlm = &b
		case "functionGroupId":
			if n, err := strconv.ParseUint(attr.Value, 10, 32); err == nil {
				v := uint32(n)
				dn.FunctionGroupId = &v
			}
		case "shortcutKey":
			dn.ShortcutKey = attr.Value
		case "publishToServer":
			b := attr.Value == "1" || attr.Value == "true"
			dn.PublishToServer = &b
		case "workbookParameter":
			b := attr.Value == "1" || attr.Value == "true"
			dn.WorkbookParameter = &b
		}
	}

	// Read chardata content
	var content struct {
		Value string `xml:",chardata"`
	}
	if err := d.DecodeElement(&content, &start); err != nil {
		return err
	}
	dn.Value = content.Value
	return nil
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_DefinedName.
func (dn *CT_DefinedName) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	attrs := []xmlb.Attr{
		xmlb.StrAttr("name", dn.Name),
	}
	if dn.Comment != "" {
		attrs = append(attrs, xmlb.StrAttr("comment", dn.Comment))
	}
	if dn.CustomMenu != "" {
		attrs = append(attrs, xmlb.StrAttr("customMenu", dn.CustomMenu))
	}
	if dn.Description != "" {
		attrs = append(attrs, xmlb.StrAttr("description", dn.Description))
	}
	if dn.Help != "" {
		attrs = append(attrs, xmlb.StrAttr("help", dn.Help))
	}
	if dn.StatusBar != "" {
		attrs = append(attrs, xmlb.StrAttr("statusBar", dn.StatusBar))
	}
	if dn.LocalSheetId != nil {
		attrs = append(attrs, xmlb.UintAttr("localSheetId", *dn.LocalSheetId))
	}
	if dn.Hidden != nil {
		attrs = append(attrs, xmlb.BoolAttr("hidden", *dn.Hidden))
	}
	if dn.Function != nil {
		attrs = append(attrs, xmlb.BoolAttr("function", *dn.Function))
	}
	if dn.VbProcedure != nil {
		attrs = append(attrs, xmlb.BoolAttr("vbProcedure", *dn.VbProcedure))
	}
	if dn.Xlm != nil {
		attrs = append(attrs, xmlb.BoolAttr("xlm", *dn.Xlm))
	}
	if dn.FunctionGroupId != nil {
		attrs = append(attrs, xmlb.UintAttr("functionGroupId", *dn.FunctionGroupId))
	}
	if dn.ShortcutKey != "" {
		attrs = append(attrs, xmlb.StrAttr("shortcutKey", dn.ShortcutKey))
	}
	if dn.PublishToServer != nil {
		attrs = append(attrs, xmlb.BoolAttr("publishToServer", *dn.PublishToServer))
	}
	if dn.WorkbookParameter != nil {
		attrs = append(attrs, xmlb.BoolAttr("workbookParameter", *dn.WorkbookParameter))
	}
	b.WriteElement(ns, localName, dn.Value, attrs...)
}

// CT_CalcPr represents the calcPr element.
type CT_CalcPr struct {
	CalcId                *uint32   `xml:"calcId,attr,omitempty"`
	CalcMode              string    `xml:"calcMode,attr,omitempty"`
	CalcCompleted         *BoolLex  `xml:"calcCompleted,attr,omitempty"`
	FullCalcOnLoad        *BoolLex  `xml:"fullCalcOnLoad,attr,omitempty"`
	RefMode               string    `xml:"refMode,attr,omitempty"`
	Iterate               *BoolLex  `xml:"iterate,attr,omitempty"`
	IterateCount          *uint32   `xml:"iterateCount,attr,omitempty"`
	IterateDelta          *FloatLex `xml:"iterateDelta,attr,omitempty"`
	FullPrecision         *BoolLex  `xml:"fullPrecision,attr,omitempty"`
	CalcOnSave            *BoolLex  `xml:"calcOnSave,attr,omitempty"`
	ConcurrentCalc        *BoolLex  `xml:"concurrentCalc,attr,omitempty"`
	ConcurrentManualCount *uint32   `xml:"concurrentManualCount,attr,omitempty"`
	ForceFullCalc         *BoolLex  `xml:"forceFullCalc,attr,omitempty"`
}

// CT_ExtensionList represents the extLst element.
type CT_ExtensionList struct {
	Ext []CT_Extension `xml:"ext"`
}

// CT_Extension represents a single ext element with URI-based dispatch.
// Unknown extensions use RawContent for round-trip preservation.
type CT_Extension struct {
	URI           string        `xml:"uri,attr"`
	RawContent    []byte        `xml:"-"`
	InlineNSDecls []xmlb.NSDecl `xml:"-"` // xmlns declarations on the ext element (e.g., xmlns:x15="...")
	// NSDeclsFirst records that the source wrote the xmlns declarations
	// before the uri attribute (<ext xmlns:x15="..." uri="...">); Excel
	// itself writes uri first, but both orders occur in the wild.
	NSDeclsFirst bool `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_Extension.
func (e *CT_Extension) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	uriSeen := false
	for _, attr := range start.Attr {
		if attr.Name.Local == "uri" && attr.Name.Space == "" {
			e.URI = attr.Value
			uriSeen = true
		} else if attr.Name.Space == "xmlns" {
			// Capture inline namespace declarations (e.g., xmlns:x15="...")
			if !uriSeen && len(e.InlineNSDecls) == 0 {
				e.NSDeclsFirst = true
			}
			e.InlineNSDecls = append(e.InlineNSDecls, xmlb.NSDecl{
				Prefix: attr.Name.Local,
				URI:    attr.Value,
			})
		}
	}

	// Preserve raw content for all extensions
	var inner struct {
		Content []byte `xml:",innerxml"`
	}
	if err := d.DecodeElement(&inner, &start); err != nil {
		return err
	}
	e.RawContent = inner.Content
	return nil
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Extension.
func (e *CT_Extension) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	attrs := make([]xmlb.Attr, 0, len(e.InlineNSDecls)+1)
	if !e.NSDeclsFirst {
		attrs = append(attrs, xmlb.StrAttr("uri", e.URI))
	}
	for _, nsd := range e.InlineNSDecls {
		attrs = append(attrs, xmlb.Attr{Name: "xmlns:" + nsd.Prefix, Value: nsd.URI})
	}
	if e.NSDeclsFirst {
		attrs = append(attrs, xmlb.StrAttr("uri", e.URI))
	}
	b.StartElement(ns, localName, attrs...)
	if len(e.RawContent) > 0 {
		b.WriteRaw(e.RawContent)
	}
	b.EndElement(ns, localName)
}
