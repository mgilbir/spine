package oxml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// x14ac namespace for spreadsheet extensions (e.g., dyDescent attribute).
const nsX14AC = "http://schemas.microsoft.com/office/spreadsheetml/2009/9/ac"

// ---------------------------------------------------------------------------
// Root type
// ---------------------------------------------------------------------------

// CT_Worksheet is the root element of a worksheet part (sheet*.xml).
type CT_Worksheet struct {
	XMLName               xml.Name                   `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main worksheet"`
	SheetPr               *CT_SheetPr                `xml:"sheetPr,omitempty"`
	Dimension             *CT_SheetDimension         `xml:"dimension,omitempty"`
	SheetViews            *CT_SheetViews             `xml:"sheetViews,omitempty"`
	SheetFormatPr         *CT_SheetFormatPr          `xml:"sheetFormatPr,omitempty"`
	Cols                  []CT_Cols                  `xml:"cols"`
	SheetData             CT_SheetData               `xml:"sheetData"`
	SheetCalcPr           *CT_SheetCalcPr            `xml:"sheetCalcPr,omitempty"`
	SheetProtection       *CT_SheetProtection        `xml:"sheetProtection,omitempty"`
	Scenarios             *CT_Scenarios              `xml:"-"`
	AutoFilter            *CT_AutoFilter             `xml:"autoFilter,omitempty"`
	SortState             *CT_SortState              `xml:"sortState,omitempty"`
	MergeCells            *CT_MergeCells             `xml:"mergeCells,omitempty"`
	PhoneticPr            *CT_PhoneticPr             `xml:"phoneticPr,omitempty"`
	ConditionalFormatting []CT_ConditionalFormatting `xml:"conditionalFormatting"`
	DataValidations       *CT_DataValidations        `xml:"dataValidations,omitempty"`
	Hyperlinks            *CT_Hyperlinks             `xml:"hyperlinks,omitempty"`
	PrintOptions          *CT_PrintOptions           `xml:"printOptions,omitempty"`
	PageMargins           *CT_PageMargins            `xml:"pageMargins,omitempty"`
	PageSetup             *CT_PageSetup              `xml:"pageSetup,omitempty"`
	HeaderFooter          *CT_HeaderFooter           `xml:"headerFooter,omitempty"`
	RowBreaks             *CT_PageBreak              `xml:"rowBreaks,omitempty"`
	ColBreaks             *CT_PageBreak              `xml:"colBreaks,omitempty"`
	Drawing               *CT_Drawing                `xml:"drawing,omitempty"`
	LegacyDrawing         *CT_LegacyDrawing          `xml:"legacyDrawing,omitempty"`
	OleObjects            *CT_OleObjects             `xml:"-"`
	TableParts            *CT_TableParts             `xml:"tableParts,omitempty"`
	ExtLst                *CT_ExtensionList          `xml:"extLst,omitempty"`
	OriginalNSDecls       []xmlb.NSDecl              `xml:"-"`
	// OriginalRootAttrs preserves all root-element attributes (namespace
	// declarations and regular attributes like mc:Ignorable / xr:uid) in order.
	OriginalRootAttrs []xmlb.RootAttr `xml:"-"`
	// ChildOrder preserves the order of child elements; each entry is a known
	// element local name or "unknown:N" indexing UnknownChildren.
	ChildOrder []string `xml:"-"`
	// UnknownChildren captures child elements without a typed model (e.g.
	// oleObjects, controls, customSheetViews) as raw bytes, so editing a sheet
	// no longer deletes them.
	UnknownChildren []WbUnknownChild `xml:"-"`
	// ElemSeparator is inter-element whitespace for round-trip formatting.
	ElemSeparator string `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_Worksheet. It preserves
// the root attributes, child order, and any unknown child elements so that a
// re-marshaled (dirty) sheet does not silently drop content it lacks a typed
// model for (oleObjects, controls, customSheetViews, scenarios, ...).
func (ws *CT_Worksheet) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	ws.XMLName = start.Name

	// Pre-scan every namespace declaration on the root so a regular attribute
	// whose xmlns decl is written after it on the same tag still resolves its
	// prefix (C324); scanning only declarations seen so far dropped the prefix.
	rootNSByURI := rootDeclPrefixes(start.Attr)

	// Capture all root-element attributes in order for round-trip preservation,
	// distinguishing namespace declarations from regular attributes.
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Space == "xmlns":
			ws.OriginalNSDecls = append(ws.OriginalNSDecls, xmlb.NSDecl{Prefix: attr.Name.Local, URI: attr.Value})
			ws.OriginalRootAttrs = append(ws.OriginalRootAttrs, xmlb.RootAttr{IsNS: true, Prefix: attr.Name.Local, Value: attr.Value})
		case attr.Name.Space == "" && attr.Name.Local == "xmlns":
			ws.OriginalNSDecls = append([]xmlb.NSDecl{{Prefix: "", URI: attr.Value}}, ws.OriginalNSDecls...)
			ws.OriginalRootAttrs = append(ws.OriginalRootAttrs, xmlb.RootAttr{IsNS: true, Prefix: "", Value: attr.Value})
		default:
			prefix := resolveRootAttrPrefix(attr.Name.Space, rootNSByURI)
			ws.OriginalRootAttrs = append(ws.OriginalRootAttrs, xmlb.RootAttr{IsNS: false, Prefix: prefix, LocalName: attr.Name.Local, Value: attr.Value})
		}
	}

	// Map namespace URIs to prefixes for reconstructing unknown children.
	nsPrefixMap := make(map[string]string)
	for _, ra := range ws.OriginalRootAttrs {
		if ra.IsNS && ra.Prefix != "" {
			nsPrefixMap[ra.Value] = ra.Prefix
		}
	}

	// Tracks the non-repeatable children already recorded in ChildOrder (C553).
	seenSingleton := make(map[string]bool)

	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.CharData:
			// Capture inter-element whitespace for round-trip formatting.
			if ws.ElemSeparator == "" && len(t) > 0 {
				isWS := true
				for _, b := range t {
					if b != ' ' && b != '\t' && b != '\n' && b != '\r' {
						isWS = false
						break
					}
				}
				if isWS {
					ws.ElemSeparator = string(t)
				}
			}
		case xml.StartElement:
			name := t.Name.Local
			switch name {
			case "sheetPr":
				ws.SheetPr = &CT_SheetPr{}
				if err := d.DecodeElement(ws.SheetPr, &t); err != nil {
					return err
				}
			case "dimension":
				ws.Dimension = &CT_SheetDimension{}
				if err := d.DecodeElement(ws.Dimension, &t); err != nil {
					return err
				}
			case "sheetViews":
				ws.SheetViews = &CT_SheetViews{}
				if err := d.DecodeElement(ws.SheetViews, &t); err != nil {
					return err
				}
			case "sheetFormatPr":
				ws.SheetFormatPr = &CT_SheetFormatPr{}
				if err := ws.SheetFormatPr.UnmarshalXML(d, t); err != nil {
					return err
				}
			case "cols":
				var cols CT_Cols
				if err := d.DecodeElement(&cols, &t); err != nil {
					return err
				}
				ws.Cols = append(ws.Cols, cols)
			case "sheetData":
				if err := ws.SheetData.UnmarshalXML(d, t); err != nil {
					return err
				}
			case "sheetCalcPr":
				ws.SheetCalcPr = &CT_SheetCalcPr{}
				if err := d.DecodeElement(ws.SheetCalcPr, &t); err != nil {
					return err
				}
			case "sheetProtection":
				ws.SheetProtection = &CT_SheetProtection{}
				if err := d.DecodeElement(ws.SheetProtection, &t); err != nil {
					return err
				}
			case "scenarios":
				// Capture the element verbatim (reconstructed from its innerxml)
				// so an unmodified sheet round-trips byte-for-byte, and also parse
				// its typed model so the what-if scenarios can be read and appended
				// to. Authoring (Scenarios.Dirty) switches marshaling to the typed
				// model; an untouched element re-emits Raw.
				var raw struct {
					Content []byte `xml:",innerxml"`
				}
				if err := d.DecodeElement(&raw, &t); err != nil {
					return err
				}
				sc := &CT_Scenarios{Raw: encodeUnknownElement(t, raw.Content, nsPrefixMap)}
				sc.parse(t, raw.Content)
				ws.Scenarios = sc
			case "autoFilter":
				ws.AutoFilter = &CT_AutoFilter{}
				if err := d.DecodeElement(ws.AutoFilter, &t); err != nil {
					return err
				}
			case "sortState":
				ws.SortState = &CT_SortState{}
				if err := d.DecodeElement(ws.SortState, &t); err != nil {
					return err
				}
			case "mergeCells":
				ws.MergeCells = &CT_MergeCells{}
				if err := d.DecodeElement(ws.MergeCells, &t); err != nil {
					return err
				}
			case "phoneticPr":
				ws.PhoneticPr = &CT_PhoneticPr{}
				if err := d.DecodeElement(ws.PhoneticPr, &t); err != nil {
					return err
				}
			case "conditionalFormatting":
				var cf CT_ConditionalFormatting
				if err := d.DecodeElement(&cf, &t); err != nil {
					return err
				}
				ws.ConditionalFormatting = append(ws.ConditionalFormatting, cf)
			case "dataValidations":
				ws.DataValidations = &CT_DataValidations{}
				if err := d.DecodeElement(ws.DataValidations, &t); err != nil {
					return err
				}
			case "hyperlinks":
				ws.Hyperlinks = &CT_Hyperlinks{}
				if err := d.DecodeElement(ws.Hyperlinks, &t); err != nil {
					return err
				}
			case "printOptions":
				ws.PrintOptions = &CT_PrintOptions{}
				if err := d.DecodeElement(ws.PrintOptions, &t); err != nil {
					return err
				}
			case "pageMargins":
				ws.PageMargins = &CT_PageMargins{}
				if err := d.DecodeElement(ws.PageMargins, &t); err != nil {
					return err
				}
			case "pageSetup":
				ws.PageSetup = &CT_PageSetup{}
				if err := ws.PageSetup.UnmarshalXML(d, t); err != nil {
					return err
				}
			case "headerFooter":
				ws.HeaderFooter = &CT_HeaderFooter{}
				if err := d.DecodeElement(ws.HeaderFooter, &t); err != nil {
					return err
				}
			case "rowBreaks":
				ws.RowBreaks = &CT_PageBreak{}
				if err := d.DecodeElement(ws.RowBreaks, &t); err != nil {
					return err
				}
			case "colBreaks":
				ws.ColBreaks = &CT_PageBreak{}
				if err := d.DecodeElement(ws.ColBreaks, &t); err != nil {
					return err
				}
			case "drawing":
				ws.Drawing = &CT_Drawing{}
				if err := ws.Drawing.UnmarshalXML(d, t); err != nil {
					return err
				}
			case "legacyDrawing":
				ws.LegacyDrawing = &CT_LegacyDrawing{}
				if err := ws.LegacyDrawing.UnmarshalXML(d, t); err != nil {
					return err
				}
			case "oleObjects":
				// Capture verbatim (reconstructed) so an unmodified sheet
				// round-trips byte-for-byte, and parse the typed model so embedded
				// OLE objects can be enumerated and appended. Authoring
				// (OleObjects.Dirty) switches marshaling to the typed model.
				var raw struct {
					Content []byte `xml:",innerxml"`
				}
				if err := d.DecodeElement(&raw, &t); err != nil {
					return err
				}
				o := &CT_OleObjects{Raw: encodeUnknownElement(t, raw.Content, nsPrefixMap)}
				o.parse(t, raw.Content)
				ws.OleObjects = o
			case "tableParts":
				ws.TableParts = &CT_TableParts{}
				if err := d.DecodeElement(ws.TableParts, &t); err != nil {
					return err
				}
			case "extLst":
				ws.ExtLst = &CT_ExtensionList{}
				if err := d.DecodeElement(ws.ExtLst, &t); err != nil {
					return err
				}
			default:
				// Capture unknown children as raw bytes rather than dropping them.
				var raw struct {
					Content []byte `xml:",innerxml"`
				}
				if err := d.DecodeElement(&raw, &t); err != nil {
					return err
				}
				idx := len(ws.UnknownChildren)
				ws.UnknownChildren = append(ws.UnknownChildren, WbUnknownChild{
					Data: encodeUnknownElement(t, raw.Content, nsPrefixMap),
				})
				ws.ChildOrder = append(ws.ChildOrder, fmt.Sprintf("unknown:%d", idx))
				continue
			}
			// A malformed source can repeat a child the schema declares once
			// (two <mergeCells> blocks). Parsing is last-wins, but recording a
			// second ChildOrder entry made the marshaler emit the surviving
			// block twice — losing the first block's content and duplicating
			// the second (C553). Repeatable children (cols,
			// conditionalFormatting) still get one entry each.
			if !worksheetRepeatableChildren[name] && seenSingleton[name] {
				continue
			}
			seenSingleton[name] = true
			ws.ChildOrder = append(ws.ChildOrder, name)
		case xml.EndElement:
			return nil
		}
	}
}

// worksheetRepeatableChildren are the CT_Worksheet children that may legally
// occur more than once and therefore need one ChildOrder entry per occurrence.
var worksheetRepeatableChildren = map[string]bool{
	"cols":                  true,
	"conditionalFormatting": true,
}

// rootDeclPrefixes maps each namespace URI declared on a root element to the
// prefix it was bound to (empty string for the default xmlns), scanning the
// complete attribute list so a declaration written after the attribute that
// uses it still resolves.
func rootDeclPrefixes(attrs []xml.Attr) map[string]string {
	m := make(map[string]string, len(attrs))
	for _, attr := range attrs {
		switch {
		case attr.Name.Space == "xmlns":
			m[attr.Value] = attr.Name.Local
		case attr.Name.Space == "" && attr.Name.Local == "xmlns":
			m[attr.Value] = ""
		}
	}
	return m
}

// resolveRootAttrPrefix returns the prefix a namespaced root attribute is
// written with: none when unqualified, the reserved xml prefix, a prefix
// declared anywhere on the same root element, or the well-known mc/r prefix as
// a fallback when the attribute's namespace was not declared on this element.
func resolveRootAttrPrefix(space string, declared map[string]string) string {
	switch space {
	case "":
		return ""
	case xmlb.NSXML:
		// Reserved prefix, never declared: xml:space etc.
		return "xml"
	}
	if p, ok := declared[space]; ok {
		return p
	}
	switch space {
	case xmlb.NSMarkupCompatibility:
		return xmlb.PrefixMarkupCompatibility
	case nsR:
		return "r"
	}
	return ""
}

// worksheetSchemaSeq is the CT_Worksheet child element sequence from the
// SpreadsheetML schema (ECMA-376 sml.xsd). It includes elements this package
// has no typed model for (protectedRanges, oleObjects, ...) so that captured
// unknown children can still be ranked when computing a schema-ordered
// insertion point.
var worksheetSchemaSeq = []string{
	"sheetPr", "dimension", "sheetViews", "sheetFormatPr", "cols", "sheetData",
	"sheetCalcPr", "sheetProtection", "protectedRanges", "scenarios",
	"autoFilter", "sortState", "dataConsolidate", "customSheetViews",
	"mergeCells", "phoneticPr", "conditionalFormatting", "dataValidations",
	"hyperlinks", "printOptions", "pageMargins", "pageSetup", "headerFooter",
	"rowBreaks", "colBreaks", "customProperties", "cellWatches",
	"ignoredErrors", "smartTags", "drawing", "legacyDrawing",
	"legacyDrawingHF", "drawingHF", "picture", "oleObjects", "controls",
	"webPublishItems", "tableParts", "extLst",
}

// worksheetChildRank maps a CT_Worksheet child local name to its index in
// worksheetSchemaSeq.
var worksheetChildRank = func() map[string]int {
	m := make(map[string]int, len(worksheetSchemaSeq))
	for i, n := range worksheetSchemaSeq {
		m[n] = i
	}
	return m
}()

// EnsureChildOrder records name in ws.ChildOrder at its CT_Worksheet schema
// position if it is not already present. Mutators must call this when they
// populate a child element kind the parsed sheet did not contain: marshaling
// of opened sheets is ChildOrder-gated, so a populated child without an entry
// would be silently dropped (C157). It is a no-op when ChildOrder is empty
// (new sheets marshal in fixed schema order) or when name is already listed.
//
// The insertion point is immediately before the first existing entry whose
// schema rank exceeds name's rank. "unknown:N" entries are ranked by the
// captured element's local name when it is a standard worksheet child we lack
// a typed model for; otherwise they impose no constraint and keep their
// position relative to the surrounding known children.
func (ws *CT_Worksheet) EnsureChildOrder(name string) {
	if len(ws.ChildOrder) == 0 {
		return
	}
	rank, ok := worksheetChildRank[name]
	if !ok {
		return
	}
	for _, entry := range ws.ChildOrder {
		if entry == name {
			return
		}
	}
	insert := len(ws.ChildOrder)
	for i, entry := range ws.ChildOrder {
		if r, ok := ws.childOrderEntryRank(entry); ok && r > rank {
			insert = i
			break
		}
	}
	ws.ChildOrder = append(ws.ChildOrder, "")
	copy(ws.ChildOrder[insert+1:], ws.ChildOrder[insert:])
	ws.ChildOrder[insert] = name
}

// AppendConditionalFormattingOrder records one additional conditionalFormatting
// entry in ws.ChildOrder so a newly appended CT_ConditionalFormatting block is
// emitted by the ChildOrder-gated marshaler. Unlike EnsureChildOrder it is not
// idempotent: conditionalFormatting is a repeatable element and each block needs
// its own entry (the marshaler consumes one block per entry). A new block is
// placed immediately after the last existing conditionalFormatting entry, or at
// its schema position when the sheet had none. It is a no-op when ChildOrder is
// empty (new sheets marshal in fixed schema order and emit every block).
func (ws *CT_Worksheet) AppendConditionalFormattingOrder() {
	if len(ws.ChildOrder) == 0 {
		return
	}
	const name = "conditionalFormatting"
	lastCF := -1
	for i, entry := range ws.ChildOrder {
		if entry == name {
			lastCF = i
		}
	}
	insert := len(ws.ChildOrder)
	if lastCF >= 0 {
		insert = lastCF + 1
	} else {
		rank := worksheetChildRank[name]
		for i, entry := range ws.ChildOrder {
			if r, ok := ws.childOrderEntryRank(entry); ok && r > rank {
				insert = i
				break
			}
		}
	}
	ws.ChildOrder = append(ws.ChildOrder, "")
	copy(ws.ChildOrder[insert+1:], ws.ChildOrder[insert:])
	ws.ChildOrder[insert] = name
}

// childOrderEntryRank resolves the schema rank of a ChildOrder entry. Known
// element names are looked up directly; "unknown:N" entries are ranked by the
// local name of the captured element when it is a standard worksheet child.
func (ws *CT_Worksheet) childOrderEntryRank(entry string) (int, bool) {
	if idx, isUnknown := strings.CutPrefix(entry, "unknown:"); isUnknown {
		n, err := strconv.Atoi(idx)
		if err != nil || n < 0 || n >= len(ws.UnknownChildren) {
			return 0, false
		}
		r, ok := worksheetChildRank[unknownElementLocalName(ws.UnknownChildren[n].Data)]
		return r, ok
	}
	r, ok := worksheetChildRank[entry]
	return r, ok
}

// unknownElementLocalName extracts the element local name (namespace prefix
// stripped) from raw XML that starts with the element's start tag. It returns
// "" if the bytes do not start with a tag.
func unknownElementLocalName(raw []byte) string {
	if len(raw) == 0 || raw[0] != '<' {
		return ""
	}
	end := 1
	for end < len(raw) {
		switch raw[end] {
		case ' ', '\t', '\r', '\n', '>', '/':
			name := string(raw[1:end])
			if i := strings.LastIndexByte(name, ':'); i >= 0 {
				name = name[i+1:]
			}
			return name
		}
		end++
	}
	return ""
}

// ---------------------------------------------------------------------------
// Sheet properties & dimension
// ---------------------------------------------------------------------------

// CT_SheetPr represents the sheetPr element.
type CT_SheetPr struct {
	CodeName                          string `xml:"codeName,attr,omitempty"`
	EnableFormatConditionsCalculation *bool  `xml:"enableFormatConditionsCalculation,attr,omitempty"`
	FilterMode                        *bool  `xml:"filterMode,attr,omitempty"`
	Published                         *bool  `xml:"published,attr,omitempty"`
	SyncHorizontal                    *bool  `xml:"syncHorizontal,attr,omitempty"`
	SyncVertical                      *bool  `xml:"syncVertical,attr,omitempty"`
	// SyncRef is the anchor cell the synchronized-scrolling pair uses; a sheet
	// with syncHorizontal/syncVertical set but no syncRef loses the pairing.
	SyncRef              string          `xml:"syncRef,attr,omitempty"`
	TransitionEntry      *bool           `xml:"transitionEntry,attr,omitempty"`
	TransitionEvaluation *bool           `xml:"transitionEvaluation,attr,omitempty"`
	TabColor             *CT_Color       `xml:"tabColor,omitempty"`
	OutlinePr            *CT_OutlinePr   `xml:"outlinePr,omitempty"`
	PageSetUpPr          *CT_PageSetUpPr `xml:"pageSetUpPr,omitempty"`
}

// CT_OutlinePr represents the outlinePr element.
type CT_OutlinePr struct {
	ApplyStyles        *bool `xml:"applyStyles,attr,omitempty"`
	SummaryBelow       *bool `xml:"summaryBelow,attr,omitempty"`
	SummaryRight       *bool `xml:"summaryRight,attr,omitempty"`
	ShowOutlineSymbols *bool `xml:"showOutlineSymbols,attr,omitempty"`
}

// CT_PageSetUpPr represents the pageSetUpPr element.
type CT_PageSetUpPr struct {
	AutoPageBreaks *bool `xml:"autoPageBreaks,attr,omitempty"`
	FitToPage      *bool `xml:"fitToPage,attr,omitempty"`
}

// CT_SheetDimension represents the dimension element.
type CT_SheetDimension struct {
	Ref string `xml:"ref,attr"`
}

// CT_Color represents a color element with theme, indexed, RGB, or tint.
//
// Tint is a FloatLex rather than a plain float64 because Excel writes theme
// tints in E-notation ("-4.9989318521683403E-2"). A plain float64 would be
// reprinted as "-0.049989318521683403" — numerically identical, textually
// different, and styles.xml is regenerated whenever any style is added. No
// formatting rule recovers the producer's spelling from the number, so the
// lexical form has to live in the value's type (C556).
type CT_Color struct {
	Auto    *bool     `xml:"auto,attr,omitempty"`
	Indexed *uint32   `xml:"indexed,attr,omitempty"`
	Rgb     string    `xml:"rgb,attr,omitempty"`
	Theme   *uint32   `xml:"theme,attr,omitempty"`
	Tint    *FloatLex `xml:"tint,attr,omitempty"`
}

// ---------------------------------------------------------------------------
// Sheet views
// ---------------------------------------------------------------------------

// CT_SheetViews represents the sheetViews element.
type CT_SheetViews struct {
	SheetView []CT_SheetView `xml:"sheetView"`
	// CapturedChildren records the source child sequence so the extLst this
	// type does not model survives a dirty save (C431's class).
	CapturedChildren *xmlb.ChildCapture `xml:"-"`
}

// sheetViewsModeledChildren maps CT_SheetViews's modeled child local names to
// their struct field indices.
var sheetViewsModeledChildren = map[string]int{
	"sheetView": structFieldIndex(reflect.TypeOf(CT_SheetViews{}), "SheetView"),
}

// UnmarshalXML decodes a sheetViews element, preserving its unmodeled extLst.
func (svs *CT_SheetViews) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type alias CT_SheetViews
	aux := struct {
		*alias
		Inner []byte `xml:",innerxml"`
	}{alias: (*alias)(svs)}
	if err := d.DecodeElement(&aux, &start); err != nil {
		return err
	}
	if cap := captureUnmodeledChildren(aux.Inner, sheetViewsModeledChildren); cap != nil {
		svs.CapturedChildren = cap
	}
	return nil
}

// CT_SheetView represents a sheetView element.
type CT_SheetView struct {
	WindowProtection         *bool          `xml:"windowProtection,attr,omitempty"`
	ShowFormulas             *bool          `xml:"showFormulas,attr,omitempty"`
	ShowGridLines            *bool          `xml:"showGridLines,attr,omitempty"`
	ShowRowColHeaders        *bool          `xml:"showRowColHeaders,attr,omitempty"`
	ShowZeros                *bool          `xml:"showZeros,attr,omitempty"`
	RightToLeft              *bool          `xml:"rightToLeft,attr,omitempty"`
	TabSelected              *bool          `xml:"tabSelected,attr,omitempty"`
	ShowRuler                *bool          `xml:"showRuler,attr,omitempty"`
	ShowOutlineSymbols       *bool          `xml:"showOutlineSymbols,attr,omitempty"`
	DefaultGridColor         *bool          `xml:"defaultGridColor,attr,omitempty"`
	ShowWhiteSpace           *bool          `xml:"showWhiteSpace,attr,omitempty"`
	View                     string         `xml:"view,attr,omitempty"`
	TopLeftCell              string         `xml:"topLeftCell,attr,omitempty"`
	ColorId                  *uint32        `xml:"colorId,attr,omitempty"`
	ZoomScale                *uint32        `xml:"zoomScale,attr,omitempty"`
	ZoomScaleNormal          *uint32        `xml:"zoomScaleNormal,attr,omitempty"`
	ZoomScaleSheetLayoutView *uint32        `xml:"zoomScaleSheetLayoutView,attr,omitempty"`
	ZoomScalePageLayoutView  *uint32        `xml:"zoomScalePageLayoutView,attr,omitempty"`
	WorkbookViewId           uint32         `xml:"workbookViewId,attr"`
	Pane                     *CT_Pane       `xml:"pane,omitempty"`
	Selection                []CT_Selection `xml:"selection"`
	// CapturedChildren records the source child sequence so children this type
	// does not model (pivotSelection, extLst) survive a dirty save; the
	// reflection marshaler replays it. nil for programmatic sheet views (C321).
	CapturedChildren *xmlb.ChildCapture `xml:"-"`
}

// sheetViewPaneField and sheetViewSelectionField are the struct field indices
// of CT_SheetView's typed children, used to record their position in the
// captured child order.
var (
	sheetViewPaneField      = structFieldIndex(reflect.TypeOf(CT_SheetView{}), "Pane")
	sheetViewSelectionField = structFieldIndex(reflect.TypeOf(CT_SheetView{}), "Selection")
)

// structFieldIndex returns the top-level field index of the named field, or -1.
func structFieldIndex(t reflect.Type, name string) int {
	if f, ok := t.FieldByName(name); ok && len(f.Index) == 1 {
		return f.Index[0]
	}
	return -1
}

// UnmarshalXML decodes a sheetView, preserving children this type does not
// model (pivotSelection, extLst). Attributes, pane and selection are decoded
// by reflection through an alias; the verbatim inner XML is then re-scanned to
// capture the unmodeled children as raw bytes at their source positions so a
// dirty save re-emits them (C321).
func (sv *CT_SheetView) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type alias CT_SheetView
	aux := struct {
		*alias
		Inner []byte `xml:",innerxml"`
	}{alias: (*alias)(sv)}
	if err := d.DecodeElement(&aux, &start); err != nil {
		return err
	}
	if len(aux.Inner) == 0 {
		return nil
	}

	cap := &xmlb.ChildCapture{}
	sub := xml.NewDecoder(bytes.NewReader(aux.Inner))
	selIdx := 0
	for {
		pre := sub.InputOffset()
		tok, err := sub.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// The inner bytes already parsed once (DecodeElement above); a
			// re-scan failure just means no capture, never a load failure.
			return nil
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch se.Name.Local {
		case "pane":
			if err := sub.Skip(); err != nil {
				return nil
			}
			if sheetViewPaneField >= 0 {
				cap.Order = append(cap.Order, xmlb.ChildRef{Field: sheetViewPaneField, Index: 0})
			}
		case "selection":
			if err := sub.Skip(); err != nil {
				return nil
			}
			if sheetViewSelectionField >= 0 {
				cap.Order = append(cap.Order, xmlb.ChildRef{Field: sheetViewSelectionField, Index: selIdx})
			}
			selIdx++
		default:
			if err := sub.Skip(); err != nil {
				return nil
			}
			post := sub.InputOffset()
			if pre >= 0 && post <= int64(len(aux.Inner)) && pre < post {
				cap.Order = append(cap.Order, xmlb.ChildRef{Field: -1, Index: len(cap.Raw)})
				cap.Raw = append(cap.Raw, append([]byte(nil), aux.Inner[pre:post]...))
			}
		}
	}
	// Only retain a capture that carries unmodeled children; a sheet view with
	// just pane/selection marshals identically through the normal field path.
	if len(cap.Raw) > 0 {
		sv.CapturedChildren = cap
	}
	return nil
}

// CT_Pane represents the pane element.
type CT_Pane struct {
	XSplit      *float64 `xml:"xSplit,attr,omitempty"`
	YSplit      *float64 `xml:"ySplit,attr,omitempty"`
	TopLeftCell string   `xml:"topLeftCell,attr,omitempty"`
	ActivePane  string   `xml:"activePane,attr,omitempty"`
	State       string   `xml:"state,attr,omitempty"`
}

// CT_Selection represents the selection element.
type CT_Selection struct {
	Pane         string  `xml:"pane,attr,omitempty"`
	ActiveCell   string  `xml:"activeCell,attr,omitempty"`
	ActiveCellId *uint32 `xml:"activeCellId,attr,omitempty"`
	SqRef        string  `xml:"sqref,attr,omitempty"`
}

// ---------------------------------------------------------------------------
// Sheet format
// ---------------------------------------------------------------------------

// CT_SheetFormatPr represents the sheetFormatPr element.
// It has a custom x14ac:dyDescent attribute that requires custom marshal/unmarshal.
type CT_SheetFormatPr struct {
	BaseColWidth     *uint32  `xml:"-"`
	DefaultColWidth  *float64 `xml:"-"`
	DefaultRowHeight float64  `xml:"-"`
	CustomHeight     *bool    `xml:"-"`
	ZeroHeight       *bool    `xml:"-"`
	ThickTop         *bool    `xml:"-"`
	ThickBottom      *bool    `xml:"-"`
	OutlineLevelRow  *uint8   `xml:"-"`
	OutlineLevelCol  *uint8   `xml:"-"`
	DyDescent        *float64 `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_SheetFormatPr.
func (sf *CT_SheetFormatPr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "baseColWidth" && attr.Name.Space == "":
			sf.BaseColWidth = parseUintPtr(attr.Value)
		case attr.Name.Local == "defaultColWidth" && attr.Name.Space == "":
			sf.DefaultColWidth = parseFloatPtr(attr.Value)
		case attr.Name.Local == "defaultRowHeight" && attr.Name.Space == "":
			if v := parseFloatPtr(attr.Value); v != nil {
				sf.DefaultRowHeight = *v
			}
		case attr.Name.Local == "customHeight" && attr.Name.Space == "":
			b := attr.Value == "1" || attr.Value == "true"
			sf.CustomHeight = &b
		case attr.Name.Local == "zeroHeight" && attr.Name.Space == "":
			b := attr.Value == "1" || attr.Value == "true"
			sf.ZeroHeight = &b
		case attr.Name.Local == "thickTop" && attr.Name.Space == "":
			b := attr.Value == "1" || attr.Value == "true"
			sf.ThickTop = &b
		case attr.Name.Local == "thickBottom" && attr.Name.Space == "":
			b := attr.Value == "1" || attr.Value == "true"
			sf.ThickBottom = &b
		case attr.Name.Local == "outlineLevelRow" && attr.Name.Space == "":
			sf.OutlineLevelRow = parseUint8Ptr(attr.Value)
		case attr.Name.Local == "outlineLevelCol" && attr.Name.Space == "":
			sf.OutlineLevelCol = parseUint8Ptr(attr.Value)
		case attr.Name.Local == "dyDescent" && (attr.Name.Space == nsX14AC || attr.Name.Space == "x14ac"):
			sf.DyDescent = parseFloatPtr(attr.Value)
		}
	}
	return d.Skip()
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_SheetFormatPr.
func (sf *CT_SheetFormatPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if sf.BaseColWidth != nil {
		attrs = append(attrs, xmlb.UintAttr("baseColWidth", *sf.BaseColWidth))
	}
	if sf.DefaultColWidth != nil {
		attrs = append(attrs, xmlb.Attr{Name: "defaultColWidth", Value: strconv.FormatFloat(*sf.DefaultColWidth, 'f', -1, 64)})
	}
	attrs = append(attrs, xmlb.Attr{Name: "defaultRowHeight", Value: strconv.FormatFloat(sf.DefaultRowHeight, 'f', -1, 64)})
	if sf.CustomHeight != nil {
		attrs = append(attrs, xmlb.BoolAttr("customHeight", *sf.CustomHeight))
	}
	if sf.ZeroHeight != nil {
		attrs = append(attrs, xmlb.BoolAttr("zeroHeight", *sf.ZeroHeight))
	}
	if sf.ThickTop != nil {
		attrs = append(attrs, xmlb.BoolAttr("thickTop", *sf.ThickTop))
	}
	if sf.ThickBottom != nil {
		attrs = append(attrs, xmlb.BoolAttr("thickBottom", *sf.ThickBottom))
	}
	if sf.OutlineLevelRow != nil {
		attrs = append(attrs, xmlb.Attr{Name: "outlineLevelRow", Value: strconv.FormatUint(uint64(*sf.OutlineLevelRow), 10)})
	}
	if sf.OutlineLevelCol != nil {
		attrs = append(attrs, xmlb.Attr{Name: "outlineLevelCol", Value: strconv.FormatUint(uint64(*sf.OutlineLevelCol), 10)})
	}
	if sf.DyDescent != nil {
		attrs = append(attrs, xmlb.Attr{
			Namespace: nsX14AC,
			Name:      "dyDescent",
			Value:     strconv.FormatFloat(*sf.DyDescent, 'f', -1, 64),
		})
	}
	b.EmptyElement(ns, localName, attrs...)
}

