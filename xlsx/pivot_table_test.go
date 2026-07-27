package xlsx

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// buildPivotSourceWorkbook creates a workbook with a Data sheet holding a small
// table and an empty Report sheet to anchor a pivot on.
func buildPivotSourceWorkbook(t *testing.T) *Workbook {
	t.Helper()
	wb := Create()
	data := addSheetT(wb, "Data")
	rows := [][]interface{}{
		{"Region", "Product", "Sales"},
		{"North", "A", 10.0},
		{"North", "B", 20.0},
		{"South", "A", 30.0},
		{"South", "B", 40.0},
	}
	for r, row := range rows {
		for c, v := range row {
			ref := FormatCellRef(r+1, c+1)
			cell, err := data.Cell(ref)
			if err != nil {
				t.Fatalf("Cell(%s): %v", ref, err)
			}
			cell.SetValue(v)
		}
	}
	addSheetT(wb, "Report")
	return wb
}

func TestAddPivotTable_CreateAndReadBack(t *testing.T) {
	wb := buildPivotSourceWorkbook(t)
	report, err := wb.SheetByName("Report")
	if err != nil {
		t.Fatalf("SheetByName(Report): %v", err)
	}

	pt, err := report.AddPivotTable("Data!A1:C5", "A3", PivotOptions{
		RowFields:   []string{"Region"},
		ValueFields: []PivotValueField{{Field: "Sales", Aggregation: PivotSum}},
	})
	if err != nil {
		t.Fatalf("AddPivotTable: %v", err)
	}
	if pt.Name() != "PivotTable1" {
		t.Errorf("Name = %q, want PivotTable1", pt.Name())
	}

	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	// The package must carry the pivot parts and their content-type overrides.
	for _, part := range []string{
		"xl/pivotTables/pivotTable1.xml",
		"xl/pivotCache/pivotCacheDefinition1.xml",
		"xl/pivotCache/pivotCacheRecords1.xml",
	} {
		if !zipHasPart(t, out, part) {
			t.Errorf("saved package is missing %s", part)
		}
	}

	reopened, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if report := reopened.Validate(); report.HasErrors() {
		t.Errorf("reopened workbook has validation errors: %v", report)
	}

	pivots := reopened.PivotTables()
	if len(pivots) != 1 {
		t.Fatalf("PivotTables() = %d, want 1", len(pivots))
	}
	got := pivots[0]
	if got.Name() != "PivotTable1" {
		t.Errorf("Name = %q, want PivotTable1", got.Name())
	}
	if got.SourceSheet() != "Data" {
		t.Errorf("SourceSheet = %q, want Data", got.SourceSheet())
	}
	if got.SourceRange() != "A1:C5" {
		t.Errorf("SourceRange = %q, want A1:C5", got.SourceRange())
	}
	if rf := got.RowFields(); len(rf) != 1 || rf[0] != "Region" {
		t.Errorf("RowFields = %v, want [Region]", rf)
	}
	vals := got.ValueFields()
	if len(vals) != 1 {
		t.Fatalf("ValueFields = %d, want 1", len(vals))
	}
	if vals[0].Field != "Sales" || vals[0].Aggregation != PivotSum || vals[0].Name != "Sum of Sales" {
		t.Errorf("ValueFields[0] = %+v", vals[0])
	}
}

