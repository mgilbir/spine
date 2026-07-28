package pptx

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

// tableWithFlags builds a deck with one 2x2 table and rewrites its a:tblPr so
// every banding/heading flag is set in the source, as PowerPoint writes for a
// table using a header row and banding.
func tableWithFlags(t *testing.T) []byte {
	t.Helper()
	p := Create()
	s := p.AddSlide()
	tbl := s.AddTable(2, 2)
	tbl.SetFirstRow(true)

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	return rewriteZipPart(t, data, "ppt/slides/slide1.xml", func(xml []byte) []byte {
		re := regexp.MustCompile(`<a:tblPr[^>]*>`)
		loc := re.FindIndex(xml)
		if loc == nil {
			t.Fatalf("no a:tblPr in the table:\n%s", xml)
		}
		const attrs = ` rtl="1" firstRow="1" firstCol="1" lastRow="1" lastCol="1" bandRow="1" bandCol="1"`
		// Keep the source's open/close form: the generated tag is self-closing.
		tail := ">"
		if bytes.HasSuffix(xml[loc[0]:loc[1]], []byte("/>")) {
			tail = "/>"
		}
		out := append([]byte{}, xml[:loc[0]]...)
		out = append(out, "<a:tblPr"+attrs+tail...)
		return append(out, xml[loc[1]:]...)
	})
}

// firstTable returns the first table shape of the first slide.
func firstTable(t *testing.T, p *Presentation) *Table {
	t.Helper()
	for _, sh := range p.Slides()[0].Shapes() {
		if tbl, ok := sh.(*Table); ok {
			return tbl
		}
	}
	t.Fatal("no table in the deck")
	return nil
}

// C583: the six banding/heading setters were silent no-ops on a parsed table.
// The modeled zero is omitempty-suppressed, so ReplayCapturedAttrs restored the
// source's ="1" and the caller's false never reached the file.
func TestTable_ClearingStyleFlags_ReachesTheXML(t *testing.T) {
	cases := []struct {
		attr string
		set  func(*Table)
	}{
		{"firstRow", func(tb *Table) { tb.SetFirstRow(false) }},
		{"lastRow", func(tb *Table) { tb.SetLastRow(false) }},
		{"firstCol", func(tb *Table) { tb.SetFirstCol(false) }},
		{"lastCol", func(tb *Table) { tb.SetLastCol(false) }},
		{"bandRow", func(tb *Table) { tb.SetBandedRows(false) }},
		{"bandCol", func(tb *Table) { tb.SetBandedCols(false) }},
	}
	for _, tc := range cases {
		t.Run(tc.attr, func(t *testing.T) {
			deck := tableWithFlags(t)
			p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
			if err != nil {
				t.Fatal(err)
			}
			tc.set(firstTable(t, p))

			saved, err := p.SaveBytes()
			if err != nil {
				t.Fatal(err)
			}
			out := string(zipPart(t, saved, "ppt/slides/slide1.xml"))

			if strings.Contains(out, tc.attr+`="1"`) {
				t.Errorf("clearing %s did not reach the XML:\n%s", tc.attr, out)
			}
		})
	}
}

// C583: clearing one flag must not clear the others, and must not disturb the
// unmodeled rtl attribute.
func TestTable_ClearingOneFlag_LeavesTheRestAlone(t *testing.T) {
	deck := tableWithFlags(t)
	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	firstTable(t, p).SetFirstRow(false)

	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	out := string(zipPart(t, saved, "ppt/slides/slide1.xml"))

	if strings.Contains(out, `firstRow="1"`) {
		t.Errorf("firstRow was not cleared:\n%s", out)
	}
	for _, want := range []string{`lastRow="1"`, `firstCol="1"`, `lastCol="1"`, `bandRow="1"`, `bandCol="1"`} {
		if !strings.Contains(out, want) {
			t.Errorf("clearing firstRow also cleared %s\n%s", want, out)
		}
	}
	// rtl is not modeled by the Table API, so the capture must still replay it.
	if !strings.Contains(out, `rtl="1"`) {
		t.Errorf("the unmodeled rtl attribute was dropped:\n%s", out)
	}
}

