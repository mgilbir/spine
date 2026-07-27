// Package testutil provides shared test helpers for round-trip testing
// of Office documents (PPTX, DOCX, XLSX).
package testutil

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"testing"
)

// ZipEntry is one archive entry. A ZIP file may legally carry several entries
// under the same name, so the fidelity comparison works on a list of entries
// rather than a name-keyed map: collapsing duplicates would make a library that
// drops or duplicates one of them indistinguishable from a faithful one. (C573)
type ZipEntry struct {
	Name string
	Data []byte
}

// ReadZipParts reads all parts from a ZIP file (PPTX/DOCX/XLSX) into a map.
// Part names have leading slashes stripped for consistency. Duplicate entry
// names collapse (last wins) — use ReadZipEntries when duplicates matter.
func ReadZipParts(path string) (map[string][]byte, error) {
	entries, err := ReadZipEntries(path)
	if err != nil {
		return nil, err
	}
	return partsMap(entries), nil
}

// ReadZipPartsBytes reads all parts from an in-memory ZIP archive into a map.
// Part names have leading slashes stripped for consistency. Duplicate entry
// names collapse (last wins) — use ReadZipEntriesBytes when duplicates matter.
func ReadZipPartsBytes(data []byte) (map[string][]byte, error) {
	entries, err := ReadZipEntriesBytes(data)
	if err != nil {
		return nil, err
	}
	return partsMap(entries), nil
}

// ReadZipEntries reads every entry of a ZIP file in archive order, preserving
// entries that share a name. Names have leading slashes stripped.
func ReadZipEntries(path string) ([]ZipEntry, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()
	return readZipEntries(r.File)
}

// ReadZipEntriesBytes reads every entry of an in-memory ZIP archive in archive
// order, preserving entries that share a name.
func ReadZipEntriesBytes(data []byte) ([]ZipEntry, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	return readZipEntries(r.File)
}

func readZipEntries(files []*zip.File) ([]ZipEntry, error) {
	entries := make([]ZipEntry, 0, len(files))
	for _, f := range files {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		_, err = buf.ReadFrom(rc)
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		name := f.Name
		if len(name) > 0 && name[0] == '/' {
			name = name[1:]
		}
		entries = append(entries, ZipEntry{Name: name, Data: buf.Bytes()})
	}
	return entries, nil
}

func partsMap(entries []ZipEntry) map[string][]byte {
	parts := make(map[string][]byte, len(entries))
	for _, e := range entries {
		parts[e.Name] = e.Data
	}
	return parts
}

