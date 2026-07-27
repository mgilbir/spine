package testutil

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// buildZip writes an in-memory archive with the given entries in order. Names
// may repeat: ZIP has no uniqueness constraint, and wild Office packages do
// carry the same part name twice.
func buildZip(t *testing.T, entries ...ZipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		fw, err := w.Create(e.Name)
		if err != nil {
			t.Fatalf("creating %s: %v", e.Name, err)
		}
		if _, err := fw.Write(e.Data); err != nil {
			t.Fatalf("writing %s: %v", e.Name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("closing zip: %v", err)
	}
	return buf.Bytes()
}

func entry(name, data string) ZipEntry { return ZipEntry{Name: name, Data: []byte(data)} }

// TestDiffZipEntriesCountsDuplicates is the C573 guard: the part comparison
// must treat entry names as a counted multiset. Each case below compares equal
// under a name-keyed map (the previous implementation), so a library that
// dropped or invented a same-named entry produced a clean "0 changed, 0
// missing, 0 extra" report.
func TestDiffZipEntriesCountsDuplicates(t *testing.T) {
	tests := []struct {
		name                            string
		orig, rt                        []ZipEntry
		wantIdentical                   int
		wantMissing, wantExtra, wantChg int
	}{
		{
			name:          "duplicate dropped",
			orig:          []ZipEntry{entry("word/document.xml", "A"), entry("word/document.xml", "B")},
			rt:            []ZipEntry{entry("word/document.xml", "B")},
			wantIdentical: 0, // "A" has no counterpart at position 0; "B" != "A"
			wantChg:       1,
			wantMissing:   1,
		},
		{
			name:          "duplicate invented",
			orig:          []ZipEntry{entry("word/document.xml", "A")},
			rt:            []ZipEntry{entry("word/document.xml", "A"), entry("word/document.xml", "A")},
			wantIdentical: 1,
			wantExtra:     1,
		},
		{
			name:          "duplicate preserved",
			orig:          []ZipEntry{entry("a.xml", "A"), entry("a.xml", "B"), entry("b.xml", "C")},
			rt:            []ZipEntry{entry("a.xml", "A"), entry("a.xml", "B"), entry("b.xml", "C")},
			wantIdentical: 3,
		},
		{
			name:          "duplicate reordered",
			orig:          []ZipEntry{entry("a.xml", "A"), entry("a.xml", "B")},
			rt:            []ZipEntry{entry("a.xml", "B"), entry("a.xml", "A")},
			wantIdentical: 0,
			wantChg:       2,
		},
		{
			name:          "plain missing and extra still reported",
			orig:          []ZipEntry{entry("a.xml", "A")},
			rt:            []ZipEntry{entry("b.xml", "B")},
			wantIdentical: 0,
			wantMissing:   1,
			wantExtra:     1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			identical, missing, extra, changed := DiffZipEntries(tc.orig, tc.rt)
			if identical != tc.wantIdentical || len(missing) != tc.wantMissing ||
				len(extra) != tc.wantExtra || len(changed) != tc.wantChg {
				t.Errorf("got identical=%d missing=%v extra=%v changed=%v;\n"+
					"want identical=%d, %d missing, %d extra, %d changed",
					identical, missing, extra, changed,
					tc.wantIdentical, tc.wantMissing, tc.wantExtra, tc.wantChg)
			}
		})
	}
}

// TestCompareZipBytesSeesDroppedDuplicate drives the same property through the
// public comparison helper the format round-trip tests use, on real archives.
func TestCompareZipBytesSeesDroppedDuplicate(t *testing.T) {
	orig := buildZip(t,
		entry("[Content_Types].xml", "<Types/>"),
		entry("word/document.xml", "<first/>"),
		entry("word/document.xml", "<second/>"),
	)
	// A save that kept only the last of the two same-named entries.
	rt := buildZip(t,
		entry("[Content_Types].xml", "<Types/>"),
		entry("word/document.xml", "<second/>"),
	)

	missing, extra, changed := CompareZipBytes(t, orig, rt)
	if len(missing) == 0 && len(changed) == 0 {
		t.Fatalf("dropped duplicate entry went unreported: missing=%v extra=%v changed=%v", missing, extra, changed)
	}
	joined := strings.Join(append(append([]string{}, missing...), changed...), " ")
	if !strings.Contains(joined, "word/document.xml") {
		t.Errorf("report does not name the affected part: missing=%v changed=%v", missing, changed)
	}
	if !strings.Contains(joined, "entry ") {
		t.Errorf("report does not identify which occurrence: missing=%v changed=%v", missing, changed)
	}
}

// TestReadZipEntriesPreservesDuplicates pins the read side: the map-returning
// helpers collapse duplicates by design, the entry-returning ones must not.
func TestReadZipEntriesPreservesDuplicates(t *testing.T) {
	data := buildZip(t, entry("a.xml", "A"), entry("a.xml", "B"))

	entries, err := ReadZipEntriesBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("ReadZipEntriesBytes returned %d entries, want 2", len(entries))
	}

	parts, err := ReadZipPartsBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 1 || string(parts["a.xml"]) != "B" {
		t.Errorf("ReadZipPartsBytes = %v, want the documented last-wins collapse", parts)
	}
}
