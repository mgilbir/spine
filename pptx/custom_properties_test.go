package pptx

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

func cpSavePres(t *testing.T, p *Presentation) []byte {
	t.Helper()
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	return data
}

// TestPptxCustomPropertiesCreateRoundTrip sets each value type on a created
// presentation, saves, reopens, and reads the values back.
func TestPptxCustomPropertiesCreateRoundTrip(t *testing.T) {
	p := Create()
	tm := time.Date(2022, 1, 2, 3, 4, 5, 0, time.UTC)
	vals := map[string]any{"Str": "s & t", "Int": int64(11), "Float": 1.25, "Bool": true, "Date": tm}
	for k, v := range vals {
		if err := p.SetCustomProperty(k, v); err != nil {
			t.Fatalf("SetCustomProperty(%q): %v", k, err)
		}
	}
	out := cpSavePres(t, p)

	if _, ok := cpPart(t, out, "docProps/custom.xml"); !ok {
		t.Fatal("docProps/custom.xml not written")
	}
	if ct, _ := cpPart(t, out, "[Content_Types].xml"); !bytes.Contains(ct, []byte("custom-properties+xml")) {
		t.Error("content-type override missing")
	}
	if rels, _ := cpPart(t, out, "_rels/.rels"); !bytes.Contains(rels, []byte("docProps/custom.xml")) {
		t.Error("package relationship missing")
	}

	p2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p2.Close() }()
	m := p2.CustomProperties()
	if m["Str"] != "s & t" || m["Int"] != int64(11) || m["Float"] != 1.25 || m["Bool"] != true {
		t.Errorf("values = %v", m)
	}
	if dt, _ := m["Date"].(time.Time); !dt.Equal(tm) {
		t.Errorf("Date = %v", m["Date"])
	}
}

// TestPptxCustomPropertiesPreservedVerbatim checks an untouched round-trip
// reproduces docProps/custom.xml byte-for-byte.
func TestPptxCustomPropertiesPreservedVerbatim(t *testing.T) {
	p := Create()
	_ = p.SetCustomProperty("Keep", "value")
	first := cpSavePres(t, p)
	firstCX, ok := cpPart(t, first, "docProps/custom.xml")
	if !ok {
		t.Fatal("no custom.xml in first save")
	}
	p2, err := OpenReader(bytes.NewReader(first), int64(len(first)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p2.Close() }()
	second := cpSavePres(t, p2)
	secondCX, _ := cpPart(t, second, "docProps/custom.xml")
	if !bytes.Equal(firstCX, secondCX) {
		t.Errorf("custom.xml drifted\n first: %s\nsecond: %s", firstCX, secondCX)
	}
}

// TestPptxCustomPropertiesAddOnRoundTrip opens a presentation that has no
// docProps/custom.xml, adds a property, and verifies the part, content-type
// override, and package relationship are created (exercising injection into the
// preserved root _rels/.rels).
func TestPptxCustomPropertiesAddOnRoundTrip(t *testing.T) {
	base := cpSavePres(t, Create())
	if _, ok := cpPart(t, base, "docProps/custom.xml"); ok {
		t.Skip("baseline unexpectedly has custom.xml")
	}
	p, err := OpenReader(bytes.NewReader(base), int64(len(base)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	if err := p.SetCustomProperty("Added", "later"); err != nil {
		t.Fatal(err)
	}
	out := cpSavePres(t, p)
	if _, ok := cpPart(t, out, "docProps/custom.xml"); !ok {
		t.Fatal("custom.xml not created")
	}
	if ct, _ := cpPart(t, out, "[Content_Types].xml"); !bytes.Contains(ct, []byte("custom-properties+xml")) {
		t.Error("content-type override missing")
	}
	if rels, _ := cpPart(t, out, "_rels/.rels"); !bytes.Contains(rels, []byte("docProps/custom.xml")) {
		t.Error("package relationship not injected")
	}
	p2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p2.Close() }()
	if v, _ := p2.CustomProperties()["Added"].(string); v != "later" {
		t.Errorf("Added = %q, want later", v)
	}
}

// TestPptxCustomPropertiesCorpusByteIdentical round-trips a real presentation
// with custom properties, and also removes one; skipped without the corpus.
func TestPptxCustomPropertiesCorpusByteIdentical(t *testing.T) {
	const path = "../testdata/corpus/cc/pptx/e49a54e590feff70.pptx"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skip("corpus file not available:", path)
	}
	origCX, ok := cpPart(t, raw, "docProps/custom.xml")
	if !ok {
		t.Skip("fixture lacks custom.xml")
	}
	p, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	out := cpSavePres(t, p)
	gotCX, ok := cpPart(t, out, "docProps/custom.xml")
	if !ok {
		t.Fatal("custom.xml missing after round-trip")
	}
	if !bytes.Equal(origCX, gotCX) {
		t.Errorf("custom.xml drifted\n orig: %s\n got: %s", origCX, gotCX)
	}

	// Removing a property regenerates the part and drops it on reopen.
	p2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p2.Close() }()
	names := p2.CustomProperties()
	var name string
	for k := range names {
		name = k
		break
	}
	if !p2.RemoveCustomProperty(name) {
		t.Fatalf("RemoveCustomProperty(%q) = false", name)
	}
	out2 := cpSavePres(t, p2)
	p3, err := OpenReader(bytes.NewReader(out2), int64(len(out2)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p3.Close() }()
	if _, ok := p3.CustomProperties()[name]; ok {
		t.Errorf("%q still present after removal", name)
	}
}
