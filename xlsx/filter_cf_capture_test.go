package xlsx

import (
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// C274: CT_FilterColumn modeled only filters/customFilters and CT_Filters
// modeled only filter children, so a dirty save dropped every other filter
// kind (top10, dynamicFilter, colorFilter, iconFilter, extLst) and date
// filters (dateGroupItem plus the calendarType attribute). The unmodeled
// children must be captured raw and re-emitted, with modeled filters intact.
func TestFilterColumnUnmodeledChildrenPreserved(t *testing.T) {
	const src = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<sheetData/>` +
		`<autoFilter ref="A1:C5">` +
		`<filterColumn colId="0"><top10 top="1" percent="0" val="5"/></filterColumn>` +
		`<filterColumn colId="1"><dynamicFilter type="today"/></filterColumn>` +
		`<filterColumn colId="2"><filters calendarType="gregorian"><filter val="a"/><dateGroupItem year="2020" dateTimeGrouping="year"/></filters></filterColumn>` +
		`</autoFilter>` +
		`</worksheet>`

	var ws oxml.CT_Worksheet
	if err := xmlb.UnmarshalWithSource([]byte(src), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Modeled filters remain exposed for reads.
	if ws.AutoFilter == nil || len(ws.AutoFilter.FilterColumn) != 3 {
		t.Fatalf("autoFilter not decoded: %+v", ws.AutoFilter)
	}
	fcDate := ws.AutoFilter.FilterColumn[2]
	if fcDate.Filters == nil || fcDate.Filters.CalendarType != "gregorian" {
		t.Fatalf("calendarType attr not decoded: %+v", fcDate.Filters)
	}
	if len(fcDate.Filters.Filter) != 1 || fcDate.Filters.Filter[0].Val != "a" {
		t.Fatalf("modeled filter not decoded: %+v", fcDate.Filters)
	}

	data, err := marshalWorksheetXML(&ws)
	if err != nil {
		t.Fatalf("marshalWorksheetXML: %v", err)
	}
	out := string(data)
	for _, want := range []string{
		`<top10 top="1" percent="0" val="5"/>`,
		`<dynamicFilter type="today"/>`,
		`calendarType="gregorian"`,
		`<dateGroupItem year="2020" dateTimeGrouping="year"/>`,
		`<filter val="a"/>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("dropped %q on dirty save:\n%s", want, out)
		}
	}
	// dateGroupItem must follow the modeled filter (source order).
	if i, j := strings.Index(out, "<filter "), strings.Index(out, "<dateGroupItem"); i < 0 || j < 0 || i > j {
		t.Errorf("filters child order not preserved:\n%s", out)
	}
}

// C274: CT_CfRule modeled formula/colorScale/dataBar/iconSet but not extLst,
// which carries the x14:id linking a 2010+ conditional-format rule to its x14
// counterpart (dataBars). Dropping it on a dirty save severed the pairing; the
// extLst must be captured raw and re-emitted after the modeled dataBar.
func TestCfRuleExtLstPreserved(t *testing.T) {
	const src = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<sheetData/>` +
		`<conditionalFormatting sqref="A1:A5">` +
		`<cfRule type="dataBar" priority="1">` +
		`<dataBar><cfvo type="min"/><cfvo type="max"/><color rgb="FF638EC6"/></dataBar>` +
		`<extLst><ext xmlns:x14="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main" uri="{B025F937-C7B1-47D3-B67F-A62EFF666E3E}">` +
		`<x14:id>{DA7ABA51-AAAA-BBBB-CCCC-000000000001}</x14:id></ext></extLst>` +
		`</cfRule>` +
		`</conditionalFormatting>` +
		`</worksheet>`

	var ws oxml.CT_Worksheet
	if err := xmlb.UnmarshalWithSource([]byte(src), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(ws.ConditionalFormatting) != 1 || len(ws.ConditionalFormatting[0].CfRule) != 1 {
		t.Fatalf("conditionalFormatting not decoded: %+v", ws.ConditionalFormatting)
	}
	if ws.ConditionalFormatting[0].CfRule[0].DataBar == nil {
		t.Fatalf("modeled dataBar not decoded")
	}

	data, err := marshalWorksheetXML(&ws)
	if err != nil {
		t.Fatalf("marshalWorksheetXML: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, `<x14:id>{DA7ABA51-AAAA-BBBB-CCCC-000000000001}</x14:id>`) {
		t.Errorf("cfRule extLst x14:id dropped on dirty save:\n%s", out)
	}
	if !strings.Contains(out, `uri="{B025F937-C7B1-47D3-B67F-A62EFF666E3E}"`) {
		t.Errorf("cfRule extLst ext uri dropped:\n%s", out)
	}
	// extLst must follow the modeled dataBar (source order); the x14:id
	// pairing stays attached to this rule.
	if i, j := strings.Index(out, "<dataBar"), strings.Index(out, "<extLst"); i < 0 || j < 0 || i > j {
		t.Errorf("cfRule child order not preserved:\n%s", out)
	}
}
