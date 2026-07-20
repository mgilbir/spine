package xlsx

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"testing"
	"time"
)

func cpPart(t *testing.T, data []byte, name string) ([]byte, bool) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, zf := range zr.File {
		if zf.Name == name {
			rc, err := zf.Open()
			if err != nil {
				t.Fatal(err)
			}
			var b bytes.Buffer
			_, _ = io.Copy(&b, rc)
			_ = rc.Close()
			return b.Bytes(), true
		}
	}
	return nil, false
}

func cpSaveWB(t *testing.T, w *Workbook) []byte {
	t.Helper()
	data, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	return data
}

// TestXlsxCustomPropertiesCreateRoundTrip sets each value type on a created
// workbook, saves, reopens, and reads the values back.
func TestXlsxCustomPropertiesCreateRoundTrip(t *testing.T) {
	w := Create()
	w.AddSheet("Sheet1")
	tm := time.Date(2022, 1, 2, 3, 4, 5, 0, time.UTC)
	vals := map[string]any{"Str": "s & t", "Int": int64(11), "Float": 1.25, "Bool": false, "Date": tm}
	for k, v := range vals {
		if err := w.SetCustomProperty(k, v); err != nil {
			t.Fatalf("SetCustomProperty(%q): %v", k, err)
		}
	}
	out := cpSaveWB(t, w)

	if _, ok := cpPart(t, out, "docProps/custom.xml"); !ok {
		t.Fatal("docProps/custom.xml not written")
	}
	if ct, _ := cpPart(t, out, "[Content_Types].xml"); !bytes.Contains(ct, []byte("custom-properties+xml")) {
		t.Error("content-type override missing")
	}
	if rels, _ := cpPart(t, out, "_rels/.rels"); !bytes.Contains(rels, []byte("docProps/custom.xml")) {
		t.Error("package relationship missing")
	}

	w2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w2.Close() }()
	m := w2.CustomProperties()
	if m["Str"] != "s & t" || m["Int"] != int64(11) || m["Float"] != 1.25 || m["Bool"] != false {
		t.Errorf("values = %v", m)
	}
	if dt, _ := m["Date"].(time.Time); !dt.Equal(tm) {
		t.Errorf("Date = %v", m["Date"])
	}
}

// TestXlsxCustomPropertiesPreservedVerbatim checks an untouched round-trip
// reproduces docProps/custom.xml byte-for-byte.
func TestXlsxCustomPropertiesPreservedVerbatim(t *testing.T) {
	w := Create()
	w.AddSheet("Sheet1")
	_ = w.SetCustomProperty("Keep", "value")
	first := cpSaveWB(t, w)
	firstCX, ok := cpPart(t, first, "docProps/custom.xml")
	if !ok {
		t.Fatal("no custom.xml in first save")
	}
	w2, err := OpenReader(bytes.NewReader(first), int64(len(first)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w2.Close() }()
	second := cpSaveWB(t, w2)
	secondCX, _ := cpPart(t, second, "docProps/custom.xml")
	if !bytes.Equal(firstCX, secondCX) {
		t.Errorf("custom.xml drifted\n first: %s\nsecond: %s", firstCX, secondCX)
	}
}

// TestXlsxCustomPropertiesModifyAndRemove edits and removes properties on a
// reopened workbook.
func TestXlsxCustomPropertiesModifyAndRemove(t *testing.T) {
	w := Create()
	w.AddSheet("Sheet1")
	_ = w.SetCustomProperty("A", "one")
	_ = w.SetCustomProperty("B", "two")
	first := cpSaveWB(t, w)

	w2, err := OpenReader(bytes.NewReader(first), int64(len(first)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w2.Close() }()
	_ = w2.SetCustomProperty("A", "updated")
	if !w2.RemoveCustomProperty("B") {
		t.Fatal("RemoveCustomProperty(B) = false")
	}
	out := cpSaveWB(t, w2)

	w3, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w3.Close() }()
	m := w3.CustomProperties()
	if m["A"] != "updated" {
		t.Errorf("A = %v", m["A"])
	}
	if _, ok := m["B"]; ok {
		t.Error("B still present")
	}
}

// TestXlsxCustomPropertiesAddOnRoundTrip opens a workbook that has no
// docProps/custom.xml, adds a property, and verifies the part, content-type
// override, and package relationship are created.
func TestXlsxCustomPropertiesAddOnRoundTrip(t *testing.T) {
	w := Create()
	w.AddSheet("Sheet1")
	base := cpSaveWB(t, w)
	if _, ok := cpPart(t, base, "docProps/custom.xml"); ok {
		t.Skip("baseline unexpectedly has custom.xml")
	}
	w2, err := OpenReader(bytes.NewReader(base), int64(len(base)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w2.Close() }()
	if err := w2.SetCustomProperty("Added", "later"); err != nil {
		t.Fatal(err)
	}
	out := cpSaveWB(t, w2)
	if _, ok := cpPart(t, out, "docProps/custom.xml"); !ok {
		t.Fatal("custom.xml not created")
	}
	if ct, _ := cpPart(t, out, "[Content_Types].xml"); !bytes.Contains(ct, []byte("custom-properties+xml")) {
		t.Error("content-type override missing")
	}
	if rels, _ := cpPart(t, out, "_rels/.rels"); !bytes.Contains(rels, []byte("docProps/custom.xml")) {
		t.Error("package relationship not injected")
	}
	w3, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w3.Close() }()
	if v, _ := w3.CustomProperties()["Added"].(string); v != "later" {
		t.Errorf("Added = %q, want later", v)
	}
}

// TestXlsxCustomPropertiesCorpusByteIdentical round-trips a real workbook with
// custom properties; skipped when the corpus is unavailable.
func TestXlsxCustomPropertiesCorpusByteIdentical(t *testing.T) {
	const path = "../testdata/corpus/cc/xlsx/665382ef887e8f4b.xlsx"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skip("corpus file not available:", path)
	}
	origCX, ok := cpPart(t, raw, "docProps/custom.xml")
	if !ok {
		t.Skip("fixture lacks custom.xml")
	}
	w, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	out := cpSaveWB(t, w)
	gotCX, ok := cpPart(t, out, "docProps/custom.xml")
	if !ok {
		t.Fatal("custom.xml missing after round-trip")
	}
	if !bytes.Equal(origCX, gotCX) {
		t.Errorf("custom.xml drifted\n orig: %s\n got: %s", origCX, gotCX)
	}
}