// ---------------------------------------------------------------------------
// Columns
// ---------------------------------------------------------------------------

// CT_Cols represents the cols element.
type CT_Cols struct {
	Col []CT_Col `xml:"col"`
}

// CT_Col represents a col element.
type CT_Col struct {
	Min          uint32   `xml:"min,attr"`
	Max          uint32   `xml:"max,attr"`
	Width        *float64 `xml:"width,attr,omitempty"`
	Style        *uint32  `xml:"style,attr,omitempty"`
	Hidden       *bool    `xml:"hidden,attr,omitempty"`
	BestFit      *bool    `xml:"bestFit,attr,omitempty"`
	CustomWidth  *bool    `xml:"customWidth,attr,omitempty"`
	Phonetic     *bool    `xml:"phonetic,attr,omitempty"`
	OutlineLevel *uint8   `xml:"outlineLevel,attr,omitempty"`
	Collapsed    *bool    `xml:"collapsed,attr,omitempty"`
}

// ---------------------------------------------------------------------------
// Sheet data (rows and cells)
// ---------------------------------------------------------------------------

// CT_SheetData represents the sheetData element.
type CT_SheetData struct {
	Row []CT_Row `xml:"row"`
}

// UnmarshalXML implements custom unmarshaling for CT_SheetData.
// Delegates to CT_Row custom unmarshal for x14ac:dyDescent support.
func (sd *CT_SheetData) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "row" {
				var row CT_Row
				if err := row.UnmarshalXML(d, t); err != nil {
					return err
				}
				sd.Row = append(sd.Row, row)
			} else {
				if err := d.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			return nil
		}
	}
}

