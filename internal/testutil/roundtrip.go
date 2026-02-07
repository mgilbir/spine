// Package testutil provides shared test helpers for round-trip testing
// of Office documents (PPTX, DOCX, XLSX).
package testutil

import (
	"archive/zip"
	"bytes"
	"sort"
	"strings"
	"testing"
)

// ReadZipParts reads all parts from a ZIP file (PPTX/DOCX/XLSX) into a map.
// Part names have leading slashes stripped for consistency.
func ReadZipParts(path string) (map[string][]byte, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()

	parts := make(map[string][]byte)
	for _, f := range r.File {
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
		parts[name] = buf.Bytes()
	}
	return parts, nil
}

// CompareZipFiles compares two ZIP files and returns lists of missing, extra, and changed parts.
func CompareZipFiles(t *testing.T, original, roundtrip string) (missing, extra, changed []string) {
	t.Helper()

	origParts, err := ReadZipParts(original)
	if err != nil {
		t.Fatalf("Failed to read original: %v", err)
	}

	rtParts, err := ReadZipParts(roundtrip)
	if err != nil {
		t.Fatalf("Failed to read roundtrip: %v", err)
	}

	for _, name := range SortedKeys(origParts) {
		if _, ok := rtParts[name]; !ok {
			missing = append(missing, name)
		}
	}

	for _, name := range SortedKeys(rtParts) {
		if _, ok := origParts[name]; !ok {
			extra = append(extra, name)
		}
	}

	for _, name := range SortedKeys(origParts) {
		rtContent, ok := rtParts[name]
		if !ok {
			continue
		}
		if !bytes.Equal(origParts[name], rtContent) {
			changed = append(changed, name)
		}
	}

	return
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
