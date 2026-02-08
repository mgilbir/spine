package oxml

// CT_PivotTableDefinition represents a pivot table definition.
type CT_PivotTableDefinition struct {
	Name              string  `xml:"name,attr,omitempty"`
	CacheId           uint32  `xml:"cacheId,attr,omitempty"`
	DataOnRows        *bool   `xml:"dataOnRows,attr,omitempty"`
	DataPosition      *uint32 `xml:"dataPosition,attr,omitempty"`
	AutoFormatId      *uint32 `xml:"autoFormatId,attr,omitempty"`
	ApplyNumberFormats *bool  `xml:"applyNumberFormats,attr,omitempty"`
	ApplyBorderFormats *bool  `xml:"applyBorderFormats,attr,omitempty"`
	ApplyFontFormats  *bool   `xml:"applyFontFormats,attr,omitempty"`
	ApplyPatternFormats *bool `xml:"applyPatternFormats,attr,omitempty"`
	ApplyAlignmentFormats *bool `xml:"applyAlignmentFormats,attr,omitempty"`
	ApplyWidthHeightFormats *bool `xml:"applyWidthHeightFormats,attr,omitempty"`
	DataCaption       string  `xml:"dataCaption,attr,omitempty"`
	GrandTotalCaption string  `xml:"grandTotalCaption,attr,omitempty"`
	ErrorCaption      string  `xml:"errorCaption,attr,omitempty"`
	ShowError         *bool   `xml:"showError,attr,omitempty"`
	MissingCaption    string  `xml:"missingCaption,attr,omitempty"`
	ShowMissing       *bool   `xml:"showMissing,attr,omitempty"`
	PageStyle         string  `xml:"pageStyle,attr,omitempty"`
	PivotTableStyle   string  `xml:"pivotTableStyle,attr,omitempty"`
	VacatedStyle      string  `xml:"vacatedStyle,attr,omitempty"`
	Tag               string  `xml:"tag,attr,omitempty"`
	UpdatedVersion    *uint32 `xml:"updatedVersion,attr,omitempty"`
	MinRefreshableVersion *uint32 `xml:"minRefreshableVersion,attr,omitempty"`
	AsteriskTotals    *bool   `xml:"asteriskTotals,attr,omitempty"`
	ShowItems         *bool   `xml:"showItems,attr,omitempty"`
	EditData          *bool   `xml:"editData,attr,omitempty"`
	DisableFieldList  *bool   `xml:"disableFieldList,attr,omitempty"`
	ShowCalcMbrs      *bool   `xml:"showCalcMbrs,attr,omitempty"`
	VisualTotals      *bool   `xml:"visualTotals,attr,omitempty"`
	ShowMultipleLabel *bool   `xml:"showMultipleLabel,attr,omitempty"`
	ShowDataDropDown  *bool   `xml:"showDataDropDown,attr,omitempty"`
	ShowDrill         *bool   `xml:"showDrill,attr,omitempty"`
	PrintDrill        *bool   `xml:"printDrill,attr,omitempty"`
	ShowMemberPropertyTips *bool `xml:"showMemberPropertyTips,attr,omitempty"`
	ShowDataTips      *bool   `xml:"showDataTips,attr,omitempty"`
	EnableWizard      *bool   `xml:"enableWizard,attr,omitempty"`
	EnableDrill       *bool   `xml:"enableDrill,attr,omitempty"`
	EnableFieldProperties *bool `xml:"enableFieldProperties,attr,omitempty"`
	PreserveFormatting *bool  `xml:"preserveFormatting,attr,omitempty"`
	UseAutoFormatting *bool   `xml:"useAutoFormatting,attr,omitempty"`
	PageWrap          *uint32 `xml:"pageWrap,attr,omitempty"`
	PageOverThenDown  *bool   `xml:"pageOverThenDown,attr,omitempty"`
	SubtotalHiddenItems *bool `xml:"subtotalHiddenItems,attr,omitempty"`
	RowGrandTotals    *bool   `xml:"rowGrandTotals,attr,omitempty"`
	ColGrandTotals    *bool   `xml:"colGrandTotals,attr,omitempty"`
	FieldPrintTitles  *bool   `xml:"fieldPrintTitles,attr,omitempty"`
	ItemPrintTitles   *bool   `xml:"itemPrintTitles,attr,omitempty"`
	MergeItem         *bool   `xml:"mergeItem,attr,omitempty"`
	ShowDropZones     *bool   `xml:"showDropZones,attr,omitempty"`
	CreatedVersion    *uint32 `xml:"createdVersion,attr,omitempty"`
	Indent            *uint32 `xml:"indent,attr,omitempty"`
	ShowEmptyRow      *bool   `xml:"showEmptyRow,attr,omitempty"`
	ShowEmptyCol      *bool   `xml:"showEmptyCol,attr,omitempty"`
	ShowHeaders       *bool   `xml:"showHeaders,attr,omitempty"`
	Compact           *bool   `xml:"compact,attr,omitempty"`
	Outline           *bool   `xml:"outline,attr,omitempty"`
	OutlineData       *bool   `xml:"outlineData,attr,omitempty"`
	CompactData       *bool   `xml:"compactData,attr,omitempty"`
	Published         *bool   `xml:"published,attr,omitempty"`
	GridDropZones     *bool   `xml:"gridDropZones,attr,omitempty"`
	Immersive         *bool   `xml:"immersive,attr,omitempty"`
	MultipleFieldFilters *bool `xml:"multipleFieldFilters,attr,omitempty"`
	ChartFormat       *uint32 `xml:"chartFormat,attr,omitempty"`
	RowHeaderCaption  string  `xml:"rowHeaderCaption,attr,omitempty"`
	ColHeaderCaption  string  `xml:"colHeaderCaption,attr,omitempty"`
	FieldListSortAscending *bool `xml:"fieldListSortAscending,attr,omitempty"`
	MdxSubqueries     *bool   `xml:"mdxSubqueries,attr,omitempty"`
	CustomListSort    *bool   `xml:"customListSort,attr,omitempty"`

	Location   *CT_Location    `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main location,omitempty"`
	PivotFields *CT_PivotFields `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main pivotFields,omitempty"`
	RowFields  *CT_RowFields   `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main rowFields,omitempty"`
	RowItems   *CT_RowItems    `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main rowItems,omitempty"`
	ColFields  *CT_ColFields   `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main colFields,omitempty"`
	ColItems   *CT_RowItems    `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main colItems,omitempty"`
	DataFields *CT_DataFields  `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main dataFields,omitempty"`
}

