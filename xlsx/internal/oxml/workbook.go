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
	wbChildFileVersion        = "fileVersion"
	wbChildWorkbookPr         = "workbookPr"
	wbChildWorkbookProtection = "workbookProtection"
	wbChildAlternateContent   = "AlternateContent"
	wbChildBookViews          = "bookViews"
	wbChildSheets             = "sheets"
	wbChildDefinedNames       = "definedNames"
	wbChildCalcPr             = "calcPr"
	wbChildExtLst             = "extLst"
)

// WbUnknownChild represents an unknown child element captured as raw bytes.
type WbUnknownChild struct {
	Data []byte // raw XML bytes including the element itself
}

// CT_Workbook is the root element of workbook.xml.
type CT_Workbook struct {
	XMLName            xml.Name                `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main workbook"`
	Conformance        string                  `xml:"conformance,attr,omitempty"`
	Ignorable          string                  `xml:"-"` // mc:Ignorable attribute value
	FileVersion        *CT_FileVersion         `xml:"fileVersion,omitempty"`
	WorkbookPr         *CT_WorkbookPr          `xml:"workbookPr,omitempty"`
	WorkbookProtection *CT_WorkbookProtection  `xml:"workbookProtection,omitempty"`
	// AlternateContent holds every root-level mc:AlternateContent block in
	// source order. The element is repeatable; a single pointer collapsed two
	// distinct blocks to the last while both ChildOrder entries remained, so a
	// zero-mod save duplicated one and lost the other (C319).
	AlternateContent []*coxml.AlternateContent `xml:"-"`
	BookViews          *CT_BookViews           `xml:"bookViews,omitempty"`
	Sheets             CT_Sheets               `xml:"sheets"`
	DefinedNames       *CT_DefinedNames        `xml:"definedNames,omitempty"`
	CalcPr             *CT_CalcPr              `xml:"calcPr,omitempty"`
	ExtLst             *CT_ExtensionList       `xml:"extLst,omitempty"`
	// PivotCaches is populated only when a pivot table is created this session
	// (Sheet.AddPivotTable). A workbook opened with existing pivot caches keeps
	// them among the verbatim UnknownChildren for byte-identical round-trip; it
	// is nil in that case.
	PivotCaches *CT_PivotCaches `xml:"-"`
	// UnknownChildren stores extension child elements (like xr:revisionPtr)
	// that we don't have typed structs for, indexed for child ordering.
	UnknownChildren []WbUnknownChild `xml:"-"`
	// ChildOrder preserves the order of child elements for round-trip.
	// Each entry is either a known element name (e.g., "fileVersion"),
	// "unknown:N" where N is the index into UnknownChildren, or "ws:N"
	// where N indexes WsRaw (verbatim inter-child whitespace).
	ChildOrder []string `xml:"-"`
	// WsRaw holds the verbatim whitespace between root children; PerGapWS
	// reports that gaps were captured, so the uniform ElemSeparator must not
	// also fire.
	WsRaw    [][]byte `xml:"-"`
	PerGapWS bool     `xml:"-"`
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

	// Capture all root element attributes in order for round-trip
	// preservation, with their verbatim source rendering (leading whitespace,
	// quote style, prefix choice).
	wb.OriginalRootAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	for _, attr := range start.Attr {
		if attr.Name.Local == "conformance" && attr.Name.Space == "" {
			wb.Conformance = attr.Value
		}
		if attr.Name.Local == "Ignorable" {
			wb.Ignorable = attr.Value
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
			isWS := len(t) > 0
			for _, b := range t {
				if b != ' ' && b != '\t' && b != '\n' && b != '\r' {
					isWS = false
					break
				}
			}
			if !isWS {
				continue
			}
			if wb.ElemSeparator == "" {
				wb.ElemSeparator = string(t)
			}
			// Per-gap verbatim capture: producers mix inline and separated
			// children in one workbook (and use CRLF the decoder normalizes),
			// which a single uniform separator cannot express.
			if raw := xmlb.RawTokenBytes(d, tokStart); raw != nil {
				wb.ChildOrder = append(wb.ChildOrder, fmt.Sprintf("ws:%d", len(wb.WsRaw)))
				wb.WsRaw = append(wb.WsRaw, raw)
				wb.PerGapWS = true
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
			case "workbookProtection":
				wb.WorkbookProtection = &CT_WorkbookProtection{}
				if err := d.DecodeElement(wb.WorkbookProtection, &t); err != nil {
					return err
				}
				// Preserve the verbatim source slice so an unmodified round trip
				// re-emits the element byte-for-byte (attribute order, self-closing
				// style), exactly as the unknown-child path did before this element
				// gained a typed model. Protect/Unprotect clear Raw so the typed
				// canonical form is marshaled instead.
				if end := d.InputOffset(); wb.RawSource != nil &&
					tokStart >= 0 && end > tokStart && end <= int64(len(wb.RawSource)) {
					wb.WorkbookProtection.Raw = append([]byte(nil), wb.RawSource[tokStart:end]...)
				}
				wb.ChildOrder = append(wb.ChildOrder, wbChildWorkbookProtection)
			case "AlternateContent":
				ac := &coxml.AlternateContent{}
				if err := d.DecodeElement(ac, &t); err != nil {
					return err
				}
				wb.AlternateContent = append(wb.AlternateContent, ac)
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

// ExistingPivotCacheIDs returns the cache ids of any <pivotCaches> element the
// workbook was opened with (captured verbatim among UnknownChildren). It does
// not modify the workbook, so an unmodified save still round-trips byte-for-byte.
func (wb *CT_Workbook) ExistingPivotCacheIDs() []uint32 {
	for _, uc := range wb.UnknownChildren {
		if IsPivotCachesElement(uc.Data) {
			entries := ParsePivotCachesElement(uc.Data)
			ids := make([]uint32, 0, len(entries))
			for _, e := range entries {
				ids = append(ids, e.CacheId)
			}
			return ids
		}
	}
	return nil
}

// TakeExistingPivotCaches finds the verbatim <pivotCaches> child the workbook
// was opened with, parses its entries, and rewires ChildOrder so the (now
// typed) PivotCaches element is emitted in its place instead of the raw bytes.
// It returns the parsed entries, or nil when the workbook had no pivot caches.
// Call this only when about to (re)emit a typed <pivotCaches>; it mutates
// ChildOrder and so must not run on an unmodified round-trip.
func (wb *CT_Workbook) TakeExistingPivotCaches() []CT_PivotCache {
	for i, uc := range wb.UnknownChildren {
		if !IsPivotCachesElement(uc.Data) {
			continue
		}
		entries := ParsePivotCachesElement(uc.Data)
		marker := "unknown:" + strconv.Itoa(i)
		for j, entry := range wb.ChildOrder {
			if entry == marker {
				wb.ChildOrder[j] = "pivotCaches"
				break
			}
		}
		return entries
	}
	return nil
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
	AppName          string             `xml:"appName,attr,omitempty"`
	LastEdited       string             `xml:"lastEdited,attr,omitempty"`
	LowestEdited     string             `xml:"lowestEdited,attr,omitempty"`
	RupBuild         string             `xml:"rupBuild,attr,omitempty"`
	CapturedAttrs    []xmlb.RootAttr    `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"` // empty-element style; see common/xml.CaptureEmptyTagStyle
}

// UnmarshalXML captures the element's verbatim attribute list and empty-tag
// style before decoding through the struct tags.
func (fv *CT_FileVersion) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	fv.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	fv.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_FileVersion
	return d.DecodeElement((*alias)(fv), &start)
}

// CT_WorkbookProtection represents the workbookProtection element
// (ECMA-376 §18.2.29). It guards the workbook's structure (adding, deleting,
// hiding or reordering sheets) and window layout. Excel enforces it only as a
// UI convenience; the workbook is not encrypted and any tool can clear it.
//
// The password/hash attributes carry either Excel's legacy 16-bit obfuscation
// hash (workbookPassword) or a modern iterated hash (workbookHashValue +
// workbookSaltValue + workbookSpinCount + workbookAlgorithmName). Neither is
// exposed once written.
type CT_WorkbookProtection struct {
	WorkbookPassword       string `xml:"workbookPassword,attr,omitempty"`
	WorkbookHashValue      string `xml:"workbookHashValue,attr,omitempty"`
	WorkbookSaltValue      string `xml:"workbookSaltValue,attr,omitempty"`
	WorkbookSpinCount      string `xml:"workbookSpinCount,attr,omitempty"`
	WorkbookAlgorithmName  string `xml:"workbookAlgorithmName,attr,omitempty"`
	RevisionsPassword      string `xml:"revisionsPassword,attr,omitempty"`
	RevisionsHashValue     string `xml:"revisionsHashValue,attr,omitempty"`
	RevisionsSaltValue     string `xml:"revisionsSaltValue,attr,omitempty"`
	RevisionsSpinCount     string `xml:"revisionsSpinCount,attr,omitempty"`
	RevisionsAlgorithmName string `xml:"revisionsAlgorithmName,attr,omitempty"`
	LockStructure          *bool  `xml:"lockStructure,attr,omitempty"`
	LockWindows            *bool  `xml:"lockWindows,attr,omitempty"`
	LockRevision           *bool  `xml:"lockRevision,attr,omitempty"`
	// Raw preserves the verbatim source bytes captured at parse time. When
	// non-nil (element parsed and not since modified) it is re-emitted as-is for
	// byte-identical round-trip; Protect/Unprotect set it to nil so the typed
	// fields are marshaled canonically instead.
	Raw []byte `xml:"-"`
}

// CT_WorkbookPr represents the workbookPr element.
type CT_WorkbookPr struct {
	Date1904                   *BoolLex           `xml:"date1904,attr,omitempty"`
	ShowObjects                string             `xml:"showObjects,attr,omitempty"`
	ShowBorderUnselectedTables *BoolLex           `xml:"showBorderUnselectedTables,attr,omitempty"`
	FilterPrivacy              *BoolLex           `xml:"filterPrivacy,attr,omitempty"`
	PromptedSolutions          *BoolLex           `xml:"promptedSolutions,attr,omitempty"`
	ShowInkAnnotation          *BoolLex           `xml:"showInkAnnotation,attr,omitempty"`
	BackupFile                 *BoolLex           `xml:"backupFile,attr,omitempty"`
	SaveExternalLinkValues     *BoolLex           `xml:"saveExternalLinkValues,attr,omitempty"`
	UpdateLinks                string             `xml:"updateLinks,attr,omitempty"`
	CodeName                   string             `xml:"codeName,attr,omitempty"`
	HidePivotFieldList         *BoolLex           `xml:"hidePivotFieldList,attr,omitempty"`
	ShowPivotChartFilter       *BoolLex           `xml:"showPivotChartFilter,attr,omitempty"`
	AllowRefreshQuery          *BoolLex           `xml:"allowRefreshQuery,attr,omitempty"`
	CheckCompatibility         *BoolLex           `xml:"checkCompatibility,attr,omitempty"`
	AutoCompressPictures       *BoolLex           `xml:"autoCompressPictures,attr,omitempty"`
	DefaultThemeVersion        *uint32            `xml:"defaultThemeVersion,attr,omitempty"`
	CapturedAttrs              []xmlb.RootAttr    `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
	CapturedEmptyTag           xmlb.EmptyTagStyle `xml:"-"` // empty-element style; see common/xml.CaptureEmptyTagStyle
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (wp *CT_WorkbookPr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	wp.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	wp.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_WorkbookPr
	return d.DecodeElement((*alias)(wp), &start)
}

// CT_BookViews represents the bookViews element.
type CT_BookViews struct {
	WorkbookView []CT_BookView `xml:"workbookView"`
	// CapturedChildren records the child sequence including verbatim
	// inter-child whitespace (pretty-printed producers).
	CapturedChildren *xmlb.ChildCapture `xml:"-"`
}

// UnmarshalXML captures the child sequence (and whitespace) while decoding.
func (bv *CT_BookViews) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	return xmlb.UnmarshalOrderedChildren(d, bv)
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
	attrOrder        []string
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"` // empty-element style; see common/xml.CaptureEmptyTagStyle
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
	bv.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
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
		b.EmptyElementStyled(bv.CapturedEmptyTag, ns, localName, attrs...)
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
	b.EmptyElementStyled(bv.CapturedEmptyTag, ns, localName, attrs...)
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
	// CapturedChildren records the child sequence including verbatim
	// inter-child whitespace (pretty-printed producers).
	CapturedChildren *xmlb.ChildCapture `xml:"-"`
}

// UnmarshalXML captures the child sequence (and whitespace) while decoding.
func (sh *CT_Sheets) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	return xmlb.UnmarshalOrderedChildren(d, sh)
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
	// CapturedAttrs preserves the verbatim source attribute list; replayed
	// on marshal (LibreOffice writes state before name).
	CapturedAttrs    []xmlb.RootAttr    `xml:"-"`
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"` // empty-element style; see common/xml.CaptureEmptyTagStyle
}

// UnmarshalXML implements custom unmarshaling for CT_Sheet.
// Handles the r:id attribute which uses the relationships namespace.
func (s *CT_Sheet) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	s.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	s.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
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

// SetState updates the sheet's visibility state ("" = visible, "hidden" or
// "veryHidden") and reconciles the captured source attribute list. A parsed
// sheet replays its verbatim attributes (CapturedAttrs), where ReplayCapturedAttrs
// substitutes the modeled State value when a "state" attribute is present but
// re-emits an unmatched captured attribute verbatim. Making a hidden sheet
// visible drops the modeled attribute entirely, so the stale captured "state"
// must be removed here or the sheet would stay hidden on save.
func (s *CT_Sheet) SetState(state string) {
	s.State = state
	if state != "" || s.CapturedAttrs == nil {
		return
	}
	for i := range s.CapturedAttrs {
		ra := &s.CapturedAttrs[i]
		if !ra.IsNS && ra.Space == "" && ra.LocalName == "state" {
			s.CapturedAttrs = append(s.CapturedAttrs[:i], s.CapturedAttrs[i+1:]...)
			return
		}
	}
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
	if s.CapturedAttrs != nil {
		// Verbatim replay: producer attribute order and inline declarations
		// (which the capture already contains) both survive.
		b.EmptyElementStyled(s.CapturedEmptyTag, ns, localName, b.ReplayCapturedAttrs(s.CapturedAttrs, attrs)...)
		return
	}
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
	// CapturedEmptyTag records how an empty <definedNames> was written.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
	// CapturedChildren records the child sequence including verbatim
	// inter-child whitespace (pretty-printed producers).
	CapturedChildren *xmlb.ChildCapture `xml:"-"`
}

// UnmarshalXML captures the element's empty-tag style before decoding the
// definedName children.
func (dns *CT_DefinedNames) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	dns.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	return xmlb.UnmarshalOrderedChildren(d, dns)
}

// CT_DefinedName represents a definedName element. It has chardata value plus
// attributes, requiring custom UnmarshalXML and MarshalToBuilder.
type CT_DefinedName struct {
	Name         string  `xml:"-"`
	Comment      string  `xml:"-"`
	CustomMenu   string  `xml:"-"`
	Description  string  `xml:"-"`
	Help         string  `xml:"-"`
	StatusBar    string  `xml:"-"`
	LocalSheetId *uint32 `xml:"-"`
	// The boolean attributes hold BoolLex so a source's lexical form
	// ("true"/"false" from LibreOffice vs "1"/"0" from Excel) round-trips.
	Hidden            *BoolLex `xml:"-"`
	Function          *BoolLex `xml:"-"`
	VbProcedure       *BoolLex `xml:"-"`
	Xlm               *BoolLex `xml:"-"`
	FunctionGroupId   *uint32  `xml:"-"`
	ShortcutKey       string   `xml:"-"`
	PublishToServer   *BoolLex `xml:"-"`
	WorkbookParameter *BoolLex `xml:"-"`
	Value             string   `xml:"-"`
	// CapturedAttrs preserves the verbatim source attribute list; replayed
	// on marshal.
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
	// rawValue preserves the verbatim source form of the value — producer
	// entity choices (&apos;, &#039;, &quot;) and raw CR bytes in _x000D_
	// sequences the decoder normalizes. Replayed only while Value still
	// equals origValue, so edits win.
	rawValue  []byte
	origValue string
}

// UnmarshalXML implements custom unmarshaling for CT_DefinedName.
func (dn *CT_DefinedName) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	dn.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
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
			v := &BoolLex{}
			if err := v.UnmarshalXMLAttr(attr); err != nil {
				return err
			}
			dn.Hidden = v
		case "function":
			v := &BoolLex{}
			if err := v.UnmarshalXMLAttr(attr); err != nil {
				return err
			}
			dn.Function = v
		case "vbProcedure":
			v := &BoolLex{}
			if err := v.UnmarshalXMLAttr(attr); err != nil {
				return err
			}
			dn.VbProcedure = v
		case "xlm":
			v := &BoolLex{}
			if err := v.UnmarshalXMLAttr(attr); err != nil {
				return err
			}
			dn.Xlm = v
		case "functionGroupId":
			if n, err := strconv.ParseUint(attr.Value, 10, 32); err == nil {
				v := uint32(n)
				dn.FunctionGroupId = &v
			}
		case "shortcutKey":
			dn.ShortcutKey = attr.Value
		case "publishToServer":
			v := &BoolLex{}
			if err := v.UnmarshalXMLAttr(attr); err != nil {
				return err
			}
			dn.PublishToServer = v
		case "workbookParameter":
			v := &BoolLex{}
			if err := v.UnmarshalXMLAttr(attr); err != nil {
				return err
			}
			dn.WorkbookParameter = v
		}
	}

	// Read chardata content, keeping its verbatim source form for replay.
	style := xmlb.CaptureEmptyTagStyle(d)
	innerStart, hasSrc := xmlb.InputOffsetOf(d)
	var content struct {
		Value string `xml:",chardata"`
	}
	if err := d.DecodeElement(&content, &start); err != nil {
		return err
	}
	dn.Value = content.Value
	if hasSrc && !style.IsSelfClose() {
		dn.rawValue = xmlb.CaptureRawInner(d, innerStart)
		dn.origValue = dn.Value
	}
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
		attrs = append(attrs, xmlb.StrAttr("hidden", dn.Hidden.AttrValue()))
	}
	if dn.Function != nil {
		attrs = append(attrs, xmlb.StrAttr("function", dn.Function.AttrValue()))
	}
	if dn.VbProcedure != nil {
		attrs = append(attrs, xmlb.StrAttr("vbProcedure", dn.VbProcedure.AttrValue()))
	}
	if dn.Xlm != nil {
		attrs = append(attrs, xmlb.StrAttr("xlm", dn.Xlm.AttrValue()))
	}
	if dn.FunctionGroupId != nil {
		attrs = append(attrs, xmlb.UintAttr("functionGroupId", *dn.FunctionGroupId))
	}
	if dn.ShortcutKey != "" {
		attrs = append(attrs, xmlb.StrAttr("shortcutKey", dn.ShortcutKey))
	}
	if dn.PublishToServer != nil {
		attrs = append(attrs, xmlb.StrAttr("publishToServer", dn.PublishToServer.AttrValue()))
	}
	if dn.WorkbookParameter != nil {
		attrs = append(attrs, xmlb.StrAttr("workbookParameter", dn.WorkbookParameter.AttrValue()))
	}
	if dn.CapturedAttrs != nil {
		attrs = b.ReplayCapturedAttrs(dn.CapturedAttrs, attrs)
	}
	if dn.rawValue != nil && dn.Value == dn.origValue {
		// Unedited value: replay the verbatim source form (entity choices,
		// raw CR bytes).
		b.StartElement(ns, localName, attrs...)
		b.WriteRaw(dn.rawValue)
		b.EndElement(ns, localName)
		return
	}
	b.WriteElement(ns, localName, dn.Value, attrs...)
}

