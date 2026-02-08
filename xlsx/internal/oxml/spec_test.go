package oxml

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/spec/spectest"
)

var smlTypeMap = map[string]reflect.Type{
	// Root types
	"workbook":  reflect.TypeOf(CT_Workbook{}),
	"worksheet": reflect.TypeOf(CT_Worksheet{}),
	"sst":       reflect.TypeOf(CT_Sst{}),

	// Workbook elements
	"bookViews":    reflect.TypeOf(CT_BookViews{}),
	"workbookPr":   reflect.TypeOf(CT_WorkbookPr{}),
	"sheets":       reflect.TypeOf(CT_Sheets{}),
	"sheet":        reflect.TypeOf(CT_Sheet{}),
	"definedNames": reflect.TypeOf(CT_DefinedNames{}),
	"definedName":  reflect.TypeOf(CT_DefinedName{}),
	"calcPr":       reflect.TypeOf(CT_CalcPr{}),
	"fileVersion":  reflect.TypeOf(CT_FileVersion{}),
	"extLst":       reflect.TypeOf(CT_ExtensionList{}),

	// Worksheet elements
	"sheetPr":               reflect.TypeOf(CT_SheetPr{}),
	"dimension":             reflect.TypeOf(CT_SheetDimension{}),
	"sheetViews":            reflect.TypeOf(CT_SheetViews{}),
	"sheetView":             reflect.TypeOf(CT_SheetView{}),
	"cols":                  reflect.TypeOf(CT_Cols{}),
	"col":                   reflect.TypeOf(CT_Col{}),
	"sheetData":             reflect.TypeOf(CT_SheetData{}),
	"row":                   reflect.TypeOf(CT_Row{}),
	"c":                     reflect.TypeOf(CT_Cell{}),
	"f":                     reflect.TypeOf(CT_CellFormula{}),
	"mergeCells":            reflect.TypeOf(CT_MergeCells{}),
	"hyperlinks":            reflect.TypeOf(CT_Hyperlinks{}),
	"pageMargins":           reflect.TypeOf(CT_PageMargins{}),
	"pageSetup":             reflect.TypeOf(CT_PageSetup{}),
	"headerFooter":          reflect.TypeOf(CT_HeaderFooter{}),
	"drawing":               reflect.TypeOf(CT_Drawing{}),
	"tableParts":            reflect.TypeOf(CT_TableParts{}),
	"sheetProtection":       reflect.TypeOf(CT_SheetProtection{}),
	"autoFilter":            reflect.TypeOf(CT_AutoFilter{}),
	"filterColumn":          reflect.TypeOf(CT_FilterColumn{}),
	"filters":               reflect.TypeOf(CT_Filters{}),
	"customFilters":         reflect.TypeOf(CT_CustomFilters{}),
	"sortState":             reflect.TypeOf(CT_SortState{}),
	"conditionalFormatting": reflect.TypeOf(CT_ConditionalFormatting{}),
	"colorScale":            reflect.TypeOf(CT_ColorScale{}),
	"dataBar":               reflect.TypeOf(CT_DataBar{}),
	"iconSet":               reflect.TypeOf(CT_IconSet{}),
	"rowBreaks":             reflect.TypeOf(CT_PageBreak{}),
	"colBreaks":             reflect.TypeOf(CT_PageBreak{}),

	// Shared string types
	"si": reflect.TypeOf(CT_Rst{}),
	"r":  reflect.TypeOf(CT_RElt{}),

	// Style types
	"numFmts":      reflect.TypeOf(CT_NumFmts{}),
	"fonts":        reflect.TypeOf(CT_Fonts{}),
	"fill":         reflect.TypeOf(CT_Fill{}),
	"fills":        reflect.TypeOf(CT_Fills{}),
	"borders":      reflect.TypeOf(CT_Borders{}),
	"cellStyleXfs": reflect.TypeOf(CT_CellStyleXfs{}),
	"cellStyles":   reflect.TypeOf(CT_CellStyles{}),

	// Worksheet extension types (Batch 3)
	"drawingHF":       reflect.TypeOf(CT_DrawingHF{}),
	"commentPr":       reflect.TypeOf(CT_CommentPr{}),
	"controlPr":       reflect.TypeOf(CT_ControlPr{}),
	"objectPr":        reflect.TypeOf(CT_ObjectPr{}),
	"sharedItems":     reflect.TypeOf(CT_SharedItems{}),
	"DataBinding":     reflect.TypeOf(CT_DataBinding{}),
	"protectedRanges": reflect.TypeOf(CT_ProtectedRanges{}),
	"table":           reflect.TypeOf(CT_Table{}),
	"tableColumn":     reflect.TypeOf(CT_TableColumn{}),

	// Worksheet extension types (Phase 3)
	"scenarios":       reflect.TypeOf(CT_Scenarios{}),
	"ignoredErrors":   reflect.TypeOf(CT_IgnoredErrors{}),
	"customSheetView": reflect.TypeOf(CT_CustomSheetView{}),
	"dataConsolidate": reflect.TypeOf(CT_DataConsolidate{}),
	"cellWatches":     reflect.TypeOf(CT_CellWatches{}),
	"controls":        reflect.TypeOf(CT_Controls{}),
	"control":         reflect.TypeOf(CT_Control{}),
	"oleObjects":      reflect.TypeOf(CT_OleObjects{}),
	"picture":         reflect.TypeOf(CT_SheetPicture{}),

	// Workbook extension types (Phase 3)
	"customWorkbookViews": reflect.TypeOf(CT_CustomWorkbookViews{}),
	"webPublishItems":     reflect.TypeOf(CT_WebPublishItems{}),
	"smartTagPr":          reflect.TypeOf(CT_SmartTagPr{}),
	"smartTagTypes":       reflect.TypeOf(CT_SmartTagTypes{}),
	"smartTags":           reflect.TypeOf(CT_SmartTags{}),
	"cellSmartTags":       reflect.TypeOf(CT_CellSmartTags{}),

	// Pivot table types (Phase 3)
	"pivotTableDefinition": reflect.TypeOf(CT_PivotTableDefinition{}),
	"pivotField":           reflect.TypeOf(CT_PivotField{}),
	"pivotCaches":          reflect.TypeOf(CT_PivotCaches{}),
	"pivotSelection":       reflect.TypeOf(CT_PivotSelection{}),
	"cacheSource":          reflect.TypeOf(CT_CacheSource{}),
	"cacheHierarchy":       reflect.TypeOf(CT_CacheHierarchy{}),
	"cacheFields":          reflect.TypeOf(CT_CacheFields{}),
	"rowFields":            reflect.TypeOf(CT_RowFields{}),
	"colFields":            reflect.TypeOf(CT_ColFields{}),
	"dataFields":           reflect.TypeOf(CT_DataFields{}),
	"rowItems":             reflect.TypeOf(CT_RowItems{}),
	"item":                 reflect.TypeOf(CT_Item{}),
	"location":             reflect.TypeOf(CT_Location{}),
	"dimensions":           reflect.TypeOf(CT_Dimensions{}),
	"measureGroups":        reflect.TypeOf(CT_MeasureGroups{}),
	"measureGroup":         reflect.TypeOf(CT_MeasureGroup{}),
	"fieldsUsage":          reflect.TypeOf(CT_FieldsUsage{}),
	"fieldUsage":           reflect.TypeOf(CT_FieldUsage{}),
	"groupMembers":         reflect.TypeOf(CT_GroupMembers{}),
	"groupMember":          reflect.TypeOf(CT_GroupMember{}),
	"rangePr":              reflect.TypeOf(CT_RangePr{}),
	"maps":                 reflect.TypeOf(CT_MeasureDimensionMaps{}),
	"map":                  reflect.TypeOf(CT_MeasureDimensionMap{}),
	"mpMap":                reflect.TypeOf(CT_MpMap{}),

	// Connection / external data types (Phase 3)
	"connection":              reflect.TypeOf(CT_Connection{}),
	"queryTable":              reflect.TypeOf(CT_QueryTable{}),
	"queryTableDeletedFields": reflect.TypeOf(CT_QueryTableDeletedFields{}),
	"ddeLink":                 reflect.TypeOf(CT_DdeLink{}),
	"oleLink":                 reflect.TypeOf(CT_OleLink{}),
	"externalLink":            reflect.TypeOf(CT_ExternalLink{}),
	"volTypes":                reflect.TypeOf(CT_VolTypes{}),
	"tp":                      reflect.TypeOf(CT_VolTopic{}),
	"tr":                      reflect.TypeOf(CT_VolTr{}),
	"cell":                    reflect.TypeOf(CT_ExternalCell{}),

	// Comments / calc chain / metadata (Phase 3)
	"comments":  reflect.TypeOf(CT_Comments{}),
	"calcChain": reflect.TypeOf(CT_CalcChain{}),
	"metadata":  reflect.TypeOf(CT_Metadata{}),
	"Order":     reflect.TypeOf(CT_Order{}),

	// XML mapping types (Phase 3)
	"MapInfo": reflect.TypeOf(CT_MapInfo{}),
	"Map":     reflect.TypeOf(CT_Map{}),
	"Schema":  reflect.TypeOf(CT_Schema{}),
}