// CT_Row represents a row element.
// It has a custom x14ac:dyDescent attribute that requires custom marshal/unmarshal.
type CT_Row struct {
	R     *uint32 `xml:"-"`
	Spans string  `xml:"-"`
	S     *uint32 `xml:"-"`
	// CustomFormat records that the row's s style index was applied by the user
	// rather than inherited; Excel drops the row formatting when it is missing.
	CustomFormat *bool    `xml:"-"`
	Ht           *float64 `xml:"-"`
	Hidden       *bool    `xml:"-"`
	CustomHeight *bool    `xml:"-"`
	OutlineLevel *uint8   `xml:"-"`
	Collapsed    *bool    `xml:"-"`
	ThickTop     *bool    `xml:"-"`
	ThickBot     *bool    `xml:"-"`
	Ph           *bool    `xml:"-"`
	// C holds pointers so that a *Cell handle obtained from the public API
	// stays valid when later cells are appended to the same row (appending a
	// value slice would reallocate its backing array and detach prior
	// handles).
	C         []*CT_Cell `xml:"-"`
	DyDescent *float64   `xml:"-"`
	// ExtRaw holds the verbatim bytes of children this type does not model
	// (extLst, which follows the cells in schema order), so a dirty save
	// re-emits them instead of dropping them. Captured lazily: a row with only
	// c children allocates nothing.
	ExtRaw [][]byte `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_Row.
func (r *CT_Row) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "r" && attr.Name.Space == "":
			// Assign only when the reference parses: an unparsable r="abc"
			// previously left v==0, emitting a schema-invalid r="0".
			r.R = parseUintPtr(attr.Value)
		case attr.Name.Local == "spans" && attr.Name.Space == "":
			r.Spans = attr.Value
		case attr.Name.Local == "s" && attr.Name.Space == "":
			r.S = parseUintPtr(attr.Value)
		case attr.Name.Local == "customFormat" && attr.Name.Space == "":
			b := attr.Value == "1" || attr.Value == "true"
			r.CustomFormat = &b
		case attr.Name.Local == "ht" && attr.Name.Space == "":
			r.Ht = parseFloatPtr(attr.Value)
		case attr.Name.Local == "hidden" && attr.Name.Space == "":
			b := attr.Value == "1" || attr.Value == "true"
			r.Hidden = &b
		case attr.Name.Local == "customHeight" && attr.Name.Space == "":
			b := attr.Value == "1" || attr.Value == "true"
			r.CustomHeight = &b
		case attr.Name.Local == "outlineLevel" && attr.Name.Space == "":
			r.OutlineLevel = parseUint8Ptr(attr.Value)
		case attr.Name.Local == "collapsed" && attr.Name.Space == "":
			b := attr.Value == "1" || attr.Value == "true"
			r.Collapsed = &b
		case attr.Name.Local == "thickTop" && attr.Name.Space == "":
			b := attr.Value == "1" || attr.Value == "true"
			r.ThickTop = &b
		case attr.Name.Local == "thickBot" && attr.Name.Space == "":
			b := attr.Value == "1" || attr.Value == "true"
			r.ThickBot = &b
		case attr.Name.Local == "ph" && attr.Name.Space == "":
			b := attr.Value == "1" || attr.Value == "true"
			r.Ph = &b
		case attr.Name.Local == "dyDescent" && (attr.Name.Space == nsX14AC || attr.Name.Space == "x14ac"):
			r.DyDescent = parseFloatPtr(attr.Value)
		}
	}

	// Decode child elements. Walking the tokens (rather than decoding into a
	// helper struct with an innerxml field) keeps the common path allocation-free
	// on what is the library's hottest element, while still preserving the
	// unmodeled children a helper struct would silently drop.
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "c" {
				cell := &CT_Cell{}
				if err := d.DecodeElement(cell, &t); err != nil {
					return err
				}
				r.C = append(r.C, cell)
				continue
			}
			var raw struct {
				Content []byte `xml:",innerxml"`
			}
			if err := d.DecodeElement(&raw, &t); err != nil {
				return err
			}
			r.ExtRaw = append(r.ExtRaw, encodeUnknownElement(t, raw.Content, nil))
		case xml.EndElement:
			return nil
		}
	}
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Row.
func (r *CT_Row) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if r.R != nil {
		attrs = append(attrs, xmlb.UintAttr("r", *r.R))
	}
	if r.Spans != "" {
		attrs = append(attrs, xmlb.StrAttr("spans", r.Spans))
	}
	if r.S != nil {
		attrs = append(attrs, xmlb.UintAttr("s", *r.S))
	}
	if r.CustomFormat != nil {
		attrs = append(attrs, xmlb.BoolAttr("customFormat", *r.CustomFormat))
	}
	if r.Ht != nil {
		attrs = append(attrs, xmlb.Attr{Name: "ht", Value: strconv.FormatFloat(*r.Ht, 'f', -1, 64)})
	}
	if r.Hidden != nil {
		attrs = append(attrs, xmlb.BoolAttr("hidden", *r.Hidden))
	}
	if r.CustomHeight != nil {
		attrs = append(attrs, xmlb.BoolAttr("customHeight", *r.CustomHeight))
	}
	if r.OutlineLevel != nil {
		attrs = append(attrs, xmlb.Attr{Name: "outlineLevel", Value: strconv.FormatUint(uint64(*r.OutlineLevel), 10)})
	}
	if r.Collapsed != nil {
		attrs = append(attrs, xmlb.BoolAttr("collapsed", *r.Collapsed))
	}
	if r.ThickTop != nil {
		attrs = append(attrs, xmlb.BoolAttr("thickTop", *r.ThickTop))
	}
	if r.ThickBot != nil {
		attrs = append(attrs, xmlb.BoolAttr("thickBot", *r.ThickBot))
	}
	if r.Ph != nil {
		attrs = append(attrs, xmlb.BoolAttr("ph", *r.Ph))
	}
	if r.DyDescent != nil {
		attrs = append(attrs, xmlb.Attr{
			Namespace: nsX14AC,
			Name:      "dyDescent",
			Value:     strconv.FormatFloat(*r.DyDescent, 'f', -1, 64),
		})
	}

	if len(r.C) == 0 && len(r.ExtRaw) == 0 {
		b.EmptyElement(ns, localName, attrs...)
		return
	}

	sortCellsByColumn(r.C)

	b.StartElement(ns, localName, attrs...)
	for i := range r.C {
		r.C[i].MarshalToBuilder(b, ns, "c")
	}
	// extLst follows the cells in schema order.
	for _, raw := range r.ExtRaw {
		b.WriteRaw(raw)
	}
	b.EndElement(ns, localName)
}

// sortCellsByColumn orders a row's cells by ascending column, which OOXML
// requires, without disturbing cells whose column cannot be derived.
//
// c/@r is optional: a cell that omits it implicitly occupies the column after
// its predecessor (the same legality rowNumberOf handles for rows). Keying such
// a cell to column 0 sorted every implicit cell ahead of every explicit one and,
// with an unstable sort, shuffled them among themselves — so a row written as
// <c r="A1">11</c><c>22</c><c>33</c> came back as 22, 33, 11 with the values
// under the wrong columns (C368). Each cell's key is therefore its own derived
// column when it has one and one past its predecessor's otherwise, which is
// exactly the position the reader assigns it; the sort is stable so cells that
// still tie keep their source order. Rows already in ascending order — the
// overwhelmingly common case — are left untouched.
func sortCellsByColumn(cells []*CT_Cell) {
	if len(cells) < 2 {
		return
	}
	keys := make([]int, len(cells))
	prev := 0
	ordered := true
	for i, c := range cells {
		col := cellRefColIndex(c.R)
		if col == 0 {
			col = prev + 1
		}
		keys[i] = col
		if i > 0 && col < keys[i-1] {
			ordered = false
		}
		prev = col
	}
	if ordered {
		return
	}
	idx := make([]int, len(cells))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return keys[idx[a]] < keys[idx[b]] })
	sorted := make([]*CT_Cell, len(cells))
	for i, j := range idx {
		sorted[i] = cells[j]
	}
	copy(cells, sorted)
}

// cellRefColIndex extracts the column index from a cell reference like "A1", "BC42".
// Returns 0 on invalid input.
func cellRefColIndex(ref string) int {
	col := 0
	for _, ch := range ref {
		if ch >= 'A' && ch <= 'Z' {
			col = col*26 + int(ch-'A') + 1
		} else if ch >= 'a' && ch <= 'z' {
			col = col*26 + int(ch-'a') + 1
		} else {
			break
		}
	}
	return col
}

// CT_Cell represents a c (cell) element. Cm is the cell-metadata index (1-based
// into xl/metadata.xml <cellMetadata>): Excel links a dynamic-array (spill)
// master cell to its XLDAPR metadata record through it. Vm is the parallel
// value-metadata index, preserved so a cell that carried one round-trips.
type CT_Cell struct {
	R  string          `xml:"r,attr"`
	S  *uint32         `xml:"s,attr,omitempty"`
	T  string          `xml:"t,attr,omitempty"`
	Cm *uint32         `xml:"cm,attr,omitempty"`
	Vm *uint32         `xml:"vm,attr,omitempty"`
	// Ph is the show-phonetic flag Excel sets on a cell in a Japanese
	// phonetic-guide workbook; it is the last CT_Cell attribute in schema order
	// (r, s, t, cm, vm, ph). Captured so such a cell round-trips rather than
	// silently losing its phonetic marking on a dirty save.
	Ph *bool           `xml:"ph,attr,omitempty"`
	F  *CT_CellFormula `xml:"f,omitempty"`
	V  *string         `xml:"v,omitempty"`
	Is *CT_Rst         `xml:"is,omitempty"`
	// ExtRaw holds the verbatim bytes of children this type does not model
	// (extLst, last in schema order), so a dirty save re-emits them. Captured
	// lazily: an ordinary cell allocates nothing for it.
	ExtRaw [][]byte `xml:"-"`
}

// UnmarshalXML decodes a cell, preserving children this type does not model
// (extLst) as verbatim bytes so a dirty save re-emits them rather than dropping
// them. It walks the tokens rather than decoding through an alias with an
// innerxml field: a cell is the library's most numerous element, and an
// innerxml capture would copy the inner bytes of every one of them.
func (c *CT_Cell) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Space != "" {
			continue
		}
		switch attr.Name.Local {
		case "r":
			c.R = attr.Value
		case "s":
			c.S = parseUintPtr(attr.Value)
		case "t":
			c.T = attr.Value
		case "cm":
			c.Cm = parseUintPtr(attr.Value)
		case "vm":
			c.Vm = parseUintPtr(attr.Value)
		case "ph":
			c.Ph = boolPtr(parseOnOff(attr.Value))
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "f":
				c.F = &CT_CellFormula{}
				if err := d.DecodeElement(c.F, &t); err != nil {
					return err
				}
			case "v":
				var v string
				if err := d.DecodeElement(&v, &t); err != nil {
					return err
				}
				c.V = &v
			case "is":
				c.Is = &CT_Rst{}
				if err := d.DecodeElement(c.Is, &t); err != nil {
					return err
				}
			default:
				var raw struct {
					Content []byte `xml:",innerxml"`
				}
				if err := d.DecodeElement(&raw, &t); err != nil {
					return err
				}
				c.ExtRaw = append(c.ExtRaw, encodeUnknownElement(t, raw.Content, nil))
			}
		case xml.EndElement:
			return nil
		}
	}
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Cell. Attributes are
// emitted in schema order (r, s, t, cm, vm, ph) so a metadata-linked or
// phonetic cell reparses identically.
func (c *CT_Cell) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if c.R != "" {
		// r is optional: a cell that omitted it in the source keeps it omitted
		// (its column is implied by position). Emitting r="" instead produced a
		// schema-invalid empty ST_CellRef (C368).
		attrs = append(attrs, xmlb.StrAttr("r", c.R))
	}
	if c.S != nil {
		attrs = append(attrs, xmlb.UintAttr("s", *c.S))
	}
	if c.T != "" {
		attrs = append(attrs, xmlb.StrAttr("t", c.T))
	}
	if c.Cm != nil {
		attrs = append(attrs, xmlb.UintAttr("cm", *c.Cm))
	}
	if c.Vm != nil {
		attrs = append(attrs, xmlb.UintAttr("vm", *c.Vm))
	}
	if c.Ph != nil {
		attrs = append(attrs, xmlb.BoolAttr("ph", *c.Ph))
	}

	if c.F == nil && c.V == nil && c.Is == nil && len(c.ExtRaw) == 0 {
		b.EmptyElement(ns, localName, attrs...)
		return
	}

	b.StartElement(ns, localName, attrs...)
	if c.F != nil {
		c.F.MarshalToBuilder(b, ns, "f")
	}
	if c.V != nil {
		b.WriteElement(ns, "v", *c.V)
	}
	if c.Is != nil {
		b.MarshalElement(ns, "is", c.Is)
	}
	// extLst is last in schema order.
	for _, raw := range c.ExtRaw {
		b.WriteRaw(raw)
	}
	b.EndElement(ns, localName)
}

// CT_CellFormula represents the f (formula) element. Aca (alwaysCalcArray) and
// Ca (calculateCell) are the flags Excel sets on a dynamic-array/spill formula
// (t="array"); they are captured so such a formula round-trips rather than
// silently losing its dynamic-array marking.
type CT_CellFormula struct {
	T     string  `xml:"-"`
	Aca   *bool   `xml:"-"`
	Ref   string  `xml:"-"`
	Ca    *bool   `xml:"-"`
	Si    *uint32 `xml:"-"`
	Value string  `xml:"-"`
	// CapturedAttrs preserves the verbatim source attribute list so a shared
	// or data-table formula round-trips the attributes this type has no field
	// for (dt2D, dtr, r1, r2, del1, del2, bx). Modeled attributes stay
	// authoritative on replay; nil for programmatic formulas.
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_CellFormula.
func (f *CT_CellFormula) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	f.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "t":
			f.T = attr.Value
		case "aca":
			f.Aca = boolPtr(parseOnOff(attr.Value))
		case "ref":
			f.Ref = attr.Value
		case "ca":
			f.Ca = boolPtr(parseOnOff(attr.Value))
		case "si":
			// Parse-or-skip: an unparsable si used to become si="0", which
			// silently moved the formula into shared-formula group 0 and merged
			// it with unrelated cells (C552). Leaving it unset omits the
			// attribute and lets the captured source form replay verbatim.
			f.Si = parseUintPtr(attr.Value)
		}
	}
	var content string
	if err := d.DecodeElement(&content, &start); err != nil {
		return err
	}
	f.Value = content
	return nil
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_CellFormula. The
// attributes are emitted in the schema order Excel uses (t, aca, ref, ca, si)
// so a dynamic-array formula reparses identically.
func (f *CT_CellFormula) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if f.T != "" {
		attrs = append(attrs, xmlb.StrAttr("t", f.T))
	}
	if f.Aca != nil {
		attrs = append(attrs, xmlb.BoolAttr("aca", *f.Aca))
	}
	if f.Ref != "" {
		attrs = append(attrs, xmlb.StrAttr("ref", f.Ref))
	}
	if f.Ca != nil {
		attrs = append(attrs, xmlb.BoolAttr("ca", *f.Ca))
	}
	if f.Si != nil {
		attrs = append(attrs, xmlb.UintAttr("si", *f.Si))
	}
	if f.CapturedAttrs != nil {
		// Replay unmodeled attributes (data-table r1/r2/dt2D/dtr/del1/del2/bx)
		// in source order while modeled values stay authoritative.
		attrs = b.ReplayCapturedAttrs(f.CapturedAttrs, attrs)
	}
	b.WriteElement(ns, localName, f.Value, attrs...)
}

// ---------------------------------------------------------------------------
// Merge cells
// ---------------------------------------------------------------------------

// CT_MergeCells represents the mergeCells element.
type CT_MergeCells struct {
	Count     *uint32        `xml:"count,attr,omitempty"`
	MergeCell []CT_MergeCell `xml:"mergeCell"`
}

// CT_MergeCell represents a mergeCell element.
type CT_MergeCell struct {
	Ref string `xml:"ref,attr"`
}

// ---------------------------------------------------------------------------
// Hyperlinks
// ---------------------------------------------------------------------------

// CT_Hyperlinks represents the hyperlinks element.
type CT_Hyperlinks struct {
	Hyperlink []CT_Hyperlink `xml:"hyperlink"`
}

// CT_Hyperlink represents a hyperlink element with r:id.
type CT_Hyperlink struct {
	Ref      string `xml:"-"`
	RID      string `xml:"-"`
	Location string `xml:"-"`
	Display  string `xml:"-"`
	Tooltip  string `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_Hyperlink.
func (h *CT_Hyperlink) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "ref":
			h.Ref = attr.Value
		case attr.Name.Local == "id" && attr.Name.Space == nsR:
			h.RID = attr.Value
		case attr.Name.Local == "location":
			h.Location = attr.Value
		case attr.Name.Local == "display":
			h.Display = attr.Value
		case attr.Name.Local == "tooltip":
			h.Tooltip = attr.Value
		}
	}
	return d.Skip()
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Hyperlink.
func (h *CT_Hyperlink) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	attrs = append(attrs, xmlb.StrAttr("ref", h.Ref))
	if h.RID != "" {
		attrs = append(attrs, xmlb.RelAttr("id", h.RID))
	}
	if h.Location != "" {
		attrs = append(attrs, xmlb.StrAttr("location", h.Location))
	}
	if h.Display != "" {
		attrs = append(attrs, xmlb.StrAttr("display", h.Display))
	}
	if h.Tooltip != "" {
		attrs = append(attrs, xmlb.StrAttr("tooltip", h.Tooltip))
	}
	b.EmptyElement(ns, localName, attrs...)
}

