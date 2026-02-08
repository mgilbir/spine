package oxml

// CT_CustomWorkbookViews represents a collection of custom workbook views.
type CT_CustomWorkbookViews struct {
	CustomWorkbookView []*CT_CustomWorkbookView `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main customWorkbookView,omitempty"`
}

// CT_CustomWorkbookView represents an individual custom workbook view.
type CT_CustomWorkbookView struct {
	Name                 string  `xml:"name,attr,omitempty"`
	Guid                 string  `xml:"guid,attr,omitempty"`
	AutoUpdate           *bool   `xml:"autoUpdate,attr,omitempty"`
	MergeInterval        *uint32 `xml:"mergeInterval,attr,omitempty"`
	ChangesSavedWin      *bool   `xml:"changesSavedWin,attr,omitempty"`
	OnlySync             *bool   `xml:"onlySync,attr,omitempty"`
	PersonalView         *bool   `xml:"personalView,attr,omitempty"`
	IncludePrintSettings *bool   `xml:"includePrintSettings,attr,omitempty"`
	IncludeHiddenRowCol  *bool   `xml:"includeHiddenRowCol,attr,omitempty"`
	Maximized            *bool   `xml:"maximized,attr,omitempty"`
	Minimized            *bool   `xml:"minimized,attr,omitempty"`
	ShowHorizontalScroll *bool   `xml:"showHorizontalScroll,attr,omitempty"`
	ShowVerticalScroll   *bool   `xml:"showVerticalScroll,attr,omitempty"`
	ShowSheetTabs        *bool   `xml:"showSheetTabs,attr,omitempty"`
	XWindow              *int32  `xml:"xWindow,attr,omitempty"`
	YWindow              *int32  `xml:"yWindow,attr,omitempty"`
	WindowWidth          uint32  `xml:"windowWidth,attr,omitempty"`
	WindowHeight         uint32  `xml:"windowHeight,attr,omitempty"`
	TabRatio             *uint32 `xml:"tabRatio,attr,omitempty"`
	ActiveSheetId        *uint32 `xml:"activeSheetId,attr,omitempty"`
	ShowFormulaBar       *bool   `xml:"showFormulaBar,attr,omitempty"`
	ShowStatusbar        *bool   `xml:"showStatusbar,attr,omitempty"`
	ShowComments         string  `xml:"showComments,attr,omitempty"`
	ShowObjects          string  `xml:"showObjects,attr,omitempty"`
}

// CT_WebPublishItems represents a collection of web publish items.
type CT_WebPublishItems struct {
	Count          *uint32              `xml:"count,attr,omitempty"`
	WebPublishItem []*CT_WebPublishItem `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main webPublishItem,omitempty"`
}

// CT_WebPublishItem represents an individual web publish item.
type CT_WebPublishItem struct {
	Id              uint32 `xml:"id,attr,omitempty"`
	DivId           string `xml:"divId,attr,omitempty"`
	SourceType      string `xml:"sourceType,attr,omitempty"`
	SourceRef       string `xml:"sourceRef,attr,omitempty"`
	SourceObject    string `xml:"sourceObject,attr,omitempty"`
	DestinationFile string `xml:"destinationFile,attr,omitempty"`
	Title           string `xml:"title,attr,omitempty"`
	AutoRepublish   *bool  `xml:"autoRepublish,attr,omitempty"`
}

// CT_SmartTagPr represents smart tag properties (SML version).
type CT_SmartTagPr struct {
	Embed *bool  `xml:"embed,attr,omitempty"`
	Show  string `xml:"show,attr,omitempty"`
}

// CT_SmartTagTypes represents a collection of smart tag types.
type CT_SmartTagTypes struct {
	SmartTagType []*CT_SmartTagType `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main smartTagType,omitempty"`
}

// CT_SmartTagType represents a smart tag type definition.
type CT_SmartTagType struct {
	NamespaceUri string `xml:"namespaceUri,attr,omitempty"`
	Name         string `xml:"name,attr,omitempty"`
	Url          string `xml:"url,attr,omitempty"`
}

// CT_SmartTags represents a collection of smart tags (sheet-level).
type CT_SmartTags struct {
	CellSmartTags []*CT_CellSmartTags `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main cellSmartTags,omitempty"`
}

// CT_CellSmartTags represents smart tags for a cell range.
type CT_CellSmartTags struct {
	R            string             `xml:"r,attr,omitempty"`
	CellSmartTag []*CT_CellSmartTag `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main cellSmartTag,omitempty"`
}

// CT_CellSmartTag represents an individual cell smart tag.
type CT_CellSmartTag struct {
	Type           uint32              `xml:"type,attr,omitempty"`
	Deleted        *bool               `xml:"deleted,attr,omitempty"`
	XmlBased       *bool               `xml:"xmlBased,attr,omitempty"`
	CellSmartTagPr []*CT_CellSmartTagPr `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main cellSmartTagPr,omitempty"`
}

// CT_CellSmartTagPr represents a cell smart tag property.
type CT_CellSmartTagPr struct {
	Key string `xml:"key,attr,omitempty"`
	Val string `xml:"val,attr,omitempty"`
}

// CT_MapInfo represents XML mapping info (root element).
type CT_MapInfo struct {
	SelectionNamespaces string       `xml:"SelectionNamespaces,attr,omitempty"`
	Schema              []*CT_Schema `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main Schema,omitempty"`
	Map                 []*CT_Map    `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main Map,omitempty"`
}

// CT_Schema represents an XML schema definition.
type CT_Schema struct {
	ID        string `xml:"ID,attr,omitempty"`
	SchemaRef string `xml:"SchemaRef,attr,omitempty"`
	Namespace string `xml:"Namespace,attr,omitempty"`
}

// CT_Map represents an XML mapping.
type CT_Map struct {
	ID                               uint32          `xml:"ID,attr,omitempty"`
	Name                             string          `xml:"Name,attr,omitempty"`
	RootElement                      string          `xml:"RootElement,attr,omitempty"`
	SchemaID                         string          `xml:"SchemaID,attr,omitempty"`
	ShowImportExportValidationErrors *bool           `xml:"ShowImportExportValidationErrors,attr,omitempty"`
	AutoFit                          *bool           `xml:"AutoFit,attr,omitempty"`
	Append                           *bool           `xml:"Append,attr,omitempty"`
	PreserveSortAFLayout             *bool           `xml:"PreserveSortAFLayout,attr,omitempty"`
	PreserveFormat                   *bool           `xml:"PreserveFormat,attr,omitempty"`
	DataBinding                      *CT_DataBinding `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main DataBinding,omitempty"`
}

// CT_MpMap represents a measure-property map (used in pivot tables).
type CT_MpMap struct {
	V             *uint32 `xml:"v,attr,omitempty"`
	DimensionName string  `xml:"dimensionName,attr,omitempty"`
}
