package opc

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// Part lookup is served from a lower-cased map, with an EqualFold linear scan as
// the fallback. Those two are not the same relation: Unicode has letters whose
// simple case folding differs from ToLower, so a name can be EqualFold-equal to
// a query and still miss in the map. U+017F LATIN SMALL LETTER LONG S is the
// standard example — EqualFold("s", "ſ") is true while ToLower leaves "ſ"
// unchanged.
//
// The fallback exists for exactly that case and a full coverage pass over the
// module showed it never executing: `lookupFileLinear` and `lookupRawLinear` were
// both dead in every test. Untested case-insensitive fallbacks have already
// produced one defect in this package (C596, where RemoveOverride's fold
// fallback returned whichever map key Go's randomized iteration yielded first),
// so these get direct tests rather than being trusted.
//
// The Kelvin sign U+212A is *not* usable here, despite being the other famous
// fold-divergent letter: Go's ToLower maps it to "k", so the map lookup already
// succeeds and the fallback is never reached. Checked rather than assumed.

// foldDivergent is a part name whose ASCII spelling is EqualFold-equal to it but
// does not survive ToLower to the same key.
const foldDivergentPart = "/cuſtom/data.xml"

// asciiSpelling is what a caller would naturally ask for.
const asciiSpelling = "/custom/data.xml"

func TestFoldDivergenceAssumptionHolds(t *testing.T) {
	if !strings.EqualFold(foldDivergentPart, asciiSpelling) {
		t.Fatalf("fixture is not EqualFold-equal: %q vs %q", foldDivergentPart, asciiSpelling)
	}
	// These are the keys the index actually builds and probes. Comparing them
	// with EqualFold would defeat the purpose: the point is that ToLower — the
	// relation the map uses — does *not* bring the two names together, which is
	// the only reason the EqualFold fallback exists.
	storedKey := strings.ToLower(foldDivergentPart)
	probeKey := strings.ToLower(asciiSpelling)
	if storedKey == probeKey {
		t.Fatalf("fixture does not diverge under ToLower (both fold to %q), so it cannot reach the fallback",
			storedKey)
	}
	if !foldSensitive(foldDivergentPart) {
		t.Fatalf("foldSensitive(%q) = false, so the index would not enable its fallback", foldDivergentPart)
	}
}

// TestGetFileFallsBackToEqualFoldScan covers lookupFileLinear through the index.
func TestGetFileFallsBackToEqualFoldScan(t *testing.T) {
	r := &Reader{
		Files: []*File{
			{Name: "/word/document.xml"},
			{Name: foldDivergentPart},
		},
		index: &partIndex{},
	}

	got := r.GetFile(asciiSpelling)
	if got == nil {
		t.Fatalf("GetFile(%q) = nil; the ToLower index misses this name and the EqualFold fallback did not run",
			asciiSpelling)
	}
	if got.Name != foldDivergentPart {
		t.Errorf("GetFile(%q).Name = %q, want %q", asciiSpelling, got.Name, foldDivergentPart)
	}

	// The fallback must still report a genuine miss rather than returning
	// whatever it scanned last — the failure mode C596 was.
	if miss := r.GetFile("/nowhere/at-all.xml"); miss != nil {
		t.Errorf("GetFile of an absent part returned %q, want nil", miss.Name)
	}
}

// TestGetFileWithoutIndexUsesLinearScan covers the other entry into
// lookupFileLinear: a Reader assembled by a caller rather than by NewReader,
// which has no index at all.
func TestGetFileWithoutIndexUsesLinearScan(t *testing.T) {
	r := &Reader{Files: []*File{{Name: foldDivergentPart}}}
	if r.index != nil {
		t.Fatal("fixture has an index; this test must exercise the index-less path")
	}
	got := r.GetFile(asciiSpelling)
	if got == nil || got.Name != foldDivergentPart {
		t.Fatalf("GetFile(%q) = %v, want the %q entry", asciiSpelling, got, foldDivergentPart)
	}
	if miss := r.GetFile("/nowhere.xml"); miss != nil {
		t.Errorf("GetFile of an absent part returned %q, want nil", miss.Name)
	}
}

// TestGetRawZipFileFallsBackToEqualFoldScan covers lookupRawLinear, the same
// fallback over the raw zip entries. It goes through a real archive because the
// raw index is keyed on the zip entry names rather than on Files.
func TestGetRawZipFileFallsBackToEqualFoldScan(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	ct, err := zw.Create("[Content_Types].xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ct.Write([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="xml" ContentType="application/xml"/></Types>`)); err != nil {
		t.Fatal(err)
	}
	// Entry names in a zip carry no leading slash.
	w, err := zw.Create(strings.TrimPrefix(foldDivergentPart, "/"))
	if err != nil {
		t.Fatal(err)
	}
	const payload = "<data/>"
	if _, err := w.Write([]byte(payload)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	data := buf.Bytes()
	r, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	got, err := r.GetRawZipFile(asciiSpelling)
	if err != nil {
		t.Fatalf("GetRawZipFile(%q): %v; the ToLower index misses this name and the EqualFold fallback did not run",
			asciiSpelling, err)
	}
	if string(got) != payload {
		t.Errorf("GetRawZipFile(%q) = %q, want %q", asciiSpelling, got, payload)
	}

	if _, err := r.GetRawZipFile("/nowhere/at-all.xml"); err == nil {
		t.Error("GetRawZipFile of an absent entry returned no error, want a miss")
	}
}
