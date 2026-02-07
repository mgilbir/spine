package docx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mgilbir/spine/internal/testutil"
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
	defer func() { _ = doc2.Close() }()

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
				_ = d1.Close()
				t.Fatalf("Failed to save: %v", err)
			}
			_ = d1.Close()

			// Re-open the saved file to verify it's valid
			d2, err := Open(tmpFile)
			if err != nil {
				t.Fatalf("Failed to re-open saved file: %v", err)
			}
			defer func() { _ = d2.Close() }()

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
				_ = d.Close()
				t.Fatalf("Failed to save: %v", err)
			}
			_ = d.Close()

			// Compare the two files
			missing, extra, changed := testutil.CompareZipFiles(t, tc.path, tmpFile)

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
				origParts, _ := testutil.ReadZipParts(tc.path)
				rtParts, _ := testutil.ReadZipParts(tmpFile)
				t.Errorf("%d changed parts:", len(changed))
				for _, name := range changed {
					origSize := len(origParts[name])
					rtSize := len(rtParts[name])
					t.Errorf("  CHANGED: %s (%d -> %d bytes)", name, origSize, rtSize)
					testutil.ShowDiff(t, name, origParts[name], rtParts[name])
				}
			}
		})
	}
}