// ---------------------------------------------------------------------------
// Page layout
// ---------------------------------------------------------------------------

// CT_PageMargins represents the pageMargins element.
type CT_PageMargins struct {
	Left   float64 `xml:"left,attr"`
	Right  float64 `xml:"right,attr"`
	Top    float64 `xml:"top,attr"`
	Bottom float64 `xml:"bottom,attr"`
	Header float64 `xml:"header,attr"`
	Footer float64 `xml:"footer,attr"`
}

// CT_PageSetup represents the pageSetup element with r:id.
type CT_PageSetup struct {
	PaperSize *uint32 `xml:"-"`
	// PaperHeight and PaperWidth are ST_PositiveUniversalMeasure strings
	// ("210mm", "11in"): the custom paper size a producer writes instead of a
	// paperSize index. Kept as text so the source unit and precision replay
	// unchanged.
	PaperHeight        string  `xml:"-"`
	PaperWidth         string  `xml:"-"`
	Scale              *uint32 `xml:"-"`
	FirstPageNumber    *uint32 `xml:"-"`
	FitToWidth         *uint32 `xml:"-"`
	FitToHeight        *uint32 `xml:"-"`
	PageOrder          string  `xml:"-"`
	Orientation        string  `xml:"-"`
	UsePrinterDefaults *bool   `xml:"-"`
	BlackAndWhite      *bool   `xml:"-"`
	Draft              *bool   `xml:"-"`
	CellComments       string  `xml:"-"`
	UseFirstPageNumber *bool   `xml:"-"`
	Errors             string  `xml:"-"`
	HorizontalDpi      *uint32 `xml:"-"`
	VerticalDpi        *uint32 `xml:"-"`
	Copies             *uint32 `xml:"-"`
	RID                string  `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_PageSetup. Every numeric
// attribute is parse-or-skip: fmt.Sscanf previously assigned unconditionally,
// so paperSize="A4" became paperSize="0" — a real paper size, silently
// substituted (C552).
func (ps *CT_PageSetup) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "paperSize" && attr.Name.Space == "":
			ps.PaperSize = parseUintPtr(attr.Value)
		case attr.Name.Local == "paperHeight" && attr.Name.Space == "":
			ps.PaperHeight = attr.Value
		case attr.Name.Local == "paperWidth" && attr.Name.Space == "":
			ps.PaperWidth = attr.Value
		case attr.Name.Local == "scale" && attr.Name.Space == "":
			ps.Scale = parseUintPtr(attr.Value)
		case attr.Name.Local == "firstPageNumber" && attr.Name.Space == "":
			ps.FirstPageNumber = parseUintPtr(attr.Value)
		case attr.Name.Local == "fitToWidth" && attr.Name.Space == "":
			ps.FitToWidth = parseUintPtr(attr.Value)
		case attr.Name.Local == "fitToHeight" && attr.Name.Space == "":
			ps.FitToHeight = parseUintPtr(attr.Value)
		case attr.Name.Local == "pageOrder" && attr.Name.Space == "":
			ps.PageOrder = attr.Value
		case attr.Name.Local == "orientation" && attr.Name.Space == "":
			ps.Orientation = attr.Value
		case attr.Name.Local == "usePrinterDefaults" && attr.Name.Space == "":
			b := attr.Value == "1" || attr.Value == "true"
			ps.UsePrinterDefaults = &b
		case attr.Name.Local == "blackAndWhite" && attr.Name.Space == "":
			b := attr.Value == "1" || attr.Value == "true"
			ps.BlackAndWhite = &b
		case attr.Name.Local == "draft" && attr.Name.Space == "":
			b := attr.Value == "1" || attr.Value == "true"
			ps.Draft = &b
		case attr.Name.Local == "cellComments" && attr.Name.Space == "":
			ps.CellComments = attr.Value
		case attr.Name.Local == "useFirstPageNumber" && attr.Name.Space == "":
			b := attr.Value == "1" || attr.Value == "true"
			ps.UseFirstPageNumber = &b
		case attr.Name.Local == "errors" && attr.Name.Space == "":
			ps.Errors = attr.Value
		case attr.Name.Local == "horizontalDpi" && attr.Name.Space == "":
			ps.HorizontalDpi = parseUintPtr(attr.Value)
		case attr.Name.Local == "verticalDpi" && attr.Name.Space == "":
			ps.VerticalDpi = parseUintPtr(attr.Value)
		case attr.Name.Local == "copies" && attr.Name.Space == "":
			ps.Copies = parseUintPtr(attr.Value)
		case attr.Name.Local == "id" && attr.Name.Space == nsR:
			ps.RID = attr.Value
		}
	}
	return d.Skip()
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_PageSetup.
func (ps *CT_PageSetup) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if ps.PaperSize != nil {
		attrs = append(attrs, xmlb.UintAttr("paperSize", *ps.PaperSize))
	}
	if ps.PaperHeight != "" {
		attrs = append(attrs, xmlb.StrAttr("paperHeight", ps.PaperHeight))
	}
	if ps.PaperWidth != "" {
		attrs = append(attrs, xmlb.StrAttr("paperWidth", ps.PaperWidth))
	}
	if ps.Scale != nil {
		attrs = append(attrs, xmlb.UintAttr("scale", *ps.Scale))
	}
	if ps.FirstPageNumber != nil {
		attrs = append(attrs, xmlb.UintAttr("firstPageNumber", *ps.FirstPageNumber))
	}
	if ps.FitToWidth != nil {
		attrs = append(attrs, xmlb.UintAttr("fitToWidth", *ps.FitToWidth))
	}
	if ps.FitToHeight != nil {
		attrs = append(attrs, xmlb.UintAttr("fitToHeight", *ps.FitToHeight))
	}
	if ps.PageOrder != "" {
		attrs = append(attrs, xmlb.StrAttr("pageOrder", ps.PageOrder))
	}
	if ps.Orientation != "" {
		attrs = append(attrs, xmlb.StrAttr("orientation", ps.Orientation))
	}
	if ps.UsePrinterDefaults != nil {
		attrs = append(attrs, xmlb.BoolAttr("usePrinterDefaults", *ps.UsePrinterDefaults))
	}
	if ps.BlackAndWhite != nil {
		attrs = append(attrs, xmlb.BoolAttr("blackAndWhite", *ps.BlackAndWhite))
	}
	if ps.Draft != nil {
		attrs = append(attrs, xmlb.BoolAttr("draft", *ps.Draft))
	}
	if ps.CellComments != "" {
		attrs = append(attrs, xmlb.StrAttr("cellComments", ps.CellComments))
	}
	if ps.UseFirstPageNumber != nil {
		attrs = append(attrs, xmlb.BoolAttr("useFirstPageNumber", *ps.UseFirstPageNumber))
	}
	if ps.Errors != "" {
		attrs = append(attrs, xmlb.StrAttr("errors", ps.Errors))
	}
	if ps.HorizontalDpi != nil {
		attrs = append(attrs, xmlb.UintAttr("horizontalDpi", *ps.HorizontalDpi))
	}
	if ps.VerticalDpi != nil {
		attrs = append(attrs, xmlb.UintAttr("verticalDpi", *ps.VerticalDpi))
	}
	if ps.Copies != nil {
		attrs = append(attrs, xmlb.UintAttr("copies", *ps.Copies))
	}
	if ps.RID != "" {
		attrs = append(attrs, xmlb.RelAttr("id", ps.RID))
	}
	b.EmptyElement(ns, localName, attrs...)
}

// CT_HeaderFooter represents the headerFooter element.
type CT_HeaderFooter struct {
	DifferentOddEven *bool   `xml:"differentOddEven,attr,omitempty"`
	DifferentFirst   *bool   `xml:"differentFirst,attr,omitempty"`
	ScaleWithDoc     *bool   `xml:"scaleWithDoc,attr,omitempty"`
	AlignWithMargins *bool   `xml:"alignWithMargins,attr,omitempty"`
	OddHeader        *string `xml:"oddHeader,omitempty"`
	OddFooter        *string `xml:"oddFooter,omitempty"`
	EvenHeader       *string `xml:"evenHeader,omitempty"`
	EvenFooter       *string `xml:"evenFooter,omitempty"`
	FirstHeader      *string `xml:"firstHeader,omitempty"`
	FirstFooter      *string `xml:"firstFooter,omitempty"`
}

// CT_PrintOptions represents the printOptions element.
type CT_PrintOptions struct {
	HorizontalCentered *bool `xml:"horizontalCentered,attr,omitempty"`
	VerticalCentered   *bool `xml:"verticalCentered,attr,omitempty"`
	Headings           *bool `xml:"headings,attr,omitempty"`
	GridLines          *bool `xml:"gridLines,attr,omitempty"`
	GridLinesSet       *bool `xml:"gridLinesSet,attr,omitempty"`
}

// ---------------------------------------------------------------------------
// Drawing & legacy drawing (r:id types)
// ---------------------------------------------------------------------------

// CT_Drawing represents the drawing element with r:id.
type CT_Drawing struct {
	RID string `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_Drawing.
func (d *CT_Drawing) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Local == "id" && attr.Name.Space == nsR {
			d.RID = attr.Value
		}
	}
	return dec.Skip()
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Drawing.
func (d *CT_Drawing) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.EmptyElement(ns, localName, xmlb.RelAttr("id", d.RID))
}