// CT_PivotFields represents a collection of pivot fields.
type CT_PivotFields struct {
	Count      *uint32         `xml:"count,attr,omitempty"`
	PivotField []*CT_PivotField `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main pivotField,omitempty"`
}

// CT_PivotField represents a pivot field.
type CT_PivotField struct {
	Name               string  `xml:"name,attr,omitempty"`
	Axis               string  `xml:"axis,attr,omitempty"`
	DataField          *bool   `xml:"dataField,attr,omitempty"`
	SubtotalCaption    string  `xml:"subtotalCaption,attr,omitempty"`
	ShowDropDowns      *bool   `xml:"showDropDowns,attr,omitempty"`
	HiddenLevel        *bool   `xml:"hiddenLevel,attr,omitempty"`
	UniqueMemberProperty string `xml:"uniqueMemberProperty,attr,omitempty"`
	Compact            *bool   `xml:"compact,attr,omitempty"`
	AllDrilled         *bool   `xml:"allDrilled,attr,omitempty"`
	NumFmtId           *uint32 `xml:"numFmtId,attr,omitempty"`
	Outline            *bool   `xml:"outline,attr,omitempty"`
	SubtotalTop        *bool   `xml:"subtotalTop,attr,omitempty"`
	DragToRow          *bool   `xml:"dragToRow,attr,omitempty"`
	DragToCol          *bool   `xml:"dragToCol,attr,omitempty"`
	MultipleItemSelectionAllowed *bool `xml:"multipleItemSelectionAllowed,attr,omitempty"`
	DragToPage         *bool   `xml:"dragToPage,attr,omitempty"`
	DragToData         *bool   `xml:"dragToData,attr,omitempty"`
	DragOff            *bool   `xml:"dragOff,attr,omitempty"`
	ShowAll            *bool   `xml:"showAll,attr,omitempty"`
	InsertBlankRow     *bool   `xml:"insertBlankRow,attr,omitempty"`
	ServerField        *bool   `xml:"serverField,attr,omitempty"`
	InsertPageBreak    *bool   `xml:"insertPageBreak,attr,omitempty"`
	AutoShow           *bool   `xml:"autoShow,attr,omitempty"`
	TopAutoShow        *bool   `xml:"topAutoShow,attr,omitempty"`
	HideNewItems       *bool   `xml:"hideNewItems,attr,omitempty"`
	MeasureFilter      *bool   `xml:"measureFilter,attr,omitempty"`
	IncludeNewItemsInFilter *bool `xml:"includeNewItemsInFilter,attr,omitempty"`
	ItemPageCount      *uint32 `xml:"itemPageCount,attr,omitempty"`
	SortType           string  `xml:"sortType,attr,omitempty"`
	DataSourceSort     *bool   `xml:"dataSourceSort,attr,omitempty"`
	NonAutoSortDefault *bool   `xml:"nonAutoSortDefault,attr,omitempty"`
	RankBy             *uint32 `xml:"rankBy,attr,omitempty"`
	DefaultSubtotal    *bool   `xml:"defaultSubtotal,attr,omitempty"`
	SumSubtotal        *bool   `xml:"sumSubtotal,attr,omitempty"`
	CountASubtotal     *bool   `xml:"countASubtotal,attr,omitempty"`
	AvgSubtotal        *bool   `xml:"avgSubtotal,attr,omitempty"`
	MaxSubtotal        *bool   `xml:"maxSubtotal,attr,omitempty"`
	MinSubtotal        *bool   `xml:"minSubtotal,attr,omitempty"`
	ProductSubtotal    *bool   `xml:"productSubtotal,attr,omitempty"`
	CountSubtotal      *bool   `xml:"countSubtotal,attr,omitempty"`
	StdDevSubtotal     *bool   `xml:"stdDevSubtotal,attr,omitempty"`
	StdDevPSubtotal    *bool   `xml:"stdDevPSubtotal,attr,omitempty"`
	VarSubtotal        *bool   `xml:"varSubtotal,attr,omitempty"`
	VarPSubtotal       *bool   `xml:"varPSubtotal,attr,omitempty"`
	ShowPropCell       *bool   `xml:"showPropCell,attr,omitempty"`
	ShowPropTip        *bool   `xml:"showPropTip,attr,omitempty"`
	ShowPropAsCaption  *bool   `xml:"showPropAsCaption,attr,omitempty"`
	DefaultAttributeDrillState *bool `xml:"defaultAttributeDrillState,attr,omitempty"`

	Items  *CT_Items  `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main items,omitempty"`
}

