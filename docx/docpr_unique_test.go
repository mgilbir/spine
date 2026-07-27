package docx

import (
	"regexp"
	"testing"

	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/opc"
)

// TestDocPrIDsUniqueAcrossImageAndChart verifies that an image and a chart added
// to the same document get distinct wp:docPr ids (C295). Both used to derive the
// id from their own part number, so each emitted <wp:docPr id="1">, which ECMA
// forbids (docPr ids must be document-unique).
func TestDocPrIDsUniqueAcrossImageAndChart(t *testing.T) {
	doc := Create()
	if _, err := doc.AddParagraph().AddRun().AddImageFromBytes(minimalPNG(), opc.ContentTypePNG); err != nil {
		t.Fatalf("AddImageFromBytes: %v", err)
	}
	c := chart.NewColumn()
	c.SetCategories([]string{"a", "b"})
	c.AddSeries("s", []float64{1, 2})
	if err := doc.AddChart(c, 5000000, 3000000); err != nil {
		t.Fatalf("AddChart: %v", err)
	}

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	docXML := zipEntryString(t, saved, "word/document.xml")

	ids := regexp.MustCompile(`<wp:docPr id="(\d+)"`).FindAllStringSubmatch(docXML, -1)
	if len(ids) != 2 {
		t.Fatalf("want 2 wp:docPr elements, got %d:\n%s", len(ids), docXML)
	}
	if ids[0][1] == ids[1][1] {
		t.Errorf("image and chart share wp:docPr id %q (must be document-unique)", ids[0][1])
	}
}