// CT_LegacyDrawing represents the legacyDrawing element with r:id.
type CT_LegacyDrawing struct {
	RID string `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_LegacyDrawing.
func (ld *CT_LegacyDrawing) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Local == "id" && attr.Name.Space == nsR {
			ld.RID = attr.Value
		}
	}
	return dec.Skip()
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_LegacyDrawing.
func (ld *CT_LegacyDrawing) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.EmptyElement(ns, localName, xmlb.RelAttr("id", ld.RID))
}

// ---------------------------------------------------------------------------
// Table parts (r:id type)
// ---------------------------------------------------------------------------

// CT_TableParts represents the tableParts element.
type CT_TableParts struct {
	Count     *uint32        `xml:"count,attr,omitempty"`
	TablePart []CT_TablePart `xml:"tablePart"`
}

// CT_TablePart represents a tablePart element with r:id.
type CT_TablePart struct {
	RID string `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_TablePart.
func (tp *CT_TablePart) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Local == "id" && attr.Name.Space == nsR {
			tp.RID = attr.Value
		}
	}
	return d.Skip()
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_TablePart.
func (tp *CT_TablePart) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.EmptyElement(ns, localName, xmlb.RelAttr("id", tp.RID))
}

// ---------------------------------------------------------------------------
// Auto filter
// ---------------------------------------------------------------------------