// CT_Items represents a collection of items.
type CT_Items struct {
	Count *uint32    `xml:"count,attr,omitempty"`
	Item  []*CT_Item `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main item,omitempty"`
}

// CT_Item represents a pivot item.
type CT_Item struct {
	N string  `xml:"n,attr,omitempty"`
	T string  `xml:"t,attr,omitempty"`
	H *bool   `xml:"h,attr,omitempty"`
	S *bool   `xml:"s,attr,omitempty"`
	D *bool   `xml:"d,attr,omitempty"`
	M *bool   `xml:"m,attr,omitempty"`
	C *bool   `xml:"c,attr,omitempty"`
	X *uint32 `xml:"x,attr,omitempty"`
}

// CT_Location represents a pivot table location.
type CT_Location struct {
	Ref          string  `xml:"ref,attr,omitempty"`
	FirstHeaderRow uint32 `xml:"firstHeaderRow,attr,omitempty"`
	FirstDataRow uint32  `xml:"firstDataRow,attr,omitempty"`
	FirstDataCol uint32  `xml:"firstDataCol,attr,omitempty"`
	RowPageCount *uint32 `xml:"rowPageCount,attr,omitempty"`
	ColPageCount *uint32 `xml:"colPageCount,attr,omitempty"`
}

// CT_RowFields represents pivot table row fields.
type CT_RowFields struct {
	Count *uint32       `xml:"count,attr,omitempty"`
	Field []*CT_Field   `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main field,omitempty"`
}