func TestAddPivotTable_RowsColsValuesFilters(t *testing.T) {
	wb := Create()
	data := addSheetT(wb, "Data")
	rows := [][]interface{}{
		{"Region", "Product", "Year", "Sales", "Qty"},
		{"North", "A", "2023", 10.0, 1.0},
		{"North", "B", "2024", 20.0, 2.0},
		{"South", "A", "2023", 30.0, 3.0},
		{"South", "B", "2024", 40.0, 4.0},
	}
	for r, row := range rows {
		for c, v := range row {
			cell, _ := data.Cell(FormatCellRef(r+1, c+1))
			cell.SetValue(v)
		}
	}
	report := addSheetT(wb, "Report")

	_, err := report.AddPivotTable("Data!A1:E5", "A1", PivotOptions{
		RowFields:    []string{"Region"},
		ColumnFields: []string{"Product"},
		Filters:      []string{"Year"},
		ValueFields: []PivotValueField{
			{Field: "Sales", Aggregation: PivotSum},
			{Field: "Qty", Aggregation: PivotAverage},
		},
	})
	if err != nil {
		t.Fatalf("AddPivotTable: %v", err)
	}

	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	reopened, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if r := reopened.Validate(); r.HasErrors() {
		t.Errorf("validation errors: %v", r)
	}
	pivots := reopened.PivotTables()
	if len(pivots) != 1 {
		t.Fatalf("PivotTables() = %d, want 1", len(pivots))
	}
	got := pivots[0]
	if rf := got.RowFields(); len(rf) != 1 || rf[0] != "Region" {
		t.Errorf("RowFields = %v, want [Region]", rf)
	}
	if cf := got.ColumnFields(); len(cf) != 1 || cf[0] != "Product" {
		t.Errorf("ColumnFields = %v, want [Product]", cf)
	}
	if f := got.Filters(); len(f) != 1 || f[0] != "Year" {
		t.Errorf("Filters = %v, want [Year]", f)
	}
	vals := got.ValueFields()
	if len(vals) != 2 {
		t.Fatalf("ValueFields = %d, want 2", len(vals))
	}
	if vals[0].Field != "Sales" || vals[0].Aggregation != PivotSum {
		t.Errorf("ValueFields[0] = %+v", vals[0])
	}
	if vals[1].Field != "Qty" || vals[1].Aggregation != PivotAverage {
		t.Errorf("ValueFields[1] = %+v", vals[1])
	}
}

// buildPivotProfitWorkbook creates a workbook with Region/Sales/Cost columns
// and an empty Report sheet, for calculated-field tests.
func buildPivotProfitWorkbook(t *testing.T) *Workbook {
	t.Helper()
	wb := Create()
	data := addSheetT(wb, "Data")
	rows := [][]interface{}{
		{"Region", "Sales", "Cost"},
		{"North", 100.0, 60.0},
		{"South", 200.0, 150.0},
		{"North", 300.0, 210.0},
	}
	for r, row := range rows {
		for c, v := range row {
			cell, _ := data.Cell(FormatCellRef(r+1, c+1))
			cell.SetValue(v)
		}
	}
	addSheetT(wb, "Report")
	return wb
}

