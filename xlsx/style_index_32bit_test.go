package xlsx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/fuzzseed"
)

// TestStyleIndexAbove2p31 confirms a style, font, fill, border, cellStyleXfs or
// dxf index with the top bit set is treated as out of range rather than
// indexed. Those bounds were once written as int(idx) < len(...): on a 32-bit
// platform int(0xFFFFFFFF) is -1, so the check passed and the slice was then
// indexed with the unconverted uint32 and panicked. Every index here comes
// straight from the file, and the copy path needs no style call by the caller.
// Only a 32-bit build (CI's GOARCH=386 job) can fail this test; on 64-bit the
// old check happened to hold.
func TestStyleIndexAbove2p31(t *testing.T) {
	base := buildXlsxPartsFixture()
	const huge = "4294967295"
	appendSheets := func(w *Workbook) { _, _ = Create().AppendSheetsFrom(w) }
	for _, tc := range []struct {
		name, part, from, to string
		drive                func(w *Workbook)
	}{
		{"cell s via CellStyleAt", partSheet1, `<c r="G1" s="1">`, `<c r="G1" s="` + huge + `">`, func(w *Workbook) {
			s, _ := w.SheetByName("Data")
			c, _ := s.Cell("G1")
			if _, err := w.Styles().CellStyleAt(*c.StyleIndex()); err == nil {
				t.Error("CellStyleAt: want an out-of-range error")
			}
		}},
		{"cell s via AppendSheetsFrom", partSheet1, `<c r="G1" s="1">`, `<c r="G1" s="` + huge + `">`, appendSheets},
		{"xf fontId via CellStyleAt", partStyles, `fontId="1" fillId="2" borderId="1"`, `fontId="` + huge + `" fillId="2" borderId="1"`, func(w *Workbook) {
			_, _ = w.Styles().CellStyleAt(1)
		}},
		{"xf fontId via AppendSheetsFrom", partStyles, `fontId="1" fillId="2" borderId="1"`, `fontId="` + huge + `" fillId="2" borderId="1"`, appendSheets},
		{"xf fillId via CellStyleAt", partStyles, `fontId="1" fillId="2" borderId="1"`, `fontId="1" fillId="` + huge + `" borderId="1"`, func(w *Workbook) {
			_, _ = w.Styles().CellStyleAt(1)
		}},
		{"xf fillId via AppendSheetsFrom", partStyles, `fontId="1" fillId="2" borderId="1"`, `fontId="1" fillId="` + huge + `" borderId="1"`, appendSheets},
		{"xf borderId via CellStyleAt", partStyles, `fontId="1" fillId="2" borderId="1"`, `fontId="1" fillId="2" borderId="` + huge + `"`, func(w *Workbook) {
			_, _ = w.Styles().CellStyleAt(1)
		}},
		{"xf borderId via AppendSheetsFrom", partStyles, `fontId="1" fillId="2" borderId="1"`, `fontId="1" fillId="2" borderId="` + huge + `"`, appendSheets},
		{"cellStyle xfId via NamedStyles", partStyles, `<cellStyle name="Normal" xfId="0"`, `<cellStyle name="Normal" xfId="` + huge + `"`, func(w *Workbook) {
			_ = w.Styles().NamedStyles()
		}},
		{"cellStyle xfId via ApplyNamedStyle", partStyles, `<cellStyle name="Normal" xfId="0"`, `<cellStyle name="Normal" xfId="` + huge + `"`, func(w *Workbook) {
			if _, err := w.Styles().ApplyNamedStyle("Normal"); err == nil {
				t.Error("ApplyNamedStyle: want an out-of-range error")
			}
		}},
		{"cfRule dxfId", "xl/worksheets/sheet2.xml", `</sheetData>`, `</sheetData><conditionalFormatting sqref="A1"><cfRule type="expression" priority="1" dxfId="` + huge + `"><formula>TRUE</formula></cfRule></conditionalFormatting>`, func(w *Workbook) {
			s, _ := w.SheetByName("Notes")
			cfs := s.ConditionalFormats()
			if len(cfs) != 1 || len(cfs[0].Rules) != 1 {
				t.Fatalf("conditional formats = %d, want 1 with 1 rule", len(cfs))
			}
			if d := cfs[0].Rules[0].DifferentialFormat(); d != nil {
				t.Errorf("DifferentialFormat = %+v, want nil", d)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			orig := string(fuzzseed.ZipEntry(base, tc.part))
			mod := strings.Replace(orig, tc.from, tc.to, 1)
			if mod == orig {
				t.Fatal("substitution did not apply")
			}
			pkg := fuzzseed.EditZip(base, [][2]string{{tc.part, mod}})
			w, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic: %v", r)
				}
			}()
			tc.drive(w)
		})
	}
	// The public API takes a uint32 too, so a caller can reach it directly.
	t.Run("CellStyleAt(MaxUint32)", func(t *testing.T) {
		w, err := OpenReader(bytes.NewReader(base), int64(len(base)))
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic: %v", r)
			}
		}()
		if _, err := w.Styles().CellStyleAt(0xFFFFFFFF); err == nil {
			t.Error("want an out-of-range error")
		}
	})
}