// C418: a span could be widened but never narrowed. normalizeMergeCells only
// ever set continuation flags, so after SetColSpan(2), save, SetColSpan(1),
// save, the covered cell still emitted hMerge="1" with no spanning master —
// a merge with no owner.
func TestTable_NarrowingColSpan_ClearsTheContinuationFlag(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	tbl := s.AddTable(2, 2)
	tbl.Rows()[0].Cells()[0].SetColSpan(2)

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(zipPart(t, data, "ppt/slides/slide1.xml")), `hMerge="1"`) {
		t.Fatal("SetColSpan(2) did not mark the covered cell")
	}

	tbl.Rows()[0].Cells()[0].SetColSpan(1)
	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, out, "ppt/slides/slide1.xml"))

	if strings.Contains(xml, `hMerge="1"`) {
		t.Errorf("narrowing the span left a stale hMerge with no spanning master:\n%s", xml)
	}
	if strings.Contains(xml, `gridSpan="2"`) {
		t.Errorf("the span itself was not narrowed:\n%s", xml)
	}
}

// C418: the same for a row span.
func TestTable_NarrowingRowSpan_ClearsTheContinuationFlag(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	tbl := s.AddTable(2, 2)
	tbl.Rows()[0].Cells()[0].SetRowSpan(2)

	if _, err := p.SaveBytes(); err != nil {
		t.Fatal(err)
	}
	tbl.Rows()[0].Cells()[0].SetRowSpan(1)

	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, out, "ppt/slides/slide1.xml"))
	if strings.Contains(xml, `vMerge="1"`) {
		t.Errorf("narrowing the row span left a stale vMerge:\n%s", xml)
	}
}

// C586: the six flags used to be dropped from the capture unconditionally by
// the setter's flush, which cleared them correctly but also deleted a zero the
// producer had written *explicitly*. Replay now decides per attribute — it
// drops a captured value only when the model disagrees with it — so an explicit
// bandRow="0" survives a flush that clears a different flag.
func TestTable_ExplicitZeroFlagSurvivesAFlush(t *testing.T) {
	deck := tableWithExplicitZeroFlags(t)
	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	// Touch the table props so the flush path runs.
	firstTable(t, p).SetFirstRow(false)

	saved, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	out := string(zipPart(t, saved, "ppt/slides/slide1.xml"))

	if !strings.Contains(out, `bandRow="0"`) {
		t.Errorf("an explicitly-written bandRow=\"0\" was deleted by the flush:\n%s", out)
	}
	if strings.Contains(out, `firstRow="1"`) {
		t.Errorf("clearing firstRow did not reach the XML:\n%s", out)
	}
	if !strings.Contains(out, `rtl="1"`) {
		t.Errorf("the unmodeled rtl attribute was lost:\n%s", out)
	}
}

// tableWithExplicitZeroFlags builds a deck whose a:tblPr carries firstRow="1"
// alongside an explicitly-written bandRow="0" and an unmodeled rtl="1".
func tableWithExplicitZeroFlags(t *testing.T) []byte {
	t.Helper()
	p := Create()
	s := p.AddSlide()
	s.AddTable(2, 2)

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	return rewriteZipPart(t, data, "ppt/slides/slide1.xml", func(xml []byte) []byte {
		re := regexp.MustCompile(`<a:tblPr[^>]*>`)
		loc := re.FindIndex(xml)
		if loc == nil {
			t.Fatalf("no a:tblPr in the table:\n%s", xml)
		}
		const attrs = ` rtl="1" firstRow="1" bandRow="0"`
		tail := ">"
		if bytes.HasSuffix(xml[loc[0]:loc[1]], []byte("/>")) {
			tail = "/>"
		}
		out := append([]byte{}, xml[:loc[0]]...)
		out = append(out, "<a:tblPr"+attrs+tail...)
		return append(out, xml[loc[1]:]...)
	})
}

// C418: widening still works — the recompute must not break the C310 fix.
func TestTable_WideningColSpan_StillMarksCoveredCells(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	tbl := s.AddTable(2, 3)
	tbl.Rows()[0].Cells()[0].SetColSpan(3)

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if strings.Count(xml, `hMerge="1"`) != 2 {
		t.Errorf("a 3-wide span must mark 2 covered cells, got %d:\n%s",
			strings.Count(xml, `hMerge="1"`), xml)
	}
}
