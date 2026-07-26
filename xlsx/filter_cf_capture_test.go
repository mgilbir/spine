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