// CT_ColFields represents pivot table column fields.
type CT_ColFields struct {
	Count *uint32       `xml:"count,attr,omitempty"`
	Field []*CT_Field   `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main field,omitempty"`
}

// CT_Field represents a field reference in pivot tables.
type CT_Field struct {
	X int32 `xml:"x,attr"`
}

// CT_RowItems represents pivot table row items.
type CT_RowItems struct {
	Count *uint32     `xml:"count,attr,omitempty"`
	I     []*CT_I     `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main i,omitempty"`
}

// CT_I represents a row/column item.
type CT_I struct {
	T string   `xml:"t,attr,omitempty"`
	R *uint32  `xml:"r,attr,omitempty"`
	I *uint32  `xml:"i,attr,omitempty"`
	X []*CT_X  `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main x,omitempty"`
}

// CT_X represents an index reference.
type CT_X struct {
	V *int32 `xml:"v,attr,omitempty"`
}

// CT_DataFields represents pivot table data fields.
type CT_DataFields struct {
	Count     *uint32        `xml:"count,attr,omitempty"`
	DataField []*CT_DataField `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main dataField,omitempty"`
}

// CT_DataField represents a data field in a pivot table.
type CT_DataField struct {
	Name       string  `xml:"name,attr,omitempty"`
	Fld        uint32  `xml:"fld,attr"`
	Subtotal   string  `xml:"subtotal,attr,omitempty"`
	ShowDataAs string  `xml:"showDataAs,attr,omitempty"`
	BaseField  *int32  `xml:"baseField,attr,omitempty"`
	BaseItem   *uint32 `xml:"baseItem,attr,omitempty"`
	NumFmtId   *uint32 `xml:"numFmtId,attr,omitempty"`
}

// CT_PivotCaches represents pivot caches in the workbook.
type CT_PivotCaches struct {
	PivotCache []*CT_PivotCache `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main pivotCache,omitempty"`
}

// CT_PivotCache represents a single pivot cache reference.
type CT_PivotCache struct {
	CacheId uint32 `xml:"cacheId,attr"`
	RID     string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
}

// CT_PivotSelection represents a pivot table selection.
type CT_PivotSelection struct {
	Pane          string  `xml:"pane,attr,omitempty"`
	ShowHeader    *bool   `xml:"showHeader,attr,omitempty"`
	Label         *bool   `xml:"label,attr,omitempty"`
	Data          *bool   `xml:"data,attr,omitempty"`
	Extendable    *bool   `xml:"extendable,attr,omitempty"`
	Count         *uint32 `xml:"count,attr,omitempty"`
	Axis          string  `xml:"axis,attr,omitempty"`
	Dimension     *uint32 `xml:"dimension,attr,omitempty"`
	Start         *uint32 `xml:"start,attr,omitempty"`
	Min           *uint32 `xml:"min,attr,omitempty"`
	Max           *uint32 `xml:"max,attr,omitempty"`
	ActiveRow     *uint32 `xml:"activeRow,attr,omitempty"`
	ActiveCol     *uint32 `xml:"activeCol,attr,omitempty"`
	PreviousRow   *uint32 `xml:"previousRow,attr,omitempty"`
	PreviousCol   *uint32 `xml:"previousCol,attr,omitempty"`
	Click         *uint32 `xml:"click,attr,omitempty"`
	RID           string  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
}

// CT_CacheSource represents a pivot cache data source.
type CT_CacheSource struct {
	Type              string               `xml:"type,attr,omitempty"`
	ConnectionId      *uint32              `xml:"connectionId,attr,omitempty"`
	WorksheetSource   *CT_WorksheetSource  `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main worksheetSource,omitempty"`
	Consolidation     *CT_Consolidation    `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main consolidation,omitempty"`
}

