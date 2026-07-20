package oxml

import (
	"encoding/xml"
	"fmt"
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
	AutoFilter            *CT_AutoFilter             `xml:"autoFilter,omitempty"`
	SortState             *CT_SortState              `xml:"sortState,omitempty"`
	DataConsolidate       *struct{}                  `xml:"-"`
	CustomSheetViews      *struct{}                  `xml:"-"`
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
			prefix := ""
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
				for _, ra := range ws.OriginalRootAttrs {
					if ra.IsNS && ra.Value == attr.Name.Space {
						prefix = ra.Prefix
						break
					}
				}
			}
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
			ws.ChildOrder = append(ws.ChildOrder, name)
		case xml.EndElement:
			return nil
		}
	}
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
	CodeName                          string          `xml:"codeName,attr,omitempty"`
	EnableFormatConditionsCalculation *bool           `xml:"enableFormatConditionsCalculation,attr,omitempty"`
	FilterMode                        *bool           `xml:"filterMode,attr,omitempty"`
	Published                         *bool           `xml:"published,attr,omitempty"`
	SyncHorizontal                    *bool           `xml:"syncHorizontal,attr,omitempty"`
	SyncVertical                      *bool           `xml:"syncVertical,attr,omitempty"`
	TransitionEntry                   *bool           `xml:"transitionEntry,attr,omitempty"`
	TransitionEvaluation              *bool           `xml:"transitionEvaluation,attr,omitempty"`
	TabColor                          *CT_Color       `xml:"tabColor,omitempty"`
	OutlinePr                         *CT_OutlinePr   `xml:"outlinePr,omitempty"`
	PageSetUpPr                       *CT_PageSetUpPr `xml:"pageSetUpPr,omitempty"`
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
type CT_Color struct {
	Auto    *bool    `xml:"auto,attr,omitempty"`
	Indexed *uint32  `xml:"indexed,attr,omitempty"`
	Rgb     string   `xml:"rgb,attr,omitempty"`
	Theme   *uint32  `xml:"theme,attr,omitempty"`
	Tint    *float64 `xml:"tint,attr,omitempty"`
}

// ---------------------------------------------------------------------------
// Sheet views
// ---------------------------------------------------------------------------

// CT_SheetViews represents the sheetViews element.
type CT_SheetViews struct {
	SheetView []CT_SheetView `xml:"sheetView"`
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
			if n, err := strconv.ParseUint(attr.Value, 10, 32); err == nil {
				v := uint32(n)
				sf.BaseColWidth = &v
			}
		case attr.Name.Local == "defaultColWidth" && attr.Name.Space == "":
			v, err := strconv.ParseFloat(attr.Value, 64)
			if err == nil {
				sf.DefaultColWidth = &v
			}
		case attr.Name.Local == "defaultRowHeight" && attr.Name.Space == "":
			v, err := strconv.ParseFloat(attr.Value, 64)
			if err == nil {
				sf.DefaultRowHeight = v
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
			if n, err := strconv.ParseUint(attr.Value, 10, 8); err == nil {
				v := uint8(n)
				sf.OutlineLevelRow = &v
			}
		case attr.Name.Local == "outlineLevelCol" && attr.Name.Space == "":
			if n, err := strconv.ParseUint(attr.Value, 10, 8); err == nil {
				v := uint8(n)
				sf.OutlineLevelCol = &v
			}
		case attr.Name.Local == "dyDescent" && (attr.Name.Space == nsX14AC || attr.Name.Space == "x14ac"):
			v, err := strconv.ParseFloat(attr.Value, 64)
			if err == nil {
				sf.DyDescent = &v
			}
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
	R            *uint32  `xml:"-"`
	Spans        string   `xml:"-"`
	S            *uint32  `xml:"-"`
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
}

// UnmarshalXML implements custom unmarshaling for CT_Row.
func (r *CT_Row) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "r" && attr.Name.Space == "":
			var v uint32
			_, _ = fmt.Sscanf(attr.Value, "%d", &v)
			r.R = &v
		case attr.Name.Local == "spans" && attr.Name.Space == "":
			r.Spans = attr.Value
		case attr.Name.Local == "s" && attr.Name.Space == "":
			var v uint32
			_, _ = fmt.Sscanf(attr.Value, "%d", &v)
			r.S = &v
		case attr.Name.Local == "ht" && attr.Name.Space == "":
			v, err := strconv.ParseFloat(attr.Value, 64)
			if err == nil {
				r.Ht = &v
			}
		case attr.Name.Local == "hidden" && attr.Name.Space == "":
			b := attr.Value == "1" || attr.Value == "true"
			r.Hidden = &b
		case attr.Name.Local == "customHeight" && attr.Name.Space == "":
			b := attr.Value == "1" || attr.Value == "true"
			r.CustomHeight = &b
		case attr.Name.Local == "outlineLevel" && attr.Name.Space == "":
			var v uint8
			_, _ = fmt.Sscanf(attr.Value, "%d", &v)
			r.OutlineLevel = &v
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
			v, err := strconv.ParseFloat(attr.Value, 64)
			if err == nil {
				r.DyDescent = &v
			}
		}
	}

	// Decode child elements
	var helper struct {
		C []*CT_Cell `xml:"c"`
	}
	if err := d.DecodeElement(&helper, &start); err != nil {
		return err
	}
	r.C = helper.C
	return nil
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

	if len(r.C) == 0 {
		b.EmptyElement(ns, localName, attrs...)
		return
	}

	// Sort cells by column index so they appear in ascending order,
	// which is required by the OOXML specification.
	sort.Slice(r.C, func(i, j int) bool {
		return cellRefColIndex(r.C[i].R) < cellRefColIndex(r.C[j].R)
	})

	b.StartElement(ns, localName, attrs...)
	for i := range r.C {
		r.C[i].MarshalToBuilder(b, ns, "c")
	}
	b.EndElement(ns, localName)
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

