// Package oxml contains the XML schema types for XLSX documents.
package oxml

import (
	"encoding/xml"
	"fmt"
	"strconv"

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
	XMLName          xml.Name                   `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main workbook"`
	Conformance      string                     `xml:"conformance,attr,omitempty"`
	Ignorable        string                     `xml:"-"` // mc:Ignorable attribute value
	FileVersion      *CT_FileVersion            `xml:"fileVersion,omitempty"`
	WorkbookPr       *CT_WorkbookPr             `xml:"workbookPr,omitempty"`
	AlternateContent *coxml.AlternateContent    `xml:"-"` // mc:AlternateContent child element
	BookViews        *CT_BookViews              `xml:"bookViews,omitempty"`
	Sheets           CT_Sheets                  `xml:"sheets"`
	DefinedNames     *CT_DefinedNames           `xml:"definedNames,omitempty"`
	CalcPr           *CT_CalcPr                 `xml:"calcPr,omitempty"`
	ExtLst           *CT_ExtensionList          `xml:"extLst,omitempty"`
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
	// OriginalXMLSep is the bytes between the XML declaration and the root element
	// (e.g., "\r\n" or "\n "). Set from raw part data during loading.
	OriginalXMLSep string `xml:"-"`
	// SelfClosingSpace controls whether self-closing elements use " />" (true) or "/>" (false).
	SelfClosingSpace bool `xml:"-"`
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
				idx := len(wb.UnknownChildren)
				wb.UnknownChildren = append(wb.UnknownChildren, WbUnknownChild{
					Data: encodeUnknownElement(t, raw.Content, nsPrefixMap),
				})
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

// CT_FileVersion represents the fileVersion element.
type CT_FileVersion struct {
	AppName      string `xml:"appName,attr,omitempty"`
	LastEdited   string `xml:"lastEdited,attr,omitempty"`
	LowestEdited string `xml:"lowestEdited,attr,omitempty"`
	RupBuild     string `xml:"rupBuild,attr,omitempty"`
}

// CT_WorkbookPr represents the workbookPr element.
type CT_WorkbookPr struct {
	Date1904                    *bool   `xml:"date1904,attr,omitempty"`
	ShowObjects                 string  `xml:"showObjects,attr,omitempty"`
	ShowBorderUnselectedTables  *bool   `xml:"showBorderUnselectedTables,attr,omitempty"`
	FilterPrivacy               *bool   `xml:"filterPrivacy,attr,omitempty"`
	PromptedSolutions           *bool   `xml:"promptedSolutions,attr,omitempty"`
	ShowInkAnnotation           *bool   `xml:"showInkAnnotation,attr,omitempty"`
	BackupFile                  *bool   `xml:"backupFile,attr,omitempty"`
	SaveExternalLinkValues      *bool   `xml:"saveExternalLinkValues,attr,omitempty"`
	UpdateLinks                 string  `xml:"updateLinks,attr,omitempty"`
	HidePivotFieldList          *bool   `xml:"hidePivotFieldList,attr,omitempty"`
	ShowPivotChartFilter        *bool   `xml:"showPivotChartFilter,attr,omitempty"`
	AllowRefreshQuery           *bool   `xml:"allowRefreshQuery,attr,omitempty"`
	AutoCompressPictures        *bool   `xml:"autoCompressPictures,attr,omitempty"`
	DefaultThemeVersion         *uint32 `xml:"defaultThemeVersion,attr,omitempty"`
	CodeName                    string  `xml:"codeName,attr,omitempty"`
	CheckCompatibility          *bool   `xml:"checkCompatibility,attr,omitempty"`
}

// CT_BookViews represents the bookViews element.
type CT_BookViews struct {
	WorkbookView []CT_BookView `xml:"workbookView"`
}

// CT_BookView represents a workbookView element. Has custom UnmarshalXML/MarshalToBuilder
// to handle extension attributes (e.g., xr2:uid) for round-trip preservation.
type CT_BookView struct {
	Visibility             string      `xml:"-"`
	Minimized              *bool       `xml:"-"`
	ShowHorizontalScroll   *bool       `xml:"-"`
	ShowVerticalScroll     *bool       `xml:"-"`
	ShowSheetTabs          *bool       `xml:"-"`
	XWindow                *int32      `xml:"-"`
	YWindow                *int32      `xml:"-"`
	WindowWidth            *uint32     `xml:"-"`
	WindowHeight           *uint32     `xml:"-"`
	TabRatio               *uint32     `xml:"-"`
	FirstSheet             *uint32     `xml:"-"`
	ActiveTab              *uint32     `xml:"-"`
	AutoFilterDateGrouping *bool       `xml:"-"`
	ExtAttrs               []xmlb.Attr `xml:"-"` // extension attrs (e.g., xr2:uid)
}

// UnmarshalXML implements custom unmarshaling for CT_BookView.
func (bv *CT_BookView) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "visibility":
			bv.Visibility = attr.Value
		case "minimized":
			b := attr.Value == "1" || attr.Value == "true"
			bv.Minimized = &b
		case "showHorizontalScroll":
			b := attr.Value == "1" || attr.Value == "true"
			bv.ShowHorizontalScroll = &b
		case "showVerticalScroll":
			b := attr.Value == "1" || attr.Value == "true"
			bv.ShowVerticalScroll = &b
		case "showSheetTabs":
			b := attr.Value == "1" || attr.Value == "true"
			bv.ShowSheetTabs = &b
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
			b := attr.Value == "1" || attr.Value == "true"
			bv.AutoFilterDateGrouping = &b
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
func (bv *CT_BookView) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if bv.Visibility != "" {
		attrs = append(attrs, xmlb.StrAttr("visibility", bv.Visibility))
	}
	if bv.Minimized != nil {
		attrs = append(attrs, xmlb.BoolAttr("minimized", *bv.Minimized))
	}
	if bv.ShowHorizontalScroll != nil {
		attrs = append(attrs, xmlb.BoolAttr("showHorizontalScroll", *bv.ShowHorizontalScroll))
	}
	if bv.ShowVerticalScroll != nil {
		attrs = append(attrs, xmlb.BoolAttr("showVerticalScroll", *bv.ShowVerticalScroll))
	}
	if bv.ShowSheetTabs != nil {
		attrs = append(attrs, xmlb.BoolAttr("showSheetTabs", *bv.ShowSheetTabs))
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
		attrs = append(attrs, xmlb.BoolAttr("autoFilterDateGrouping", *bv.AutoFilterDateGrouping))
	}
	// Append extension attributes (e.g., xr2:uid)
	attrs = append(attrs, bv.ExtAttrs...)
	b.EmptyElement(ns, localName, attrs...)
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
}