// CT_WorksheetSource represents a worksheet data source for cache.
type CT_WorksheetSource struct {
	Ref   string `xml:"ref,attr,omitempty"`
	Name  string `xml:"name,attr,omitempty"`
	Sheet string `xml:"sheet,attr,omitempty"`
	RID   string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
}

// CT_Consolidation represents a consolidation data source.
type CT_Consolidation struct {
	AutoPage      *bool             `xml:"autoPage,attr,omitempty"`
	Pages         *CT_Pages         `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main pages,omitempty"`
	RangeSets     *CT_RangeSets     `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main rangeSets,omitempty"`
}

// CT_Pages represents pages in consolidation.
type CT_Pages struct {
	Count *uint32   `xml:"count,attr,omitempty"`
	Page  []*CT_PCDSCPage `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main page,omitempty"`
}

// CT_PCDSCPage represents a page in consolidation.
type CT_PCDSCPage struct {
	Count    *uint32          `xml:"count,attr,omitempty"`
	PageItem []*CT_PageItem   `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main pageItem,omitempty"`
}

// CT_PageItem represents a page item.
type CT_PageItem struct {
	Name string `xml:"name,attr,omitempty"`
}

// CT_RangeSets represents range sets in consolidation.
type CT_RangeSets struct {
	Count    *uint32       `xml:"count,attr,omitempty"`
	RangeSet []*CT_RangeSet `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main rangeSet,omitempty"`
}

// CT_RangeSet represents a range set.
type CT_RangeSet struct {
	I1  *uint32 `xml:"i1,attr,omitempty"`
	I2  *uint32 `xml:"i2,attr,omitempty"`
	I3  *uint32 `xml:"i3,attr,omitempty"`
	I4  *uint32 `xml:"i4,attr,omitempty"`
	Ref string  `xml:"ref,attr,omitempty"`
	Name string `xml:"name,attr,omitempty"`
	Sheet string `xml:"sheet,attr,omitempty"`
	RID string  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
}

// CT_CacheHierarchy represents a cache hierarchy.
type CT_CacheHierarchy struct {
	UniqueName     string  `xml:"uniqueName,attr,omitempty"`
	Caption        string  `xml:"caption,attr,omitempty"`
	Measure        *bool   `xml:"measure,attr,omitempty"`
	Set            *bool   `xml:"set,attr,omitempty"`
	ParentSet      *uint32 `xml:"parentSet,attr,omitempty"`
	IconSet        *int32  `xml:"iconSet,attr,omitempty"`
	Attribute      *bool   `xml:"attribute,attr,omitempty"`
	Time           *bool   `xml:"time,attr,omitempty"`
	KeyAttribute   *bool   `xml:"keyAttribute,attr,omitempty"`
	DefaultMemberUniqueName string `xml:"defaultMemberUniqueName,attr,omitempty"`
	AllUniqueName  string  `xml:"allUniqueName,attr,omitempty"`
	AllCaption     string  `xml:"allCaption,attr,omitempty"`
	DimensionUniqueName string `xml:"dimensionUniqueName,attr,omitempty"`
	DisplayFolder  string  `xml:"displayFolder,attr,omitempty"`
	MeasureGroup   string  `xml:"measureGroup,attr,omitempty"`
	Measures       *bool   `xml:"measures,attr,omitempty"`
	Count          uint32  `xml:"count,attr"`
	OneField       *bool   `xml:"oneField,attr,omitempty"`
	MemberValueDatatype *uint32 `xml:"memberValueDatatype,attr,omitempty"`
	Unbalanced     *bool   `xml:"unbalanced,attr,omitempty"`
	UnbalancedGroup *bool  `xml:"unbalancedGroup,attr,omitempty"`
	Hidden         *bool   `xml:"hidden,attr,omitempty"`

	FieldsUsage  *CT_FieldsUsage  `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main fieldsUsage,omitempty"`
	GroupLevels  *CT_GroupLevels  `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main groupLevels,omitempty"`
}

