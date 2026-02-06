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
	name        string
	path        string
	description string
}{
	{
		name:        "minimal",
		path:        "../python-tests/test_files/minimal.pptx",
		description: "Minimal PPTX with no slides, just master and layout",
	},
	{
		name:        "no-slides",
		path:        "../python-tests/test_files/no-slides.pptx",
		description: "PPTX with no slides",
	},
	{
		name:        "no-core-props",
		path:        "../python-tests/test_files/no-core-props.pptx",
		description: "PPTX without core properties",
	},
	{
		name:        "test",
		path:        "../python-tests/test_files/test.pptx",
		description: "Basic test PPTX with one slide",
	},
	{
		name:        "test_slides",
		path:        "../python-tests/test_files/test_slides.pptx",
		description: "PPTX with slides and more content",
	},
	{
		name:        "missing_rels_item",
		path:        "../python-tests/test_files/missing_rels_item.pptx",
		description: "PPTX with missing relationship item (edge case)",
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
// This is a stricter test that may be useful for debugging serialization issues.
func TestRoundTripByteIdentical(t *testing.T) {
	t.Skip("Byte-identical round-trip not implemented - using model-based serialization")

	for _, tc := range testPptxFiles {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := os.Stat(tc.path); os.IsNotExist(err) {
				t.Skip("Test file not found:", tc.path)
			}

			// Open the original file
			p, err := Open(tc.path)
			if err != nil {
				t.Fatalf("Failed to open %s: %v", tc.path, err)
			}
			defer p.Close()

			// Save to a temp file
			tmpFile := filepath.Join(t.TempDir(), "roundtrip.pptx")
			if err := p.Save(tmpFile); err != nil {
				t.Fatalf("Failed to save: %v", err)
			}

			// Compare the two files
			diffs := comparePptxFiles(t, tc.path, tmpFile)
			if len(diffs) > 0 {
				t.Errorf("Round-trip produced %d differences for %s:", len(diffs), tc.description)
				for _, diff := range diffs {
					t.Errorf("  %s", diff)
				}
			}
		})
	}
}

// comparePptxFiles compares two PPTX files and returns a list of differences.
func comparePptxFiles(t *testing.T, original, roundtrip string) []string {
	t.Helper()

	origParts, err := readPptxParts(original)
	if err != nil {
		t.Fatalf("Failed to read original: %v", err)
	}

	rtParts, err := readPptxParts(roundtrip)
	if err != nil {
		t.Fatalf("Failed to read roundtrip: %v", err)
	}

	var diffs []string

	// Check for missing or extra files
	origNames := sortedKeys(origParts)
	rtNames := sortedKeys(rtParts)

	for _, name := range origNames {
		if _, ok := rtParts[name]; !ok {
			diffs = append(diffs, "MISSING: "+name)
		}
	}
	for _, name := range rtNames {
		if _, ok := origParts[name]; !ok {
			diffs = append(diffs, "EXTRA: "+name)
		}
	}

	// Compare content of shared files
	for name, origContent := range origParts {
		rtContent, ok := rtParts[name]
		if !ok {
			continue // Already reported as missing
		}

		if !bytes.Equal(origContent, rtContent) {
			// For XML files, do a more detailed comparison
			if strings.HasSuffix(name, ".xml") || strings.HasSuffix(name, ".rels") {
				diffs = append(diffs, "CHANGED: "+name+" (XML content differs)")
			} else {
				diffs = append(diffs, "CHANGED: "+name+" (binary content differs)")
			}
		}
	}

	return diffs
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