// AppendZipEntry rewrites the ZIP file at path into an in-memory archive with
// one extra entry appended. It is used to craft fixtures reproducing
// real-world packages that carry junk entries (e.g. [trash]/0000.dat) without
// a matching [Content_Types].xml entry.
func AppendZipEntry(t *testing.T, path, name string, content []byte) []byte {
	t.Helper()

	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("Failed to open %s: %v", path, err)
	}
	defer func() { _ = r.Close() }()

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("Failed to open entry %s: %v", f.Name, err)
		}
		fw, err := w.Create(f.Name)
		if err != nil {
			t.Fatalf("Failed to create entry %s: %v", f.Name, err)
		}
		if _, err := io.Copy(fw, rc); err != nil {
			t.Fatalf("Failed to copy entry %s: %v", f.Name, err)
		}
		_ = rc.Close()
	}
	fw, err := w.Create(name)
	if err != nil {
		t.Fatalf("Failed to create entry %s: %v", name, err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("Failed to write entry %s: %v", name, err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close zip: %v", err)
	}
	return buf.Bytes()
}

// CompareZipBytes compares two in-memory ZIP archives and returns lists of
// missing, extra, and changed parts.
func CompareZipBytes(t *testing.T, original, roundtrip []byte) (missing, extra, changed []string) {
	t.Helper()

	origEntries, err := ReadZipEntriesBytes(original)
	if err != nil {
		t.Fatalf("Failed to read original: %v", err)
	}

	rtEntries, err := ReadZipEntriesBytes(roundtrip)
	if err != nil {
		t.Fatalf("Failed to read roundtrip: %v", err)
	}

	_, missing, extra, changed = DiffZipEntries(origEntries, rtEntries)
	return missing, extra, changed
}

// CompareZipFiles compares two ZIP files and returns lists of missing, extra, and changed parts.
func CompareZipFiles(t *testing.T, original, roundtrip string) (missing, extra, changed []string) {
	t.Helper()

	origEntries, err := ReadZipEntries(original)
	if err != nil {
		t.Fatalf("Failed to read original: %v", err)
	}

	rtEntries, err := ReadZipEntries(roundtrip)
	if err != nil {
		t.Fatalf("Failed to read roundtrip: %v", err)
	}

	_, missing, extra, changed = DiffZipEntries(origEntries, rtEntries)
	return missing, extra, changed
}

// DiffZipEntries compares two archives as name-counted multisets and reports
// how many entries came through byte-identical plus the missing, extra, and
// changed ones. Entries sharing a name are matched pairwise in archive order,
// so an archive carrying a name twice is only satisfied by an output that also
// carries it twice with the same bytes in the same order; the surplus or
// shortfall is reported as extra/missing occurrences of that name.
//
// A name-keyed map cannot express that: it silently keeps one entry per name on
// both sides, which makes dropping or duplicating a same-named entry invisible
// to the fidelity accounting (C573).
func DiffZipEntries(orig, rt []ZipEntry) (identical int, missing, extra, changed []string) {
	og, oNames := groupZipEntries(orig)
	rg, rNames := groupZipEntries(rt)

	for _, name := range oNames {
		o, r := og[name], rg[name]
		n := min(len(o), len(r))
		for i := 0; i < n; i++ {
			if bytes.Equal(o[i], r[i]) {
				identical++
			} else {
				changed = append(changed, occurrenceLabel(name, i, len(o)))
			}
		}
		for i := n; i < len(o); i++ {
			missing = append(missing, occurrenceLabel(name, i, len(o)))
		}
	}

	for _, name := range rNames {
		o, r := og[name], rg[name]
		for i := len(o); i < len(r); i++ {
			extra = append(extra, occurrenceLabel(name, i, len(r)))
		}
	}

	return identical, missing, extra, changed
}

// groupZipEntries buckets entries by name, preserving archive order within a
// bucket, and returns the sorted distinct names.
func groupZipEntries(entries []ZipEntry) (map[string][][]byte, []string) {
	g := make(map[string][][]byte, len(entries))
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if _, seen := g[e.Name]; !seen {
			names = append(names, e.Name)
		}
		g[e.Name] = append(g[e.Name], e.Data)
	}
	sort.Strings(names)
	return g, names
}

// occurrenceLabel names an entry, disambiguating which occurrence is meant when
// the archive carries the name more than once.
func occurrenceLabel(name string, i, total int) string {
	if total < 2 {
		return name
	}
	return fmt.Sprintf("%s (entry %d of %d)", name, i+1, total)
}

// SortedKeys returns the keys of a map sorted alphabetically.
func SortedKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ShowDiff shows the first difference between two byte slices for debugging.
// Only shows diffs for XML and .rels files.
func ShowDiff(t *testing.T, name string, orig, rt []byte) {
	t.Helper()
	if !strings.HasSuffix(name, ".xml") && !strings.HasSuffix(name, ".rels") {
		return
	}
	origStr := string(orig)
	rtStr := string(rt)
	minLen := len(origStr)
	if len(rtStr) < minLen {
		minLen = len(rtStr)
	}
	diffPos := -1
	for i := 0; i < minLen; i++ {
		if origStr[i] != rtStr[i] {
			diffPos = i
			break
		}
	}
	if diffPos == -1 && len(origStr) != len(rtStr) {
		diffPos = minLen
	}
	if diffPos >= 0 {
		start := diffPos - 80
		if start < 0 {
			start = 0
		}
		end := diffPos + 80
		origEnd := end
		rtEnd := end
		if origEnd > len(origStr) {
			origEnd = len(origStr)
		}
		if rtEnd > len(rtStr) {
			rtEnd = len(rtStr)
		}
		t.Logf("  %s (diff at byte %d):", name, diffPos)
		t.Logf("    ORIG: ...%s...", origStr[start:origEnd])
		t.Logf("    RT:   ...%s...", rtStr[start:rtEnd])
	}
}
