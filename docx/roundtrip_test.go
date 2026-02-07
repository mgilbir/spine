package docx

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// testDocxFiles contains all DOCX files to test for round-trip fidelity.
var testDocxFiles = []struct {
	name              string
	path              string
	description       string
	skipByteIdentical bool
}{
	{
		name:        "minimal",
		path:        "testdata/minimal.docx",
		description: "Minimal DOCX with basic content",
	},
}

// TestCreateAndReopen verifies that creating, saving, and reopening a document
// preserves its structural content.
func TestCreateAndReopen(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "created.docx")

	// Create a new document
	doc := Create()
	doc.AddParagraphWithText("Hello, World!")
	p2 := doc.AddParagraph()
	p2.SetStyle("Heading1")
	run := p2.AddRun()
	run.SetText("Heading Text")
	run.SetBold(true)

	if err := doc.Save(tmpFile); err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// Reopen
	doc2, err := Open(tmpFile)
	if err != nil {
		t.Fatalf("Failed to reopen: %v", err)
	}
	defer doc2.Close()

	paras := doc2.Paragraphs()
	if len(paras) != 2 {
		t.Fatalf("Expected 2 paragraphs, got %d", len(paras))
	}

	if got := paras[0].Text(); got != "Hello, World!" {
		t.Errorf("Paragraph 0 text = %q, want %q", got, "Hello, World!")
	}

	if got := paras[1].Text(); got != "Heading Text" {
		t.Errorf("Paragraph 1 text = %q, want %q", got, "Heading Text")
	}

	if got := paras[1].Style(); got != "Heading1" {
		t.Errorf("Paragraph 1 style = %q, want %q", got, "Heading1")
	}
}

// TestRoundTrip tests that opening and saving DOCX files produces valid output
// that can be re-opened and has the same structure.
func TestRoundTrip(t *testing.T) {
	for _, tc := range testDocxFiles {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := os.Stat(tc.path); os.IsNotExist(err) {
				t.Skip("Test file not found:", tc.path)
			}

			// Open the original file
			d1, err := Open(tc.path)
			if err != nil {
				t.Fatalf("Failed to open original %s: %v", tc.path, err)
			}

			// Capture original structure
			origParaCount := len(d1.Paragraphs())
			origTableCount := len(d1.Tables())
			origBody := d1.Body()

			// Save to a temp file
			tmpFile := filepath.Join(t.TempDir(), "roundtrip.docx")
			if err := d1.Save(tmpFile); err != nil {
				d1.Close()
				t.Fatalf("Failed to save: %v", err)
			}
			d1.Close()

			// Re-open the saved file to verify it's valid
			d2, err := Open(tmpFile)
			if err != nil {
				t.Fatalf("Failed to re-open saved file: %v", err)
			}
			defer d2.Close()

			// Verify structure is preserved
			if len(d2.Paragraphs()) != origParaCount {
				t.Errorf("Paragraph count changed: got %d, want %d", len(d2.Paragraphs()), origParaCount)
			}
			if len(d2.Tables()) != origTableCount {
				t.Errorf("Table count changed: got %d, want %d", len(d2.Tables()), origTableCount)
			}
			if d2.Body() != origBody {
				t.Errorf("Body text changed:\n  got:  %q\n  want: %q", d2.Body(), origBody)
			}
		})
	}
}

// TestRoundTripByteIdentical tests byte-for-byte round-trip fidelity.
// Every part in the original DOCX must appear in the round-tripped output
// with identical content.
func TestRoundTripByteIdentical(t *testing.T) {
	for _, tc := range testDocxFiles {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := os.Stat(tc.path); os.IsNotExist(err) {
				t.Skip("Test file not found:", tc.path)
			}
			if tc.skipByteIdentical {
				t.Skip("Byte-identical round-trip not expected:", tc.description)
			}

			// Open the original file
			d, err := Open(tc.path)
			if err != nil {
				t.Fatalf("Failed to open %s: %v", tc.path, err)
			}

			// Save to a temp file
			tmpFile := filepath.Join(t.TempDir(), "roundtrip.docx")
			if err := d.Save(tmpFile); err != nil {
				d.Close()
				t.Fatalf("Failed to save: %v", err)
			}
			d.Close()

			// Compare the two files
			missing, extra, changed := compareDocxFiles(t, tc.path, tmpFile)

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
				origParts, _ := readDocxParts(tc.path)
				rtParts, _ := readDocxParts(tmpFile)
				t.Errorf("%d changed parts:", len(changed))
				for _, name := range changed {
					origSize := len(origParts[name])
					rtSize := len(rtParts[name])
					t.Errorf("  CHANGED: %s (%d -> %d bytes)", name, origSize, rtSize)
					showDocxDiff(t, name, origParts[name], rtParts[name])
				}
			}
		})
	}
}

// compareDocxFiles compares two DOCX files and returns lists of missing, extra, and changed parts.
func compareDocxFiles(t *testing.T, original, roundtrip string) (missing, extra, changed []string) {
	t.Helper()

	origParts, err := readDocxParts(original)
	if err != nil {
		t.Fatalf("Failed to read original: %v", err)
	}

	rtParts, err := readDocxParts(roundtrip)
	if err != nil {
		t.Fatalf("Failed to read roundtrip: %v", err)
	}

	for _, name := range sortedDocxKeys(origParts) {
		if _, ok := rtParts[name]; !ok {
			missing = append(missing, name)
		}
	}

	for _, name := range sortedDocxKeys(rtParts) {
		if _, ok := origParts[name]; !ok {
			extra = append(extra, name)
		}
	}

	for _, name := range sortedDocxKeys(origParts) {
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

// readDocxParts reads all parts from a DOCX file into a map.
func readDocxParts(path string) (map[string][]byte, error) {
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
		name := f.Name
		if len(name) > 0 && name[0] == '/' {
			name = name[1:]
		}
		parts[name] = buf.Bytes()
	}
	return parts, nil
}

func sortedDocxKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func showDocxDiff(t *testing.T, name string, orig, rt []byte) {
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