// CT_AutoFilter represents the autoFilter element.
type CT_AutoFilter struct {
	Ref          string            `xml:"ref,attr,omitempty"`
	FilterColumn []CT_FilterColumn `xml:"filterColumn"`
	SortState    *CT_SortState     `xml:"sortState,omitempty"`
	// CapturedChildren records the source child sequence so the extLst this
	// type does not model survives a dirty save; the reflection marshaler
	// replays it. nil for programmatic filters (C431, C274's pattern applied to
	// the whole autoFilter subtree rather than only where a finding pointed).
	CapturedChildren *xmlb.ChildCapture `xml:"-"`
}

// autoFilterModeledChildren maps CT_AutoFilter's modeled child local names to
// their struct field indices.
var autoFilterModeledChildren = map[string]int{
	"filterColumn": structFieldIndex(reflect.TypeOf(CT_AutoFilter{}), "FilterColumn"),
	"sortState":    structFieldIndex(reflect.TypeOf(CT_AutoFilter{}), "SortState"),
}

// UnmarshalXML decodes an autoFilter, preserving its unmodeled extLst verbatim.
func (af *CT_AutoFilter) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type alias CT_AutoFilter
	aux := struct {
		*alias
		Inner []byte `xml:",innerxml"`
	}{alias: (*alias)(af)}
	if err := d.DecodeElement(&aux, &start); err != nil {
		return err
	}
	if cap := captureUnmodeledChildren(aux.Inner, autoFilterModeledChildren); cap != nil {
		af.CapturedChildren = cap
	}
	return nil
}