func TestAddPivotTable_CalculatedField(t *testing.T) {
	wb := buildPivotProfitWorkbook(t)
	report, _ := wb.SheetByName("Report")
	_, err := report.AddPivotTable("Data!A1:C4", "A3", PivotOptions{
		RowFields:   []string{"Region"},
		ValueFields: []PivotValueField{{Field: "Sales", Aggregation: PivotSum}},
		CalculatedFields: []PivotCalculatedField{
			{Name: "Profit", Formula: "Sales-Cost"},
		},
	})
	if err != nil {
		t.Fatalf("AddPivotTable: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	// The cache definition carries the calculated field with its formula.
	cacheXML := string(readZipPart(t, out, "xl/pivotCache/pivotCacheDefinition1.xml"))
	if !strings.Contains(cacheXML, `formula="Sales-Cost"`) {
		t.Errorf("cache definition missing calculated formula:\n%s", cacheXML)
	}
	if !strings.Contains(cacheXML, `name="Profit" numFmtId="0" databaseField="0"`) {
		t.Errorf("cache definition missing calculated field header:\n%s", cacheXML)
	}

	reopened, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if r := reopened.Validate(); r.HasErrors() {
		t.Errorf("validation errors: %v", r)
	}
	pivots := reopened.PivotTables()
	if len(pivots) != 1 {
		t.Fatalf("PivotTables() = %d, want 1", len(pivots))
	}
	var foundProfit bool
	for _, v := range pivots[0].ValueFields() {
		if v.Field == "Profit" && v.Name == "Sum of Profit" {
			foundProfit = true
		}
	}
	if !foundProfit {
		t.Errorf("calculated field not read back as a value field: %+v", pivots[0].ValueFields())
	}
}

func TestAddPivotTable_NumericGroup(t *testing.T) {
	wb := Create()
	data := addSheetT(wb, "Data")
	rows := [][]interface{}{
		{"Name", "Age", "Sales"},
		{"A", 24.0, 10.0},
		{"B", 31.0, 20.0},
		{"C", 47.0, 30.0},
		{"D", 33.0, 40.0},
	}
	for r, row := range rows {
		for c, v := range row {
			cell, _ := data.Cell(FormatCellRef(r+1, c+1))
			cell.SetValue(v)
		}
	}
	report := addSheetT(wb, "Report")
	_, err := report.AddPivotTable("Data!A1:C5", "A3", PivotOptions{
		ValueFields: []PivotValueField{{Field: "Sales", Aggregation: PivotSum}},
		NumericGroups: []PivotNumericGroup{
			{Field: "Age", Start: 20, End: 50, Interval: 10},
		},
	})
	if err != nil {
		t.Fatalf("AddPivotTable: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	cacheXML := string(readZipPart(t, out, "xl/pivotCache/pivotCacheDefinition1.xml"))
	for _, want := range []string{"<fieldGroup base=", "<rangePr ", `groupInterval="10"`, "<groupItems ", `<s v="20-30"/>`} {
		if !strings.Contains(cacheXML, want) {
			t.Errorf("cache definition missing %q:\n%s", want, cacheXML)
		}
	}

	reopened, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if r := reopened.Validate(); r.HasErrors() {
		t.Errorf("validation errors: %v", r)
	}
	pivots := reopened.PivotTables()
	if len(pivots) != 1 {
		t.Fatalf("PivotTables() = %d, want 1", len(pivots))
	}
	if rf := pivots[0].RowFields(); len(rf) != 1 || rf[0] != "Age (grouped)" {
		t.Errorf("RowFields = %v, want [Age (grouped)]", rf)
	}
}

func TestAddPivotTable_DateGroup(t *testing.T) {
	wb := Create()
	data := addSheetT(wb, "Data")
	// Header row.
	for c, h := range []string{"When", "Sales"} {
		cell, _ := data.Cell(FormatCellRef(1, c+1))
		cell.SetValue(h)
	}
	dates := []time.Time{
		time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC),
		time.Date(2023, 3, 10, 0, 0, 0, 0, time.UTC),
		time.Date(2023, 1, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 6, 5, 0, 0, 0, 0, time.UTC),
	}
	sales := []float64{10, 20, 30, 40}
	for i, d := range dates {
		dc, _ := data.Cell(FormatCellRef(i+2, 1))
		dc.SetTime(d)
		if err := dc.SetStyle(CellStyle{Format: "mm-dd-yy"}); err != nil { // builtin date format
			t.Fatalf("SetStyle: %v", err)
		}
		sc, _ := data.Cell(FormatCellRef(i+2, 2))
		sc.SetFloat(sales[i])
	}
	report := addSheetT(wb, "Report")

	_, err := report.AddPivotTable("Data!A1:B5", "A3", PivotOptions{
		ValueFields: []PivotValueField{{Field: "Sales", Aggregation: PivotSum}},
		DateGroups:  []PivotDateGroup{{Field: "When", By: PivotByMonth}},
	})
	if err != nil {
		t.Fatalf("AddPivotTable: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	cacheXML := string(readZipPart(t, out, "xl/pivotCache/pivotCacheDefinition1.xml"))
	for _, want := range []string{
		`groupBy="months"`, `startDate="2023-01-15T00:00:00"`, "<fieldGroup base=",
		`<s v="Jan"/>`, `<s v="Dec"/>`, `containsDate="1"`, `<d v="2023-01-15T00:00:00"/>`,
	} {
		if !strings.Contains(cacheXML, want) {
			t.Errorf("cache definition missing %q:\n%s", want, cacheXML)
		}
	}

	reopened, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if r := reopened.Validate(); r.HasErrors() {
		t.Errorf("validation errors: %v", r)
	}
	pivots := reopened.PivotTables()
	if len(pivots) != 1 {
		t.Fatalf("PivotTables() = %d, want 1", len(pivots))
	}
	if rf := pivots[0].RowFields(); len(rf) != 1 || rf[0] != "When (grouped)" {
		t.Errorf("RowFields = %v, want [When (grouped)]", rf)
	}
}

func TestAddPivotTable_ItemGroup(t *testing.T) {
	wb := Create()
	data := addSheetT(wb, "Data")
	rows := [][]interface{}{
		{"State", "Sales"},
		{"CA", 10.0},
		{"WA", 20.0},
		{"NY", 30.0},
		{"FL", 40.0},
	}
	for r, row := range rows {
		for c, v := range row {
			cell, _ := data.Cell(FormatCellRef(r+1, c+1))
			cell.SetValue(v)
		}
	}
	report := addSheetT(wb, "Report")

	_, err := report.AddPivotTable("Data!A1:B5", "A3", PivotOptions{
		ValueFields: []PivotValueField{{Field: "Sales", Aggregation: PivotSum}},
		ItemGroups: []PivotItemGroup{{
			Field: "State",
			Groups: []PivotNamedGroup{
				{Name: "West", Items: []string{"CA", "WA"}},
				{Name: "East", Items: []string{"NY", "FL"}},
			},
		}},
	})
	if err != nil {
		t.Fatalf("AddPivotTable: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	cacheXML := string(readZipPart(t, out, "xl/pivotCache/pivotCacheDefinition1.xml"))
	for _, want := range []string{
		"<fieldGroup base=", "<groupItems ", `<s v="West"/>`, `<s v="East"/>`, "<discretePr ",
	} {
		if !strings.Contains(cacheXML, want) {
			t.Errorf("cache definition missing %q:\n%s", want, cacheXML)
		}
	}

	reopened, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if r := reopened.Validate(); r.HasErrors() {
		t.Errorf("validation errors: %v", r)
	}
	pivots := reopened.PivotTables()
	if len(pivots) != 1 {
		t.Fatalf("PivotTables() = %d, want 1", len(pivots))
	}
	if rf := pivots[0].RowFields(); len(rf) != 1 || rf[0] != "State (grouped)" {
		t.Errorf("RowFields = %v, want [State (grouped)]", rf)
	}
}

func TestAddPivotTable_ItemGroupUnknownItemRejected(t *testing.T) {
	wb := buildPivotSourceWorkbook(t)
	report, _ := wb.SheetByName("Report")
	_, err := report.AddPivotTable("Data!A1:C5", "A3", PivotOptions{
		ValueFields: []PivotValueField{{Field: "Sales", Aggregation: PivotSum}},
		ItemGroups: []PivotItemGroup{{
			Field:  "Region",
			Groups: []PivotNamedGroup{{Name: "G", Items: []string{"Nowhere"}}},
		}},
	})
	if err == nil {
		t.Fatal("AddPivotTable accepted an item group naming a nonexistent item; want error")
	}
}

func TestAddPivotTable_ExtendExistingCaches(t *testing.T) {
	// Round 1: create a workbook with one pivot and save it, so the reopened
	// workbook genuinely carries a <pivotCaches> element and pivot cache parts.
	wb := buildPivotSourceWorkbook(t)
	report, _ := wb.SheetByName("Report")
	if _, err := report.AddPivotTable("Data!A1:C5", "A3", PivotOptions{
		RowFields:   []string{"Region"},
		ValueFields: []PivotValueField{{Field: "Sales", Aggregation: PivotSum}},
	}); err != nil {
		t.Fatalf("first AddPivotTable: %v", err)
	}
	out1, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("first SaveBytes: %v", err)
	}

	// Round 2: reopen (now with an existing pivot cache) and add a second pivot.
	wb2, err := OpenReader(bytes.NewReader(out1), int64(len(out1)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := len(wb2.PivotTables()); got != 1 {
		t.Fatalf("reopened PivotTables() = %d, want 1", got)
	}
	report2 := addSheetT(wb2, "Report2")
	pt2, err := report2.AddPivotTable("Data!A1:C5", "A3", PivotOptions{
		RowFields:   []string{"Product"},
		ValueFields: []PivotValueField{{Field: "Sales", Aggregation: PivotSum}},
	})
	if err != nil {
		t.Fatalf("extend AddPivotTable: %v", err)
	}
	if pt2.CacheID() == 0 {
		t.Errorf("extended pivot cache id is zero")
	}

	out2, err := wb2.SaveBytes()
	if err != nil {
		t.Fatalf("second SaveBytes: %v", err)
	}

	// Both cache definitions and both pivot tables must be present.
	for _, part := range []string{
		"xl/pivotTables/pivotTable1.xml",
		"xl/pivotTables/pivotTable2.xml",
		"xl/pivotCache/pivotCacheDefinition1.xml",
		"xl/pivotCache/pivotCacheDefinition2.xml",
	} {
		if !zipHasPart(t, out2, part) {
			t.Errorf("extended package is missing %s", part)
		}
	}

	// The workbook carries exactly one <pivotCaches> with two <pivotCache> entries.
	wbXML := string(readZipPart(t, out2, "xl/workbook.xml"))
	if n := strings.Count(wbXML, "<pivotCaches"); n != 1 {
		t.Errorf("workbook.xml has %d <pivotCaches> elements, want 1:\n%s", n, wbXML)
	}
	if n := strings.Count(wbXML, "<pivotCache "); n != 2 {
		t.Errorf("workbook.xml has %d <pivotCache> entries, want 2:\n%s", n, wbXML)
	}

	reopened, err := OpenReader(bytes.NewReader(out2), int64(len(out2)))
	if err != nil {
		t.Fatalf("final reopen: %v", err)
	}
	if r := reopened.Validate(); r.HasErrors() {
		t.Errorf("validation errors: %v", r)
	}
	if got := len(reopened.PivotTables()); got != 2 {
		t.Fatalf("final PivotTables() = %d, want 2", got)
	}
}

func TestAddPivotTable_NonNumericValueRejected(t *testing.T) {
	wb := buildPivotSourceWorkbook(t)
	report, _ := wb.SheetByName("Report")
	_, err := report.AddPivotTable("Data!A1:C5", "A3", PivotOptions{
		RowFields:   []string{"Region"},
		ValueFields: []PivotValueField{{Field: "Product", Aggregation: PivotSum}},
	})
	if err == nil {
		t.Fatal("AddPivotTable accepted a Sum over a non-numeric field; want error")
	}
}

// pivotFixtureParts are the pivot parts of a hand-crafted workbook that already
// contains a pivot table, used to verify byte-identical raw preservation and
// read-back.
var pivotFixtureParts = map[string]string{
	"xl/worksheets/_rels/sheet1.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/pivotTable" Target="../pivotTables/pivotTable1.xml"/></Relationships>`,
	"xl/pivotTables/pivotTable1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<pivotTableDefinition xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" name="SalesPivot" cacheId="7" dataCaption="Values" outline="1" outlineData="1"><location ref="A3:B6" firstHeaderRow="1" firstDataRow="1" firstDataCol="1"/><pivotFields count="3"><pivotField axis="axisRow" showAll="0"><items count="3"><item x="0"/><item x="1"/><item t="default"/></items></pivotField><pivotField showAll="0"/><pivotField dataField="1" showAll="0"/></pivotFields><rowFields count="1"><field x="0"/></rowFields><rowItems count="3"><i><x/></i><i><x v="1"/></i><i t="grand"><x/></i></rowItems><colItems count="1"><i/></colItems><dataFields count="1"><dataField name="Sum of Sales" fld="2" baseField="0" baseItem="0"/></dataFields></pivotTableDefinition>`,
	"xl/pivotTables/_rels/pivotTable1.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/pivotCacheDefinition" Target="../pivotCache/pivotCacheDefinition1.xml"/></Relationships>`,
	"xl/pivotCache/pivotCacheDefinition1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<pivotCacheDefinition xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:id="rId1" refreshedBy="Excel" recordCount="4"><cacheSource type="worksheet"><worksheetSource ref="A1:C5" sheet="Sheet1"/></cacheSource><cacheFields count="3"><cacheField name="Region" numFmtId="0"><sharedItems count="2"><s v="North"/><s v="South"/></sharedItems></cacheField><cacheField name="Product" numFmtId="0"><sharedItems count="2"><s v="A"/><s v="B"/></sharedItems></cacheField><cacheField name="Sales" numFmtId="0"><sharedItems containsSemiMixedTypes="0" containsString="0" containsNumber="1" containsInteger="1" minValue="10" maxValue="40"/></cacheField></cacheFields></pivotCacheDefinition>`,
	"xl/pivotCache/_rels/pivotCacheDefinition1.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/pivotCacheRecords" Target="pivotCacheRecords1.xml"/></Relationships>`,
	"xl/pivotCache/pivotCacheRecords1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<pivotCacheRecords xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="4"><r><x v="0"/><x v="0"/><n v="10"/></r><r><x v="0"/><x v="1"/><n v="20"/></r><r><x v="1"/><x v="0"/><n v="30"/></r><r><x v="1"/><x v="1"/><n v="40"/></r></pivotCacheRecords>`,
}

const pivotFixtureOverrides = `<Override PartName="/xl/pivotTables/pivotTable1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.pivotTable+xml"/>` +
	`<Override PartName="/xl/pivotCache/pivotCacheDefinition1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.pivotCacheDefinition+xml"/>` +
	`<Override PartName="/xl/pivotCache/pivotCacheRecords1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.pivotCacheRecords+xml"/>`

const pivotFixtureWbRels = `<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/pivotCacheDefinition" Target="pivotCache/pivotCacheDefinition1.xml"/>`

const pivotFixtureSheet = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
	`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData/></worksheet>`

func TestPivotTable_ExistingRoundTripByteIdentical(t *testing.T) {
	data := buildFidelityTestXlsx(t, pivotFixtureSheet, pivotFixtureParts, pivotFixtureOverrides, pivotFixtureWbRels)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Read-back: the pivot table and its cache resolve to the expected fields.
	pivots := wb.PivotTables()
	if len(pivots) != 1 {
		t.Fatalf("PivotTables() = %d, want 1", len(pivots))
	}
	got := pivots[0]
	if got.Name() != "SalesPivot" {
		t.Errorf("Name = %q, want SalesPivot", got.Name())
	}
	if got.SourceSheet() != "Sheet1" || got.SourceRange() != "A1:C5" {
		t.Errorf("source = %q!%q, want Sheet1!A1:C5", got.SourceSheet(), got.SourceRange())
	}
	if rf := got.RowFields(); len(rf) != 1 || rf[0] != "Region" {
		t.Errorf("RowFields = %v, want [Region]", rf)
	}
	if vals := got.ValueFields(); len(vals) != 1 || vals[0].Field != "Sales" || vals[0].Name != "Sum of Sales" {
		t.Errorf("ValueFields = %+v", vals)
	}

	// Byte-identical raw preservation: an unmodified save re-emits every pivot
	// part exactly as it came in.
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	for name, want := range pivotFixtureParts {
		if strings.HasSuffix(name, ".rels") {
			continue // rels round-trip is covered elsewhere; assert the XML parts
		}
		got := readZipPart(t, out, name)
		if !bytes.Equal(got, []byte(want)) {
			t.Errorf("part %s not byte-identical after round-trip:\n got: %s\nwant: %s", name, got, want)
		}
	}
}
