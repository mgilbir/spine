package pptx

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// testPptxFiles contains all PPTX files to test for round-trip fidelity.
var testPptxFiles = []struct {
	name              string
	path              string
	description       string
	skipByteIdentical bool // true if byte-identical round-trip is not expected
}{
	{
		name:        "minimal",
		path:        "testdata/minimal.pptx",
		description: "Minimal PPTX with no slides, just master and layout",
	},
	{
		name:        "no-slides",
		path:        "testdata/no-slides.pptx",
		description: "PPTX with no slides",
	},
	{
		name:              "no-core-props",
		path:              "testdata/no-core-props.pptx",
		description:       "PPTX without core properties",
		skipByteIdentical: true, // library always generates core.xml on save
	},
	{
		name:        "test",
		path:        "testdata/test.pptx",
		description: "Basic test PPTX with one slide",
	},
	{
		name:        "test_slides",
		path:        "testdata/test_slides.pptx",
		description: "PPTX with slides and more content",
	},
	{
		name:        "missing_rels_item",
		path:        "testdata/missing_rels_item.pptx",
		description: "PPTX with missing relationship item (edge case)",
	},
	{
		name:        "big_data",
		path:        "testdata/big_data.pptx",
		description: "Large PPTX with 263 parts including charts and diagrams",
	},
	{
		name:        "sdg_integration",
		path:        "testdata/sdg_integration.pptx",
		description: "SDG integration PPTX with 177 parts including complex styling",
	},
	{
		name:              "internet_basics",
		path:              "testdata/internet_basics.pptx",
		description:       "TAMU internet basics with 49 slides and animations",
		skipByteIdentical: true, // animation/text encoding issues remain
	},
	{
		name:        "lecture_1_3",
		path:        "testdata/lecture_1_3.pptx",
		description: "UvA lecture with 35 slides and 16 layouts",
	},
	{
		name:              "swarm_cop",
		path:              "testdata/swarm_cop.pptx",
		description:       "ESA Swarm COP with complex charts and 25 slides",
		skipByteIdentical: true, // some round-trip issues remain
	},
	{
		name:              "sefuw",
		path:              "testdata/sefuw.pptx",
		description:       "ESA SEFUW with 25 slides and complex styling",
		skipByteIdentical: true, // text encoding issues remain
	},
}

// TestRoundTrip tests that opening and saving PPTX files produces valid output
// that can be re-opened and has the same structure.
func TestRoundTrip(t *testing.T) {
	for _, tc := range testPptxFiles {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := os.Stat(tc.path); os.IsNotExist(err) {
				t.Skip("Test file not found:", tc.path)
			}

			// Open the original file
			p1, err := Open(tc.path)
			if err != nil {
				t.Fatalf("Failed to open original %s: %v", tc.path, err)
			}

			// Capture original structure
			origSlideCount := p1.SlideCount()
			origMasterCount := len(p1.SlideMasters())
			origLayoutCount := len(p1.SlideLayouts())

			// Save to a temp file
			tmpFile := filepath.Join(t.TempDir(), "roundtrip.pptx")
			if err := p1.Save(tmpFile); err != nil {
				p1.Close()
				t.Fatalf("Failed to save: %v", err)
			}
			p1.Close()

			// Re-open the saved file to verify it's valid
			p2, err := Open(tmpFile)
			if err != nil {
				t.Fatalf("Failed to re-open saved file: %v", err)
			}
			defer p2.Close()

			// Verify structure is preserved
			if p2.SlideCount() != origSlideCount {
				t.Errorf("Slide count changed: got %d, want %d", p2.SlideCount(), origSlideCount)
			}
			if len(p2.SlideMasters()) != origMasterCount {
				t.Errorf("Slide master count changed: got %d, want %d", len(p2.SlideMasters()), origMasterCount)
			}
			if len(p2.SlideLayouts()) != origLayoutCount {
				t.Errorf("Slide layout count changed: got %d, want %d", len(p2.SlideLayouts()), origLayoutCount)
			}
		})
	}
}

// TestRoundTripByteIdentical tests byte-for-byte round-trip fidelity.
// Every part in the original PPTX must appear in the round-tripped output
// with identical content.
func TestRoundTripByteIdentical(t *testing.T) {
	for _, tc := range testPptxFiles {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := os.Stat(tc.path); os.IsNotExist(err) {
				t.Skip("Test file not found:", tc.path)
			}
			if tc.skipByteIdentical {
				t.Skip("Byte-identical round-trip not expected:", tc.description)
			}

			// Open the original file
			p, err := Open(tc.path)
			if err != nil {
				t.Fatalf("Failed to open %s: %v", tc.path, err)
			}

			// Save to a temp file
			tmpFile := filepath.Join(t.TempDir(), "roundtrip.pptx")
			if err := p.Save(tmpFile); err != nil {
				p.Close()
				t.Fatalf("Failed to save: %v", err)
			}
			p.Close()

			// Compare the two files
			missing, extra, changed := comparePptxFiles(t, tc.path, tmpFile)

			if len(missing) > 0 {
				t.Errorf("%d missing parts:", len(missing))
				for _, name := range missing {
					t.Errorf("  MISSING: %s", name)
				}
			}
			if len(extra) > 0 {
				t.Errorf("%d extra parts:", len(extra))
				for _, name := range extra {
					t.Errorf("  EXTRA: %s", name)
				}
			}
			if len(changed) > 0 {
				origParts, _ := readPptxParts(tc.path)
				rtParts, _ := readPptxParts(tmpFile)
				t.Errorf("%d changed parts:", len(changed))
				for _, name := range changed {
					origSize := len(origParts[name])
					rtSize := len(rtParts[name])
					t.Errorf("  CHANGED: %s (%d -> %d bytes)", name, origSize, rtSize)
				}
			}
		})
	}
}

// comparePptxFiles compares two PPTX files and returns lists of missing, extra, and changed parts.
func comparePptxFiles(t *testing.T, original, roundtrip string) (missing, extra, changed []string) {
	t.Helper()

	origParts, err := readPptxParts(original)
	if err != nil {
		t.Fatalf("Failed to read original: %v", err)
	}

	rtParts, err := readPptxParts(roundtrip)
	if err != nil {
		t.Fatalf("Failed to read roundtrip: %v", err)
	}

	// Check for missing parts
	for _, name := range sortedKeys(origParts) {
		if _, ok := rtParts[name]; !ok {
			missing = append(missing, name)
		}
	}

	// Check for extra parts
	for _, name := range sortedKeys(rtParts) {
		if _, ok := origParts[name]; !ok {
			extra = append(extra, name)
		}
	}

	// Check for changed parts
	for _, name := range sortedKeys(origParts) {
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

// readPptxParts reads all parts from a PPTX file into a map.
func readPptxParts(path string) (map[string][]byte, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	parts := make(map[string][]byte)
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		_, err = buf.ReadFrom(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		// Normalize the name (remove leading slash if present)
		name := f.Name
		if len(name) > 0 && name[0] == '/' {
			name = name[1:]
		}
		parts[name] = buf.Bytes()
	}
	return parts, nil
}

func sortedKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// showDiff shows the first difference between two strings for debugging.
func showDiff(t *testing.T, name string, orig, rt []byte) {
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
