package oxml

// ---------------------------------------------------------------------------
// Worksheet extension types for SML (SpreadsheetML) spec test compliance.
// ---------------------------------------------------------------------------

// CT_Scenarios represents the scenarios container element.
type CT_Scenarios struct {
	Current  *uint32        `xml:"current,attr,omitempty"`
	Show     *uint32        `xml:"show,attr,omitempty"`
	Scenario []*CT_Scenario `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main scenario,omitempty"`
}

// CT_Scenario represents an individual scenario element.
type CT_Scenario struct {
	Name       string           `xml:"name,attr,omitempty"`
	Locked     *bool            `xml:"locked,attr,omitempty"`
	Hidden     *bool            `xml:"hidden,attr,omitempty"`
	Count      *uint32          `xml:"count,attr,omitempty"`
	User       string           `xml:"user,attr,omitempty"`
	Comment    string           `xml:"comment,attr,omitempty"`
	InputCells []*CT_InputCells `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main inputCells,omitempty"`
}

// CT_InputCells represents input cells for a scenario.
type CT_InputCells struct {
	R        string  `xml:"r,attr,omitempty"`
	Deleted  *bool   `xml:"deleted,attr,omitempty"`
	Undone   *bool   `xml:"undone,attr,omitempty"`
	Val      string  `xml:"val,attr,omitempty"`
	NumFmtId *uint32 `xml:"numFmtId,attr,omitempty"`
}

// CT_IgnoredErrors represents a container for ignored errors.
type CT_IgnoredErrors struct {
	IgnoredError []*CT_IgnoredError `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main ignoredError,omitempty"`
}

// CT_IgnoredError represents an individual ignored error entry.
type CT_IgnoredError struct {
	SqRef              string `xml:"sqref,attr,omitempty"`
	EvalError          *bool  `xml:"evalError,attr,omitempty"`
	TwoDigitTextYear   *bool  `xml:"twoDigitTextYear,attr,omitempty"`
	NumberStoredAsText *bool  `xml:"numberStoredAsText,attr,omitempty"`
	Formula            *bool  `xml:"formula,attr,omitempty"`
	FormulaRange       *bool  `xml:"formulaRange,attr,omitempty"`
	UnlockedFormula    *bool  `xml:"unlockedFormula,attr,omitempty"`
	EmptyCellReference *bool  `xml:"emptyCellReference,attr,omitempty"`
	ListDataValidation *bool  `xml:"listDataValidation,attr,omitempty"`
	CalculatedColumn   *bool  `xml:"calculatedColumn,attr,omitempty"`
}

// CT_CustomSheetView represents a custom sheet view element.
type CT_CustomSheetView struct {
	Guid           string `xml:"guid,attr,omitempty"`
	Scale          *uint32 `xml:"scale,attr,omitempty"`
	ColorId        *uint32 `xml:"colorId,attr,omitempty"`
	ShowPageBreaks *bool   `xml:"showPageBreaks,attr,omitempty"`
	ShowFormulas   *bool   `xml:"showFormulas,attr,omitempty"`
	ShowGridLines  *bool   `xml:"showGridLines,attr,omitempty"`
	ShowRowCol     *bool   `xml:"showRowCol,attr,omitempty"`
	OutlineSymbols *bool   `xml:"outlineSymbols,attr,omitempty"`
	ZeroValues     *bool   `xml:"zeroValues,attr,omitempty"`
	FitToPage      *bool   `xml:"fitToPage,attr,omitempty"`
	PrintArea      *bool   `xml:"printArea,attr,omitempty"`
	Filter         *bool   `xml:"filter,attr,omitempty"`
	ShowAutoFilter *bool   `xml:"showAutoFilter,attr,omitempty"`
	HiddenRows     *bool   `xml:"hiddenRows,attr,omitempty"`
	HiddenColumns  *bool   `xml:"hiddenColumns,attr,omitempty"`
	State          string  `xml:"state,attr,omitempty"`
	FilterUnique   *bool   `xml:"filterUnique,attr,omitempty"`
	View           string  `xml:"view,attr,omitempty"`
	ShowRuler      *bool   `xml:"showRuler,attr,omitempty"`
	TopLeftCell    string  `xml:"topLeftCell,attr,omitempty"`

	Pane         *CT_Pane         `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main pane,omitempty"`
	Selection    *CT_Selection    `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main selection,omitempty"`
	PageMargins  *CT_PageMargins  `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main pageMargins,omitempty"`
	PageSetup    *CT_PageSetup    `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main pageSetup,omitempty"`
	HeaderFooter *CT_HeaderFooter `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main headerFooter,omitempty"`
	AutoFilter   *CT_AutoFilter   `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main autoFilter,omitempty"`
}

// CT_DataConsolidate represents data consolidation settings.
type CT_DataConsolidate struct {
	Function    string       `xml:"function,attr,omitempty"`
	StartLabels *bool        `xml:"startLabels,attr,omitempty"`
	LeftLabels  *bool        `xml:"leftLabels,attr,omitempty"`
	TopLabels   *bool        `xml:"topLabels,attr,omitempty"`
	Link        *bool        `xml:"link,attr,omitempty"`
	DataRefs    *CT_DataRefs `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main dataRefs,omitempty"`
}

// CT_DataRefs represents a collection of data references for consolidation.
type CT_DataRefs struct {
	Count   *uint32       `xml:"count,attr,omitempty"`
	DataRef []*CT_DataRef `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main dataRef,omitempty"`
}

// CT_DataRef represents a single data reference.
type CT_DataRef struct {
	Ref   string `xml:"ref,attr,omitempty"`
	Name  string `xml:"name,attr,omitempty"`
	Sheet string `xml:"sheet,attr,omitempty"`
	RID   string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
}

// CT_CellWatches represents a cell watches container.
type CT_CellWatches struct {
	CellWatch []*CT_CellWatch `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main cellWatch,omitempty"`
}

// CT_CellWatch represents a single cell watch.
type CT_CellWatch struct {
	R string `xml:"r,attr,omitempty"`
}

// CT_Controls represents an ActiveX controls container.
type CT_Controls struct {
	Control []*CT_Control `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main control,omitempty"`
}

// CT_Control represents an ActiveX control element.
type CT_Control struct {
	ShapeId uint32 `xml:"shapeId,attr,omitempty"`
	RID     string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
	Name    string `xml:"name,attr,omitempty"`
}

// CT_OleObjects represents an OLE objects container.
type CT_OleObjects struct {
	OleObject []*CT_OleObject `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main oleObject,omitempty"`
}

// CT_OleObject represents an OLE object element.
type CT_OleObject struct {
	ProgId    string `xml:"progId,attr,omitempty"`
	DvAspect  string `xml:"dvAspect,attr,omitempty"`
	Link      string `xml:"link,attr,omitempty"`
	OleUpdate string `xml:"oleUpdate,attr,omitempty"`
	AutoLoad  *bool  `xml:"autoLoad,attr,omitempty"`
	ShapeId   uint32 `xml:"shapeId,attr,omitempty"`
	RID       string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
}

// CT_SheetPicture represents a sheet background picture element.
type CT_SheetPicture struct {
	RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
}
