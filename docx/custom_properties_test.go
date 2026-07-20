package docx

import (
	"bytes"
	"os"
	"testing"
	"time"
)

// cpTestValues is the set of value types the custom-properties API supports.
func cpTestValues() (map[string]any, time.Time) {
	tm := time.Date(2021, 3, 4, 5, 6, 7, 0, time.UTC)
	return map[string]any{
		"Str":    "hello & <world>",
		"Int":    int64(42),
		"BigInt": int64(5_000_000_000),
		"Float":  3.5,
		"Bool":   true,
		"Date":   tm,
	}, tm
}

func cpAssert(t *testing.T, m map[string]any, tm time.Time) {
	t.Helper()
	if m["Str"] != "hello & <world>" {
		t.Errorf("Str = %v", m["Str"])
	}
	if m["Int"] != int64(42) {
		t.Errorf("Int = %v", m["Int"])
	}
	if m["BigInt"] != int64(5_000_000_000) {
		t.Errorf("BigInt = %v", m["BigInt"])
	}
	if m["Float"] != 3.5 {
		t.Errorf("Float = %v", m["Float"])
	}
	if m["Bool"] != true {
		t.Errorf("Bool = %v", m["Bool"])
	}
	if dt, _ := m["Date"].(time.Time); !dt.Equal(tm) {
		t.Errorf("Date = %v", m["Date"])
	}
}

// TestDocxCustomPropertiesCreateRoundTrip sets each value type on a created
// document, saves, reopens, and reads the values back. It also verifies the
// content-type override and package relationship were emitted.
func TestDocxCustomPropertiesCreateRoundTrip(t *testing.T) {
	d := Create()
	vals, tm := cpTestValues()
	for k, v := range vals {
		if err := d.SetCustomProperty(k, v); err != nil {
			t.Fatalf("SetCustomProperty(%q): %v", k, err)
		}
	}
	out := saveBytes(t, d)

	if _, ok := zipEntry(t, out, "docProps/custom.xml"); !ok {
		t.Fatal("docProps/custom.xml not written")
	}
	if ct, _ := zipEntry(t, out, "[Content_Types].xml"); !bytes.Contains(ct, []byte("custom-properties+xml")) {
		t.Error("content-type override missing")
	}
	if rels, _ := zipEntry(t, out, "_rels/.rels"); !bytes.Contains(rels, []byte("docProps/custom.xml")) {
		t.Error("package relationship missing")
	}

	d2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d2.Close() }()
	cpAssert(t, d2.CustomProperties(), tm)
}

// TestDocxCustomPropertiesPreservedVerbatim verifies that reopening a document
// with custom properties and saving without touching them reproduces
// docProps/custom.xml byte-for-byte.
func TestDocxCustomPropertiesPreservedVerbatim(t *testing.T) {
	d := Create()
	_ = d.SetCustomProperty("Keep", "value")
	_ = d.SetCustomProperty("Num", int64(9))
	first := saveBytes(t, d)
	firstCX, ok := zipEntry(t, first, "docProps/custom.xml")
	if !ok {
		t.Fatal("no custom.xml in first save")
	}

	d2, err := OpenReader(bytes.NewReader(first), int64(len(first)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d2.Close() }()
	second := saveBytes(t, d2)
	secondCX, ok := zipEntry(t, second, "docProps/custom.xml")
	if !ok {
		t.Fatal("no custom.xml in second save")
	}
	if !bytes.Equal(firstCX, secondCX) {
		t.Errorf("custom.xml drifted on untouched round-trip\n first: %s\nsecond: %s", firstCX, secondCX)
	}
}

// TestDocxCustomPropertiesModifyAndRemove opens a document that already has
// custom properties, edits one and removes another, and checks the reopened
// result.
func TestDocxCustomPropertiesModifyAndRemove(t *testing.T) {
	d := Create()
	_ = d.SetCustomProperty("A", "one")
	_ = d.SetCustomProperty("B", "two")
	_ = d.SetCustomProperty("C", int64(3))
	first := saveBytes(t, d)

	d2, err := OpenReader(bytes.NewReader(first), int64(len(first)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d2.Close() }()
	if err := d2.SetCustomProperty("A", "updated"); err != nil {
		t.Fatal(err)
	}
	if !d2.RemoveCustomProperty("B") {
		t.Fatal("RemoveCustomProperty(B) = false")
	}
	out := saveBytes(t, d2)

	d3, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d3.Close() }()
	m := d3.CustomProperties()
	if m["A"] != "updated" {
		t.Errorf("A = %v, want updated", m["A"])
	}
	if _, ok := m["B"]; ok {
		t.Error("B still present after removal")
	}
	if m["C"] != int64(3) {
		t.Errorf("C = %v, want 3", m["C"])
	}
}

// TestDocxCustomPropertiesAddOnRoundTrip opens a document that has no
// docProps/custom.xml, adds a property, and verifies the part, content-type
// override, and package relationship are created (exercising the injection of
// the relationship into the preserved root _rels/.rels).
func TestDocxCustomPropertiesAddOnRoundTrip(t *testing.T) {
	base := saveBytes(t, Create()) // a created document carries no custom.xml
	if _, ok := zipEntry(t, base, "docProps/custom.xml"); ok {
		t.Skip("baseline unexpectedly has custom.xml")
	}
	d, err := OpenReader(bytes.NewReader(base), int64(len(base)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	if err := d.SetCustomProperty("Added", "later"); err != nil {
		t.Fatal(err)
	}
	out := saveBytes(t, d)
	if _, ok := zipEntry(t, out, "docProps/custom.xml"); !ok {
		t.Fatal("custom.xml not created")
	}
	if ct, _ := zipEntry(t, out, "[Content_Types].xml"); !bytes.Contains(ct, []byte("custom-properties+xml")) {
		t.Error("content-type override missing")
	}
	if rels, _ := zipEntry(t, out, "_rels/.rels"); !bytes.Contains(rels, []byte("docProps/custom.xml")) {
		t.Error("package relationship not injected")
	}
	d2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d2.Close() }()
	if v, _ := d2.CustomProperties()["Added"].(string); v != "later" {
		t.Errorf("Added = %q, want later", v)
	}
}

// TestDocxCustomPropertiesCorpusByteIdentical round-trips a real-world document
// that carries docProps/custom.xml and checks the part is byte-identical when
// left untouched. It skips when the (gitignored) corpus is absent.
func TestDocxCustomPropertiesCorpusByteIdentical(t *testing.T) {
	const path = "../testdata/corpus/cc/docx/ccf323e6e5344608.docx"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skip("corpus file not available:", path)
	}
	origCX, ok := zipEntry(t, raw, "docProps/custom.xml")
	if !ok {
		t.Skip("fixture lacks custom.xml")
	}
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = d.Close() }()
	out := saveBytes(t, d)
	gotCX, ok := zipEntry(t, out, "docProps/custom.xml")
	if !ok {
		t.Fatal("custom.xml missing after round-trip")
	}
	if !bytes.Equal(origCX, gotCX) {
		t.Errorf("custom.xml drifted\n orig: %s\n got: %s", origCX, gotCX)
	}
}