// CT_Cell represents a c (cell) element.
type CT_Cell struct {
	R  string          `xml:"r,attr"`
	S  *uint32         `xml:"s,attr,omitempty"`
	T  string          `xml:"t,attr,omitempty"`
	F  *CT_CellFormula `xml:"f,omitempty"`
	V  *string         `xml:"v,omitempty"`
	Is *CT_Rst         `xml:"is,omitempty"`
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Cell.
func (c *CT_Cell) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	attrs = append(attrs, xmlb.StrAttr("r", c.R))
	if c.S != nil {
		attrs = append(attrs, xmlb.UintAttr("s", *c.S))
	}
	if c.T != "" {
		attrs = append(attrs, xmlb.StrAttr("t", c.T))
	}

	if c.F == nil && c.V == nil && c.Is == nil {
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
}

// UnmarshalXML implements custom unmarshaling for CT_CellFormula.
func (f *CT_CellFormula) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
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
			var v uint32
			_, _ = fmt.Sscanf(attr.Value, "%d", &v)
			f.Si = &v
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
		case (attr.Name.Local == "id" && attr.Name.Space == nsR) || attr.Name.Local == "r:id":
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
	PaperSize          *uint32 `xml:"-"`
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

// UnmarshalXML implements custom unmarshaling for CT_PageSetup.
func (ps *CT_PageSetup) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "paperSize" && attr.Name.Space == "":
			var v uint32
			_, _ = fmt.Sscanf(attr.Value, "%d", &v)
			ps.PaperSize = &v
		case attr.Name.Local == "scale" && attr.Name.Space == "":
			var v uint32
			_, _ = fmt.Sscanf(attr.Value, "%d", &v)
			ps.Scale = &v
		case attr.Name.Local == "firstPageNumber" && attr.Name.Space == "":
			var v uint32
			_, _ = fmt.Sscanf(attr.Value, "%d", &v)
			ps.FirstPageNumber = &v
		case attr.Name.Local == "fitToWidth" && attr.Name.Space == "":
			var v uint32
			_, _ = fmt.Sscanf(attr.Value, "%d", &v)
			ps.FitToWidth = &v
		case attr.Name.Local == "fitToHeight" && attr.Name.Space == "":
			var v uint32
			_, _ = fmt.Sscanf(attr.Value, "%d", &v)
			ps.FitToHeight = &v
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
			var v uint32
			_, _ = fmt.Sscanf(attr.Value, "%d", &v)
			ps.HorizontalDpi = &v
		case attr.Name.Local == "verticalDpi" && attr.Name.Space == "":
			var v uint32
			_, _ = fmt.Sscanf(attr.Value, "%d", &v)
			ps.VerticalDpi = &v
		case attr.Name.Local == "copies" && attr.Name.Space == "":
			var v uint32
			_, _ = fmt.Sscanf(attr.Value, "%d", &v)
			ps.Copies = &v
		case (attr.Name.Local == "id" && attr.Name.Space == nsR) || attr.Name.Local == "r:id":
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
		if (attr.Name.Local == "id" && attr.Name.Space == nsR) || attr.Name.Local == "r:id" {
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
		if (attr.Name.Local == "id" && attr.Name.Space == nsR) || attr.Name.Local == "r:id" {
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
		if (attr.Name.Local == "id" && attr.Name.Space == nsR) || attr.Name.Local == "r:id" {
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
}

// CT_FilterColumn represents a filterColumn element.
type CT_FilterColumn struct {
	ColId         uint32            `xml:"colId,attr"`
	HiddenButton  *bool             `xml:"hiddenButton,attr,omitempty"`
	ShowButton    *bool             `xml:"showButton,attr,omitempty"`
	Filters       *CT_Filters       `xml:"filters,omitempty"`
	CustomFilters *CT_CustomFilters `xml:"customFilters,omitempty"`
}

// CT_Filters represents the filters element.
type CT_Filters struct {
	Blank  *bool       `xml:"blank,attr,omitempty"`
	Filter []CT_Filter `xml:"filter"`
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
}

// CT_SortCondition represents a sortCondition element.
type CT_SortCondition struct {
	Descending *bool  `xml:"descending,attr,omitempty"`
	SortBy     string `xml:"sortBy,attr,omitempty"`
	Ref        string `xml:"ref,attr"`
	CustomList string `xml:"customList,attr,omitempty"`
}

// ---------------------------------------------------------------------------
// Conditional formatting
// ---------------------------------------------------------------------------

// CT_ConditionalFormatting represents the conditionalFormatting element.
type CT_ConditionalFormatting struct {
	Sqref  string      `xml:"sqref,attr"`
	Pivot  *bool       `xml:"pivot,attr,omitempty"`
	CfRule []CT_CfRule `xml:"cfRule"`
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

// CT_Cfvo represents a cfvo (conditional format value object) element.
type CT_Cfvo struct {
	Type string `xml:"type,attr"`
	Val  string `xml:"val,attr,omitempty"`
}

// ---------------------------------------------------------------------------
// Data validations
// ---------------------------------------------------------------------------

// CT_DataValidations represents the dataValidations element.
type CT_DataValidations struct {
	DisablePrompts *bool               `xml:"disablePrompts,attr,omitempty"`
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