// CT_CalcPr represents the calcPr element.
type CT_CalcPr struct {
	CalcId                *uint32            `xml:"calcId,attr,omitempty"`
	CalcMode              string             `xml:"calcMode,attr,omitempty"`
	CalcCompleted         *BoolLex           `xml:"calcCompleted,attr,omitempty"`
	FullCalcOnLoad        *BoolLex           `xml:"fullCalcOnLoad,attr,omitempty"`
	RefMode               string             `xml:"refMode,attr,omitempty"`
	Iterate               *BoolLex           `xml:"iterate,attr,omitempty"`
	IterateCount          *uint32            `xml:"iterateCount,attr,omitempty"`
	IterateDelta          *FloatLex          `xml:"iterateDelta,attr,omitempty"`
	FullPrecision         *BoolLex           `xml:"fullPrecision,attr,omitempty"`
	CalcOnSave            *BoolLex           `xml:"calcOnSave,attr,omitempty"`
	ConcurrentCalc        *BoolLex           `xml:"concurrentCalc,attr,omitempty"`
	ConcurrentManualCount *uint32            `xml:"concurrentManualCount,attr,omitempty"`
	ForceFullCalc         *BoolLex           `xml:"forceFullCalc,attr,omitempty"`
	CapturedAttrs         []xmlb.RootAttr    `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
	CapturedEmptyTag      xmlb.EmptyTagStyle `xml:"-"` // empty-element style; see common/xml.CaptureEmptyTagStyle
}

// SetFullCalcOnLoad sets (v non-nil) or clears (v nil) the fullCalcOnLoad flag,
// reconciling the captured source attribute list. A parsed calcPr replays its
// verbatim attributes; ReplayCapturedAttrs substitutes the modeled value when
// the attribute is present but re-emits an unmatched captured attribute
// verbatim, so clearing the flag must remove the stale capture or it would
// persist on save.
func (cp *CT_CalcPr) SetFullCalcOnLoad(v *BoolLex) {
	cp.FullCalcOnLoad = v
	if v != nil || cp.CapturedAttrs == nil {
		return
	}
	for i := range cp.CapturedAttrs {
		ra := &cp.CapturedAttrs[i]
		if !ra.IsNS && ra.Space == "" && ra.LocalName == "fullCalcOnLoad" {
			cp.CapturedAttrs = append(cp.CapturedAttrs[:i], cp.CapturedAttrs[i+1:]...)
			return
		}
	}
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (cp *CT_CalcPr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	cp.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	cp.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_CalcPr
	return d.DecodeElement((*alias)(cp), &start)
}

// CT_ExtensionList represents the extLst element.
type CT_ExtensionList struct {
	Ext []CT_Extension `xml:"ext"`
	// CapturedAttrs preserves the extLst element's verbatim attribute list
	// (some producers declare the extension namespace here, e.g.
	// <extLst xmlns:x15="...">); nil for lists built programmatically.
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
}

// UnmarshalXML captures the element's verbatim attribute list before decoding
// the ext children.
func (el *CT_ExtensionList) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	el.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_ExtensionList
	return d.DecodeElement((*alias)(el), &start)
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
	// ElemPrefix is the element name's verbatim source prefix (some
	// producers write <x:ext> with xmlns:x bound to the SpreadsheetML
	// namespace on the element itself).
	ElemPrefix         string `xml:"-"`
	elemPrefixCaptured bool
}

// UnmarshalXML implements custom unmarshaling for CT_Extension.
func (e *CT_Extension) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	e.ElemPrefix, e.elemPrefixCaptured = xmlb.ElementPrefix(d)
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
	if e.elemPrefixCaptured && e.ElemPrefix != "" {
		// Replay the element under its verbatim source prefix (<x:ext>).
		lit := b.QualifyAttrs(attrs)
		if len(e.RawContent) == 0 {
			b.EmptyElementLiteral(e.ElemPrefix, localName, lit...)
			return
		}
		b.StartElementLiteral(e.ElemPrefix, localName, nil, lit...)
		b.WriteRaw(e.RawContent)
		b.EndElementLiteral(e.ElemPrefix, localName)
		return
	}
	b.StartElement(ns, localName, attrs...)
	if len(e.RawContent) > 0 {
		b.WriteRaw(e.RawContent)
	}
	b.EndElement(ns, localName)
}
