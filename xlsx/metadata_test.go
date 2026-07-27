package xlsx

import (
	"bytes"
	"strings"
	"testing"
)

// SetDynamicArrayFormula plus save synthesizes xl/metadata.xml, tags the master
// cell with cm="1", and wires the workbook -> metadata relationship, so Excel
// shows the spill without a recalc.
func TestDynamicArrayMetadataSynthesis(t *testing.T) {
	w := Create()
	s := addSheetT(w, "Sheet1")
	c, err := s.Cell("D2")
	if err != nil {
		t.Fatal(err)
	}
	c.SetDynamicArrayFormula("SORT(A2:A10)", "")

	out, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	meta := readZipEntry(t, out, "xl/metadata.xml")
	for _, want := range []string{
		`<metadataType name="XLDAPR"`,
		`<xda:dynamicArrayProperties fDynamic="1" fCollapsed="0"/>`,
		`<cellMetadata count="1"><bk><rc t="1" v="0"/></bk></cellMetadata>`,
	} {
		if !strings.Contains(string(meta), want) {
			t.Errorf("metadata.xml missing %q:\n%s", want, meta)
		}
	}

	sheet1 := readZipEntry(t, out, "xl/worksheets/sheet1.xml")
	if !strings.Contains(string(sheet1), `cm="1"`) {
		t.Errorf("dynamic-array cell not tagged with cm:\n%s", sheet1)
	}

	rels := readZipEntry(t, out, "xl/_rels/workbook.xml.rels")
	if !strings.Contains(string(rels), "sheetMetadata") || !strings.Contains(string(rels), "metadata.xml") {
		t.Errorf("workbook rels missing metadata relationship:\n%s", rels)
	}

	// The cm attribute round-trips (parsed back on reopen).
	rc, err := firstSheet(t, reopen(t, w)).Cell("D2")
	if err != nil {
		t.Fatal(err)
	}
	if rc.cell.Cm == nil || *rc.cell.Cm != 1 {
		t.Errorf("reopened cm = %v, want 1", rc.cell.Cm)
	}
}

// A plain (non dynamic-array) formula does not trigger metadata synthesis.
func TestNoMetadataWithoutDynamicArray(t *testing.T) {
	w := Create()
	s := addSheetT(w, "Sheet1")
	c, err := s.Cell("A1")
	if err != nil {
		t.Fatal(err)
	}
	c.SetFormula("1+1")

	out, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if bytes.Contains(out, []byte("xl/metadata.xml")) {
		t.Error("metadata.xml synthesized for a non dynamic-array workbook")
	}
}
