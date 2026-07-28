package oxml

import (
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/internal/testutil"
)

// TestXlsxChildCaptureSchemaOrder pins the rank tables behind C329's fix for
// every SpreadsheetML type that uses the common/xml child-capture kit. Children
// added after parse are spliced by struct field index, so the declaration order
// must be the XSD content-model order — transcribed here from ISO/IEC 29500-4
// sml.xsd. Each of these types carries an unmodeled trailing extLst as a raw
// captured child, which is exactly what an appended addition used to overtake.
func TestXlsxChildCaptureSchemaOrder(t *testing.T) {
	newBuilder := func() *xmlb.Builder { return xmlb.NewSpreadsheetMLBuilder() }
	testutil.CheckSchemaChildOrder(t, newBuilder, xmlb.NSSpreadsheetML, []testutil.SchemaOrder{
		// --- styles.xml ---
		{
			Name: "CT_Xf", New: func() any { return &CT_Xf{} }, Model: testutil.Sequence,
			Children: []string{"alignment", "protection"}, // + unmodeled extLst
		},
		{
			// CT_CellStyle's only child is extLst, which is unmodeled.
			Name: "CT_CellStyle", New: func() any { return &CT_CellStyle{} }, Model: testutil.Sequence,
			Children: nil,
		},
		{
			Name: "CT_Dxf", New: func() any { return &CT_Dxf{} }, Model: testutil.Sequence,
			Children: []string{"font", "numFmt", "fill", "alignment", "border", "protection"},
		},
		// --- workbook.xml ---
		{
			Name: "CT_BookViews", New: func() any { return &CT_BookViews{} }, Model: testutil.Sequence,
			Children: []string{"workbookView"},
		},
		{
			Name: "CT_Sheets", New: func() any { return &CT_Sheets{} }, Model: testutil.Sequence,
			Children: []string{"sheet"},
		},
		{
			Name: "CT_DefinedNames", New: func() any { return &CT_DefinedNames{} }, Model: testutil.Sequence,
			Children: []string{"definedName"},
		},
		// --- worksheet.xml ---
		{
			Name: "CT_SheetViews", New: func() any { return &CT_SheetViews{} }, Model: testutil.Sequence,
			Children: []string{"sheetView"},
		},
		{
			// pivotSelection (between selection and extLst) is unmodeled.
			Name: "CT_SheetView", New: func() any { return &CT_SheetView{} }, Model: testutil.Sequence,
			Children: []string{"pane", "selection"},
		},
		{
			Name: "CT_AutoFilter", New: func() any { return &CT_AutoFilter{} }, Model: testutil.Sequence,
			Children: []string{"filterColumn", "sortState"},
		},
		{
			// CT_FilterColumn is an xsd:choice of at most one filter kind, so
			// no two children ever coexist and the order is unobservable; the
			// declaration order still follows the schema's listing.
			Name: "CT_FilterColumn", New: func() any { return &CT_FilterColumn{} }, Model: testutil.Choice,
			Children: []string{"filters", "customFilters"},
		},
		{
			// dateGroupItem (after filter) is unmodeled.
			Name: "CT_Filters", New: func() any { return &CT_Filters{} }, Model: testutil.Sequence,
			Children: []string{"filter"},
		},
		{
			Name: "CT_SortState", New: func() any { return &CT_SortState{} }, Model: testutil.Sequence,
			Children: []string{"sortCondition"},
		},
		{
			Name: "CT_ConditionalFormatting", New: func() any { return &CT_ConditionalFormatting{} }, Model: testutil.Sequence,
			Children: []string{"cfRule"},
		},
		{
			Name: "CT_CfRule", New: func() any { return &CT_CfRule{} }, Model: testutil.Sequence,
			Children: []string{"formula", "colorScale", "dataBar", "iconSet"},
		},
		{
			// CT_Cfvo's only child is extLst, which is unmodeled.
			Name: "CT_Cfvo", New: func() any { return &CT_Cfvo{} }, Model: testutil.Sequence,
			Children: nil,
		},
	})
}