// captureUnmodeledChildren re-scans the verbatim inner bytes of an element,
// building a ChildCapture that records modeled children (by the field index
// their local name maps to in modeled) at their source positions and every
// other child as verbatim raw bytes, so a dirty save re-emits children the
// type does not model. A modeled name occurring more than once advances its
// own running slice index. It returns nil when the inner XML carries no
// unmodeled children — the element then marshals identically through the
// normal field path, preserving byte identity for currently-passing input.
// This mirrors CT_SheetView's C321 raw-capture (C274).
func captureUnmodeledChildren(inner []byte, modeled map[string]int) *xmlb.ChildCapture {
	if len(inner) == 0 {
		return nil
	}
	cap := &xmlb.ChildCapture{}
	sliceIdx := make(map[string]int)
	sub := xml.NewDecoder(bytes.NewReader(inner))
	for {
		pre := sub.InputOffset()
		tok, err := sub.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// The inner bytes already parsed once (DecodeElement); a re-scan
			// failure just means no capture, never a load failure.
			return nil
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if err := sub.Skip(); err != nil {
			return nil
		}
		if field, ok := modeled[se.Name.Local]; ok && field >= 0 {
			cap.Order = append(cap.Order, xmlb.ChildRef{Field: field, Index: sliceIdx[se.Name.Local]})
			sliceIdx[se.Name.Local]++
			continue
		}
		post := sub.InputOffset()
		if pre >= 0 && post <= int64(len(inner)) && pre < post {
			cap.Order = append(cap.Order, xmlb.ChildRef{Field: -1, Index: len(cap.Raw)})
			cap.Raw = append(cap.Raw, append([]byte(nil), inner[pre:post]...))
		}
	}
	// Only retain a capture that carries unmodeled children; an element with
	// only modeled children marshals identically through the normal field path.
	if len(cap.Raw) == 0 {
		return nil
	}
	return cap
}

// CT_FilterColumn represents a filterColumn element.
type CT_FilterColumn struct {
	ColId         uint32            `xml:"colId,attr"`
	HiddenButton  *bool             `xml:"hiddenButton,attr,omitempty"`
	ShowButton    *bool             `xml:"showButton,attr,omitempty"`
	Filters       *CT_Filters       `xml:"filters,omitempty"`
	CustomFilters *CT_CustomFilters `xml:"customFilters,omitempty"`
	// CapturedChildren records the source child sequence so filter kinds this
	// type does not model (top10, dynamicFilter, colorFilter, iconFilter,
	// extLst) survive a dirty save; the reflection marshaler replays it. nil
	// for programmatic filter columns (C274).
	CapturedChildren *xmlb.ChildCapture `xml:"-"`
}

// filterColumnModeledChildren maps CT_FilterColumn's modeled child local names
// to their struct field indices, recording their position in a captured child
// order alongside the unmodeled filter kinds.
var filterColumnModeledChildren = map[string]int{
	"filters":       structFieldIndex(reflect.TypeOf(CT_FilterColumn{}), "Filters"),
	"customFilters": structFieldIndex(reflect.TypeOf(CT_FilterColumn{}), "CustomFilters"),
}

// UnmarshalXML decodes a filterColumn, preserving the filter kinds this type
// does not model (top10, dynamicFilter, colorFilter, iconFilter, extLst) as
// verbatim raw bytes so a dirty save re-emits them (C274). Attributes and the
// modeled filters/customFilters children are decoded by reflection through an
// alias; the inner XML is then re-scanned to capture the unmodeled children.
func (fc *CT_FilterColumn) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type alias CT_FilterColumn
	aux := struct {
		*alias
		Inner []byte `xml:",innerxml"`
	}{alias: (*alias)(fc)}
	if err := d.DecodeElement(&aux, &start); err != nil {
		return err
	}
	if cap := captureUnmodeledChildren(aux.Inner, filterColumnModeledChildren); cap != nil {
		fc.CapturedChildren = cap
	}
	return nil
}

// CT_Filters represents the filters element.
type CT_Filters struct {
	Blank        *bool       `xml:"blank,attr,omitempty"`
	CalendarType string      `xml:"calendarType,attr,omitempty"`
	Filter       []CT_Filter `xml:"filter"`
	// CapturedChildren records the source child sequence so dateGroupItem
	// children this type does not model survive a dirty save; the reflection
	// marshaler replays it. nil for programmatic filters (C274).
	CapturedChildren *xmlb.ChildCapture `xml:"-"`
}

// filtersModeledChildren maps CT_Filters's modeled child local names to their
// struct field indices, recording their position in a captured child order
// alongside the unmodeled dateGroupItem children.
var filtersModeledChildren = map[string]int{
	"filter": structFieldIndex(reflect.TypeOf(CT_Filters{}), "Filter"),
}

// UnmarshalXML decodes a filters element, preserving dateGroupItem children
// this type does not model as verbatim raw bytes so a dirty save re-emits them
// (C274). Attributes (including calendarType) and the modeled filter children
// are decoded by reflection through an alias; the inner XML is then re-scanned
// to capture the unmodeled children.
func (f *CT_Filters) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type alias CT_Filters
	aux := struct {
		*alias
		Inner []byte `xml:",innerxml"`
	}{alias: (*alias)(f)}
	if err := d.DecodeElement(&aux, &start); err != nil {
		return err
	}
	if cap := captureUnmodeledChildren(aux.Inner, filtersModeledChildren); cap != nil {
		f.CapturedChildren = cap
	}
	return nil
}

// CT_Filter represents a filter element.
type CT_Filter struct {
	Val string `xml:"val,attr"`
}

// CT_CustomFilters represents the customFilters element.
type CT_CustomFilters struct {
	And          *bool             `xml:"and,attr,omitempty"`
	CustomFilter []CT_CustomFilter `xml:"customFilter"`
}

// CT_CustomFilter represents a customFilter element.
type CT_CustomFilter struct {
	Operator string `xml:"operator,attr,omitempty"`
	Val      string `xml:"val,attr"`
}

// ---------------------------------------------------------------------------
// Sort state
// ---------------------------------------------------------------------------

// CT_SortState represents the sortState element.
type CT_SortState struct {
	ColumnSort    *bool              `xml:"columnSort,attr,omitempty"`
	CaseSensitive *bool              `xml:"caseSensitive,attr,omitempty"`
	SortMethod    string             `xml:"sortMethod,attr,omitempty"`
	Ref           string             `xml:"ref,attr"`
	SortCondition []CT_SortCondition `xml:"sortCondition"`
	// CapturedChildren records the source child sequence so the extLst this
	// type does not model (it carries the x14 sort-by-color conditions)
	// survives a dirty save; the reflection marshaler replays it (C431).
	CapturedChildren *xmlb.ChildCapture `xml:"-"`
}

// sortStateModeledChildren maps CT_SortState's modeled child local names to
// their struct field indices.
var sortStateModeledChildren = map[string]int{
	"sortCondition": structFieldIndex(reflect.TypeOf(CT_SortState{}), "SortCondition"),
}

// UnmarshalXML decodes a sortState, preserving its unmodeled extLst verbatim.
func (ss *CT_SortState) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type alias CT_SortState
	aux := struct {
		*alias
		Inner []byte `xml:",innerxml"`
	}{alias: (*alias)(ss)}
	if err := d.DecodeElement(&aux, &start); err != nil {
		return err
	}
	if cap := captureUnmodeledChildren(aux.Inner, sortStateModeledChildren); cap != nil {
		ss.CapturedChildren = cap
	}
	return nil
}

// CT_SortCondition represents a sortCondition element. DxfId, IconSet and
// IconId carry sort-by-color and sort-by-icon, which the model previously
// dropped so a colour sort degraded to a value sort on any sheet edit.
type CT_SortCondition struct {
	Descending *bool   `xml:"descending,attr,omitempty"`
	SortBy     string  `xml:"sortBy,attr,omitempty"`
	Ref        string  `xml:"ref,attr"`
	CustomList string  `xml:"customList,attr,omitempty"`
	DxfId      *uint32 `xml:"dxfId,attr,omitempty"`
	IconSet    string  `xml:"iconSet,attr,omitempty"`
	IconId     *uint32 `xml:"iconId,attr,omitempty"`
}

// ---------------------------------------------------------------------------
// Conditional formatting
// ---------------------------------------------------------------------------

// CT_ConditionalFormatting represents the conditionalFormatting element.
type CT_ConditionalFormatting struct {
	Sqref  string      `xml:"sqref,attr"`
	Pivot  *bool       `xml:"pivot,attr,omitempty"`
	CfRule []CT_CfRule `xml:"cfRule"`
	// CapturedChildren records the source child sequence so the extLst this
	// type does not model survives a dirty save; the reflection marshaler
	// replays it. The C274 capture reached the cfRule children but not this
	// parent, so the block's own extension list was still dropped (C431).
	CapturedChildren *xmlb.ChildCapture `xml:"-"`
}

// conditionalFormattingModeledChildren maps CT_ConditionalFormatting's modeled
// child local names to their struct field indices.
var conditionalFormattingModeledChildren = map[string]int{
	"cfRule": structFieldIndex(reflect.TypeOf(CT_ConditionalFormatting{}), "CfRule"),
}