// CT_CacheFields represents cache fields.
type CT_CacheFields struct {
	Count *uint32 `xml:"count,attr,omitempty"`
}

// CT_Dimensions represents pivot dimensions.
type CT_Dimensions struct {
	Count     *uint32        `xml:"count,attr,omitempty"`
	Dimension []*CT_PivotDimension `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main dimension,omitempty"`
}

// CT_PivotDimension represents a dimension.
type CT_PivotDimension struct {
	Measure    *bool  `xml:"measure,attr,omitempty"`
	Name       string `xml:"name,attr,omitempty"`
	UniqueName string `xml:"uniqueName,attr,omitempty"`
	Caption    string `xml:"caption,attr,omitempty"`
}

// CT_MeasureGroups represents measure groups.
type CT_MeasureGroups struct {
	Count        *uint32          `xml:"count,attr,omitempty"`
	MeasureGroup []*CT_MeasureGroup `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main measureGroup,omitempty"`
}

// CT_MeasureGroup represents a measure group.
type CT_MeasureGroup struct {
	Name    string `xml:"name,attr,omitempty"`
	Caption string `xml:"caption,attr,omitempty"`
}

// CT_FieldsUsage represents fields usage.
type CT_FieldsUsage struct {
	Count      *uint32         `xml:"count,attr,omitempty"`
	FieldUsage []*CT_FieldUsage `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main fieldUsage,omitempty"`
}

// CT_FieldUsage represents a field usage.
type CT_FieldUsage struct {
	X int32 `xml:"x,attr"`
}

// CT_GroupLevels represents group levels.
type CT_GroupLevels struct {
	Count      *uint32          `xml:"count,attr,omitempty"`
	GroupLevel []*CT_GroupLevel `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main groupLevel,omitempty"`
}

// CT_GroupLevel represents a group level.
type CT_GroupLevel struct {
	UniqueName string `xml:"uniqueName,attr,omitempty"`
	Caption    string `xml:"caption,attr,omitempty"`
	User       *bool  `xml:"user,attr,omitempty"`
	CustomRollUp *bool `xml:"customRollUp,attr,omitempty"`
	GroupMembers *CT_GroupMembers `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main groupMembers,omitempty"`
}

// CT_GroupMembers represents group members.
type CT_GroupMembers struct {
	Count       *uint32          `xml:"count,attr,omitempty"`
	GroupMember []*CT_GroupMember `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main groupMember,omitempty"`
}

// CT_GroupMember represents a group member.
type CT_GroupMember struct {
	UniqueName string `xml:"uniqueName,attr,omitempty"`
	Group      *bool  `xml:"group,attr,omitempty"`
}

// CT_MeasureDimensionMaps represents measure dimension maps (the "maps" element in pivot context).
type CT_MeasureDimensionMaps struct {
	Count *uint32                  `xml:"count,attr,omitempty"`
	Map   []*CT_MeasureDimensionMap `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main map,omitempty"`
}

// CT_MeasureDimensionMap represents a measure dimension map (the "map" element in pivot context).
type CT_MeasureDimensionMap struct {
	MeasureGroup *uint32 `xml:"measureGroup,attr,omitempty"`
	Dimension    *uint32 `xml:"dimension,attr,omitempty"`
}

// CT_RangePr represents range properties for a cache field.
type CT_RangePr struct {
	AutoStart    *bool  `xml:"autoStart,attr,omitempty"`
	AutoEnd      *bool  `xml:"autoEnd,attr,omitempty"`
	GroupBy      string `xml:"groupBy,attr,omitempty"`
	StartNum     *float64 `xml:"startNum,attr,omitempty"`
	EndNum       *float64 `xml:"endNum,attr,omitempty"`
	StartDate    string `xml:"startDate,attr,omitempty"`
	EndDate      string `xml:"endDate,attr,omitempty"`
	GroupInterval *float64 `xml:"groupInterval,attr,omitempty"`
}
