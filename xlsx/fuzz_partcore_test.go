package xlsx

import (
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/internal/fuzzseed"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// Parse and re-emit a worksheet, with no package around it.
//
// The companion to the part-level targets, which build a zip, open it, save it
// and open it again — about 11 executions a second, against tens of thousands
// here. At the nightly budget that difference is the whole exploration: a
// thousand executions barely mutates the seeds, while this reaches millions.
//
// The oracle is idempotence from the second pass. The first may legitimately
// normalize what it read; what must not happen is drift after that, which is a
// document whose bytes never settle however many times it is saved.
func FuzzXlsxWorksheetPart(f *testing.F) {
	const open = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`
	f.Add([]byte(open + `<sheetData/></worksheet>`))
	f.Add([]byte{})
	f.Add([]byte("<worksheet"))
	f.Add([]byte(open + `<sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>hi</t></is></c>` +
		`<c r="B1"><v>42</v></c><c r="C1" t="s"><v>0</v></c></row></sheetData></worksheet>`))
	f.Add([]byte(open + `<sheetPr><tabColor rgb="FFFF0000"/></sheetPr><dimension ref="A1:C3"/>` +
		`<sheetViews><sheetView workbookViewId="0"><selection activeCell="A1" sqref="A1"/></sheetView></sheetViews>` +
		`<cols><col min="1" max="1" width="24" customWidth="1"/></cols>` +
		`<sheetData><row r="1"><c r="A1"><f>SUM(B1:B2)</f><v>3</v></c></row></sheetData>` +
		`<mergeCells count="1"><mergeCell ref="A1:B1"/></mergeCells>` +
		`<pageMargins left="0.7" right="0.7" top="0.75" bottom="0.75" header="0.3" footer="0.3"/>` +
		`</worksheet>`))
	// Values that have to survive escaping unchanged on every pass.
	f.Add([]byte(open + `<sheetData><row r="1"><c r="A1" t="inlineStr"><is><t xml:space="preserve">` +
		`a &amp; b &lt;c&gt; ]]&gt; </t></is></c></row></sheetData></worksheet>`))
	// Numbers and references at their limits, where a re-parse can round.
	f.Add([]byte(open + `<sheetData><row r="1048576"><c r="XFD1048576"><v>1e308</v></c>` +
		`<c r="A1048576"><v>-0.30000000000000004</v></c></row></sheetData></worksheet>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		if !fuzzseed.NamesAreValid(data) {
			// A name the source made up is replayed as given; see
			// fuzzseed.NamesAreValid.
			return
		}
		var ws oxml.CT_Worksheet
		if err := xmlb.UnmarshalWithSource(data, &ws); err != nil {
			return
		}
		first, err := marshalWorksheetXML(&ws)
		if err != nil {
			return
		}
		second, ok := reparseWorksheet(t, first)
		if !ok {
			return
		}
		third, ok := reparseWorksheet(t, second)
		if !ok {
			return
		}
		if string(second) != string(third) {
			t.Fatalf("saving a worksheet is not idempotent: the third pass differs from the second\nsecond: %s\nthird:  %s",
				second, third)
		}
	})
}

func reparseWorksheet(t *testing.T, part []byte) ([]byte, bool) {
	t.Helper()
	var ws oxml.CT_Worksheet
	if err := xmlb.UnmarshalWithSource(part, &ws); err != nil {
		t.Fatalf("this library wrote a worksheet it cannot read back: %v\n%s", err, part)
	}
	out, err := marshalWorksheetXML(&ws)
	if err != nil {
		return nil, false
	}
	return out, true
}