// UnmarshalXML implements custom unmarshaling for CT_Sheet.
// Handles the r:id attribute which uses the relationships namespace.
func (s *CT_Sheet) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
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
	CalcId               *uint32  `xml:"calcId,attr,omitempty"`
	CalcMode             string   `xml:"calcMode,attr,omitempty"`
	FullCalcOnLoad       *bool    `xml:"fullCalcOnLoad,attr,omitempty"`
	RefMode              string   `xml:"refMode,attr,omitempty"`
	Iterate              *bool    `xml:"iterate,attr,omitempty"`
	IterateCount         *uint32  `xml:"iterateCount,attr,omitempty"`
	IterateDelta         *float64 `xml:"iterateDelta,attr,omitempty"`
	FullPrecision        *bool    `xml:"fullPrecision,attr,omitempty"`
	CalcCompleted        *bool    `xml:"calcCompleted,attr,omitempty"`
	CalcOnSave           *bool    `xml:"calcOnSave,attr,omitempty"`
	ConcurrentCalc       *bool    `xml:"concurrentCalc,attr,omitempty"`
	ConcurrentManualCount *uint32 `xml:"concurrentManualCount,attr,omitempty"`
	ForceFullCalc        *bool    `xml:"forceFullCalc,attr,omitempty"`
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
}

// UnmarshalXML implements custom unmarshaling for CT_Extension.
func (e *CT_Extension) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Local == "uri" && attr.Name.Space == "" {
			e.URI = attr.Value
		} else if attr.Name.Space == "xmlns" {
			// Capture inline namespace declarations (e.g., xmlns:x15="...")
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
	attrs := []xmlb.Attr{xmlb.StrAttr("uri", e.URI)}
	for _, nsd := range e.InlineNSDecls {
		attrs = append(attrs, xmlb.Attr{Name: "xmlns:" + nsd.Prefix, Value: nsd.URI})
	}
	b.StartElement(ns, localName, attrs...)
	if len(e.RawContent) > 0 {
		b.WriteRaw(e.RawContent)
	}
	b.EndElement(ns, localName)
}