// UnmarshalXML decodes a conditionalFormatting block, preserving its unmodeled
// extLst verbatim.
func (cf *CT_ConditionalFormatting) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type alias CT_ConditionalFormatting
	aux := struct {
		*alias
		Inner []byte `xml:",innerxml"`
	}{alias: (*alias)(cf)}
	if err := d.DecodeElement(&aux, &start); err != nil {
		return err
	}
	if cap := captureUnmodeledChildren(aux.Inner, conditionalFormattingModeledChildren); cap != nil {
		cf.CapturedChildren = cap
	}
	return nil
}

// CT_CfRule represents a cfRule element.
type CT_CfRule struct {
	Type         string         `xml:"type,attr,omitempty"`
	DxfId        *uint32        `xml:"dxfId,attr,omitempty"`
	Priority     int32          `xml:"priority,attr"`
	StopIfTrue   *bool          `xml:"stopIfTrue,attr,omitempty"`
	AboveAverage *bool          `xml:"aboveAverage,attr,omitempty"`
	Percent      *bool          `xml:"percent,attr,omitempty"`
	Bottom       *bool          `xml:"bottom,attr,omitempty"`
	Operator     string         `xml:"operator,attr,omitempty"`
	Text         string         `xml:"text,attr,omitempty"`
	TimePeriod   string         `xml:"timePeriod,attr,omitempty"`
	Rank         *uint32        `xml:"rank,attr,omitempty"`
	StdDev       *int32         `xml:"stdDev,attr,omitempty"`
	EqualAverage *bool          `xml:"equalAverage,attr,omitempty"`
	Formula      []string       `xml:"formula"`
	ColorScale   *CT_ColorScale `xml:"colorScale,omitempty"`
	DataBar      *CT_DataBar    `xml:"dataBar,omitempty"`
	IconSet      *CT_IconSet    `xml:"iconSet,omitempty"`
	// CapturedChildren records the source child sequence so the extLst this
	// type does not model survives a dirty save; the reflection marshaler
	// replays it. extLst carries the x14:id linking a 2010+ rule to its x14
	// counterpart (dataBars), so dropping it severs the pairing. nil for
	// programmatic rules (C274).
	CapturedChildren *xmlb.ChildCapture `xml:"-"`
}

// cfRuleModeledChildren maps CT_CfRule's modeled child local names to their
// struct field indices, recording their position in a captured child order
// alongside the unmodeled extLst.
var cfRuleModeledChildren = map[string]int{
	"formula":    structFieldIndex(reflect.TypeOf(CT_CfRule{}), "Formula"),
	"colorScale": structFieldIndex(reflect.TypeOf(CT_CfRule{}), "ColorScale"),
	"dataBar":    structFieldIndex(reflect.TypeOf(CT_CfRule{}), "DataBar"),
	"iconSet":    structFieldIndex(reflect.TypeOf(CT_CfRule{}), "IconSet"),
}

// UnmarshalXML decodes a cfRule, preserving the extLst this type does not model
// (which carries the x14:id pairing a 2010+ conditional-format rule to its x14
// counterpart) as verbatim raw bytes so a dirty save re-emits it (C274).
// Attributes and the modeled formula/colorScale/dataBar/iconSet children are
// decoded by reflection through an alias; the inner XML is then re-scanned to
// capture the unmodeled extLst.
func (r *CT_CfRule) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type alias CT_CfRule
	aux := struct {
		*alias
		Inner []byte `xml:",innerxml"`
	}{alias: (*alias)(r)}
	if err := d.DecodeElement(&aux, &start); err != nil {
		return err
	}
	if cap := captureUnmodeledChildren(aux.Inner, cfRuleModeledChildren); cap != nil {
		r.CapturedChildren = cap
	}
	return nil
}

// CT_ColorScale represents the colorScale element.
type CT_ColorScale struct {
	Cfvo  []CT_Cfvo  `xml:"cfvo"`
	Color []CT_Color `xml:"color"`
}

// CT_DataBar represents the dataBar element.
type CT_DataBar struct {
	MinLength *uint32   `xml:"minLength,attr,omitempty"`
	MaxLength *uint32   `xml:"maxLength,attr,omitempty"`
	ShowValue *bool     `xml:"showValue,attr,omitempty"`
	Cfvo      []CT_Cfvo `xml:"cfvo"`
	Color     *CT_Color `xml:"color,omitempty"`
}

// CT_IconSet represents the iconSet element.
type CT_IconSet struct {
	IconSet   string    `xml:"iconSet,attr,omitempty"`
	ShowValue *bool     `xml:"showValue,attr,omitempty"`
	Percent   *bool     `xml:"percent,attr,omitempty"`
	Reverse   *bool     `xml:"reverse,attr,omitempty"`
	Cfvo      []CT_Cfvo `xml:"cfvo"`
}

// CT_Cfvo represents a cfvo (conditional format value object) element. Gte
// selects >= versus > for the threshold; dropping it silently flipped an
// exclusive icon-set boundary to inclusive.
type CT_Cfvo struct {
	Type string `xml:"type,attr"`
	Val  string `xml:"val,attr,omitempty"`
	Gte  *bool  `xml:"gte,attr,omitempty"`
	// CapturedChildren records the source child sequence so the extLst this
	// type does not model survives a dirty save (C431's class).
	CapturedChildren *xmlb.ChildCapture `xml:"-"`
}

// UnmarshalXML decodes a cfvo, preserving its unmodeled extLst verbatim.
// CT_Cfvo models no child elements, so every child it carries is captured.
func (c *CT_Cfvo) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	type alias CT_Cfvo
	aux := struct {
		*alias
		Inner []byte `xml:",innerxml"`
	}{alias: (*alias)(c)}
	if err := d.DecodeElement(&aux, &start); err != nil {
		return err
	}
	if cap := captureUnmodeledChildren(aux.Inner, nil); cap != nil {
		c.CapturedChildren = cap
	}
	return nil
}

// ---------------------------------------------------------------------------
// Data validations
// ---------------------------------------------------------------------------

// CT_DataValidations represents the dataValidations element.
type CT_DataValidations struct {
	DisablePrompts *bool `xml:"disablePrompts,attr,omitempty"`
	// XWindow and YWindow are the saved screen position of the validation
	// dialog; Excel writes them and they are lost on any sheet regeneration
	// when unmodeled.
	XWindow        *uint32             `xml:"xWindow,attr,omitempty"`
	YWindow        *uint32             `xml:"yWindow,attr,omitempty"`
	Count          *uint32             `xml:"count,attr,omitempty"`
	DataValidation []CT_DataValidation `xml:"dataValidation"`
}

// CT_DataValidation represents a dataValidation element.
type CT_DataValidation struct {
	Type             string  `xml:"type,attr,omitempty"`
	ErrorStyle       string  `xml:"errorStyle,attr,omitempty"`
	ImeMode          string  `xml:"imeMode,attr,omitempty"`
	Operator         string  `xml:"operator,attr,omitempty"`
	AllowBlank       *bool   `xml:"allowBlank,attr,omitempty"`
	ShowDropDown     *bool   `xml:"showDropDown,attr,omitempty"`
	ShowInputMessage *bool   `xml:"showInputMessage,attr,omitempty"`
	ShowErrorMessage *bool   `xml:"showErrorMessage,attr,omitempty"`
	ErrorTitle       string  `xml:"errorTitle,attr,omitempty"`
	Error            string  `xml:"error,attr,omitempty"`
	PromptTitle      string  `xml:"promptTitle,attr,omitempty"`
	Prompt           string  `xml:"prompt,attr,omitempty"`
	Sqref            string  `xml:"sqref,attr"`
	Formula1         *string `xml:"formula1,omitempty"`
	Formula2         *string `xml:"formula2,omitempty"`
}

// ---------------------------------------------------------------------------
// Protection & calc
// ---------------------------------------------------------------------------

// CT_SheetProtection represents the sheetProtection element.
type CT_SheetProtection struct {
	Sheet               *bool   `xml:"sheet,attr,omitempty"`
	Objects             *bool   `xml:"objects,attr,omitempty"`
	Scenarios           *bool   `xml:"scenarios,attr,omitempty"`
	FormatCells         *bool   `xml:"formatCells,attr,omitempty"`
	FormatColumns       *bool   `xml:"formatColumns,attr,omitempty"`
	FormatRows          *bool   `xml:"formatRows,attr,omitempty"`
	InsertColumns       *bool   `xml:"insertColumns,attr,omitempty"`
	InsertRows          *bool   `xml:"insertRows,attr,omitempty"`
	InsertHyperlinks    *bool   `xml:"insertHyperlinks,attr,omitempty"`
	DeleteColumns       *bool   `xml:"deleteColumns,attr,omitempty"`
	DeleteRows          *bool   `xml:"deleteRows,attr,omitempty"`
	SelectLockedCells   *bool   `xml:"selectLockedCells,attr,omitempty"`
	Sort                *bool   `xml:"sort,attr,omitempty"`
	AutoFilter          *bool   `xml:"autoFilter,attr,omitempty"`
	PivotTables         *bool   `xml:"pivotTables,attr,omitempty"`
	SelectUnlockedCells *bool   `xml:"selectUnlockedCells,attr,omitempty"`
	Password            string  `xml:"password,attr,omitempty"`
	AlgorithmName       string  `xml:"algorithmName,attr,omitempty"`
	HashValue           string  `xml:"hashValue,attr,omitempty"`
	SaltValue           string  `xml:"saltValue,attr,omitempty"`
	SpinCount           *uint32 `xml:"spinCount,attr,omitempty"`
}

// CT_SheetCalcPr represents the sheetCalcPr element.
type CT_SheetCalcPr struct {
	FullCalcOnLoad *bool `xml:"fullCalcOnLoad,attr,omitempty"`
}

// ---------------------------------------------------------------------------
// Page breaks
// ---------------------------------------------------------------------------

// CT_PageBreak represents the rowBreaks or colBreaks element.
type CT_PageBreak struct {
	Count            *uint32    `xml:"count,attr,omitempty"`
	ManualBreakCount *uint32    `xml:"manualBreakCount,attr,omitempty"`
	Break            []CT_Break `xml:"brk"`
}

// CT_Break represents a brk (break) element.
type CT_Break struct {
	Id  *uint32 `xml:"id,attr,omitempty"`
	Min *uint32 `xml:"min,attr,omitempty"`
	Max *uint32 `xml:"max,attr,omitempty"`
	Man *bool   `xml:"man,attr,omitempty"`
	Pt  *bool   `xml:"pt,attr,omitempty"`
}

// ---------------------------------------------------------------------------
// Phonetic properties
// ---------------------------------------------------------------------------

// CT_PhoneticPr represents the phoneticPr element.
type CT_PhoneticPr struct {
	FontId    uint32 `xml:"fontId,attr"`
	Type      string `xml:"type,attr,omitempty"`
	Alignment string `xml:"alignment,attr,omitempty"`
}
