package pptx

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/mgilbir/spine/internal/testutil"
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
		name:        "svg_test",
		path:        "testdata/svg_test.pptx",
		description: "PPTX with embedded SVG images",
	},
	{
		name:        "big_data",
		path:        "testdata/external/big_data.pptx",
		description: "Large PPTX with 263 parts including charts and diagrams",
	},
	{
		name:        "sdg_integration",
		path:        "testdata/external/sdg_integration.pptx",
		description: "SDG integration PPTX with 177 parts including complex styling",
	},
	{
		name:        "internet_basics",
		path:        "testdata/external/internet_basics.pptx",
		description: "TAMU internet basics with 49 slides and animations",
	},
	{
		name:        "lecture_1_3",
		path:        "testdata/external/lecture_1_3.pptx",
		description: "UvA lecture with 35 slides and 16 layouts",
	},
	{
		name:        "swarm_cop",
		path:        "testdata/external/swarm_cop.pptx",
		description: "ESA Swarm COP with complex charts and 25 slides",
	},
	{
		name:        "sefuw",
		path:        "testdata/external/sefuw.pptx",
		description: "ESA SEFUW with 25 slides and complex styling",
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
				_ = p1.Close()
				t.Fatalf("Failed to save: %v", err)
			}
			_ = p1.Close()

			// Re-open the saved file to verify it's valid
			p2, err := Open(tmpFile)
			if err != nil {
				t.Fatalf("Failed to re-open saved file: %v", err)
			}
			defer func() { _ = p2.Close() }()

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
				_ = p.Close()
				t.Fatalf("Failed to save: %v", err)
			}
			_ = p.Close()

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
				}
			}
		})
	}
}

func TestSaveBytesAndOpenReader(t *testing.T) {
	p := Create()
	slide := p.AddSlide()
	box := slide.AddTextBox()
	box.TextFrame().SetText("Hello, world!")

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes error: %v", err)
	}

	reopened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader error: %v", err)
	}
	defer func() { _ = reopened.Close() }()

	if reopened.SlideCount() != 1 {
		t.Fatalf("SlideCount = %d, want 1", reopened.SlideCount())
	}
	textBox, ok := reopened.Slides()[0].Shapes()[0].(*TextBox)
	if !ok {
		t.Fatalf("shape type = %T, want *TextBox", reopened.Slides()[0].Shapes()[0])
	}
	if got := textBox.Text(); got != "Hello, world!" {
		t.Fatalf("slide text = %q, want %q", got, "Hello, world!")
	}
}