// smlOutOfScope lists elements that appear in SML spec examples but are not
// SpreadsheetML content types (they belong to OPC or other namespaces).
var smlOutOfScope = map[string]string{
	"Relationships": "OPC package relationships, not SML content",
	"oddHeader":     "bare xsd:string child of headerFooter, not a complex type",
}

func smlTestdataPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "spec", "testdata", "sml_examples.json")
}

func TestSML_SpecExamples_WellFormed(t *testing.T) {
	examples := spectest.LoadExamples(t, smlTestdataPath())
	spectest.TestWellFormedExamples(t, examples, spectest.WrapSML)
}

func TestSML_SpecExamples_Unmarshal(t *testing.T) {
	examples := spectest.LoadExamples(t, smlTestdataPath())
	spectest.TestUnmarshalExamplesWithSkips(t, examples, smlTypeMap, spectest.WrapSML, smlOutOfScope)
}

func TestSML_SpecExamples_RoundTrip(t *testing.T) {
	examples := spectest.LoadExamples(t, smlTestdataPath())
	marshalFn := func(v interface{}, rootElem string) ([]byte, error) {
		b := xmlb.NewSpreadsheetMLBuilder()
		b.MarshalElement(xmlb.NSSpreadsheetML, rootElem, v)
		return b.Bytes(), nil
	}
	spectest.TestRoundTripExamplesWithSkips(t, examples, smlTypeMap, spectest.WrapSML, marshalFn, smlOutOfScope)
}
