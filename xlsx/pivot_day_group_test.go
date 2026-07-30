package xlsx

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Day grouping is the one date-grouping unit whose bucket labels are generated
// rather than taken from a fixed twelve-entry list: dayOfYearItems builds all
// 366 "d-Mon" labels from a per-month day-count table, and the record indices
// point into that list positionally. A wrong entry in the day-count table (a
// 28-day February being the obvious one, since the list must include 29-Feb for
// leap-year dates to have a bucket) shifts every label after it, so records
// silently land under the wrong day. Asserting only that the pivot builds, or
// that "1-Jan" appears somewhere, would not see that: the assertions below pin
// the labels BY POSITION at the month boundaries.

// groupItemLabels extracts the <s v="..."/> labels of a cache definition's
// groupItems block, in order.
func groupItemLabels(t *testing.T, cacheXML string) []string {
	t.Helper()
	start := strings.Index(cacheXML, "<groupItems")
	if start < 0 {
		t.Fatalf("cache definition has no groupItems:\n%s", cacheXML)
	}
	end := strings.Index(cacheXML[start:], "</groupItems>")
	if end < 0 {
		t.Fatalf("cache definition has an unterminated groupItems:\n%s", cacheXML)
	}
	block := cacheXML[start : start+end]
	re := regexp.MustCompile(`<s v="([^"]*)"/>`)
	var out []string
	for _, m := range re.FindAllStringSubmatch(block, -1) {
		out = append(out, m[1])
	}
	return out
}

// A day-grouped pivot must carry all 366 day buckets between its two bound
// items, with each label at the position its date maps to.
func TestAddPivotTableDayGroupLabels(t *testing.T) {
	wb := Create()
	defer func() { _ = wb.Close() }()
	data := addSheetT(wb, "Data")
	for c, h := range []string{"When", "Sales"} {
		cell, err := data.Cell(FormatCellRef(1, c+1))
		if err != nil {
			t.Fatalf("Cell: %v", err)
		}
		cell.SetValue(h)
	}
	dates := []time.Time{
		time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),   // first bucket
		time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),  // leap day: needs its own bucket
		time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC),   // the day after the leap day slot
		time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC), // last bucket
	}
	for i, d := range dates {
		dc, err := data.Cell(FormatCellRef(i+2, 1))
		if err != nil {
			t.Fatalf("Cell: %v", err)
		}
		dc.SetTime(d)
		if err := dc.SetStyle(CellStyle{Format: "mm-dd-yy"}); err != nil {
			t.Fatalf("SetStyle: %v", err)
		}
		sc, err := data.Cell(FormatCellRef(i+2, 2))
		if err != nil {
			t.Fatalf("Cell: %v", err)
		}
		sc.SetFloat(float64((i + 1) * 10))
	}

	report := addSheetT(wb, "Report")
	if _, err := report.AddPivotTable("Data!A1:B5", "A3", PivotOptions{
		ValueFields: []PivotValueField{{Field: "Sales", Aggregation: PivotSum}},
		DateGroups:  []PivotDateGroup{{Field: "When", By: PivotByDay}},
	}); err != nil {
		t.Fatalf("AddPivotTable: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	cacheXML := string(readZipPart(t, out, "xl/pivotCache/pivotCacheDefinition1.xml"))
	if !strings.Contains(cacheXML, `groupBy="days"`) {
		t.Errorf("cache definition is not day-grouped:\n%s", cacheXML)
	}

	labels := groupItemLabels(t, cacheXML)
	// One leading bound item, 366 day buckets, one trailing bound item.
	if len(labels) != 368 {
		// A missing 29-Feb is the likely cause, and it also silently refiles
		// every leap-day record under another bucket.
		t.Fatalf("groupItems has %d labels, want 368 (2 bounds + 366 days); first day %q, last day %q",
			len(labels), labels[1], labels[len(labels)-2])
	}
	// Positional checks at every month boundary: a wrong day count for any
	// month shifts every later label, and only a positional assertion sees it.
	firstOfMonth := []struct {
		index int
		label string
	}{
		{1, "1-Jan"},
		{32, "1-Feb"},
		{60, "29-Feb"}, // the leap day must have its own bucket
		{61, "1-Mar"},
		{92, "1-Apr"},
		{122, "1-May"},
		{153, "1-Jun"},
		{183, "1-Jul"},
		{214, "1-Aug"},
		{245, "1-Sep"},
		{275, "1-Oct"},
		{306, "1-Nov"},
		{336, "1-Dec"},
		{366, "31-Dec"},
	}
	for _, want := range firstOfMonth {
		if got := labels[want.index]; got != want.label {
			t.Errorf("groupItems[%d] = %q, want %q (the day-of-year table is off)",
				want.index, got, want.label)
		}
	}
	// Every day label must be unique: a duplicated bucket means two calendar
	// days collapse into one row of the pivot.
	seen := make(map[string]bool, len(labels))
	for _, l := range labels[1 : len(labels)-1] {
		if seen[l] {
			t.Errorf("duplicate day bucket %q", l)
		}
		seen[l] = true
	}

	re, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = re.Close() }()
	if r := re.Validate(); r.HasErrors() {
		t.Errorf("day-grouped pivot does not validate: %v", r)
	}
}
