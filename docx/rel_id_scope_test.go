package docx

import (
	"testing"

	"github.com/mgilbir/spine/opc"
)

// C297: relationship ids are scoped per owner part. A header/footer part whose
// own .rels already carry an id at or above the main part's next free rId must
// not get a duplicate Id when a header-scoped relationship (e.g. an image) is
// added — the allocator seeds from the owning part's rels, not the main part's.
func TestNextRelIDForPart_HeaderScopeNoDuplicate(t *testing.T) {
	d := Create()
	h := d.AddHeader(HeaderDefault) // consumes rId1 on the main part
	hdrPart := h.partName

	// Simulate a wild file whose header .rels already carries the very id the
	// main-part-seeded counter would hand out next (rId2), above the main part's
	// max.
	d.relationships[hdrPart] = append(d.relationships[hdrPart], &opc.Relationship{
		ID:     "rId2",
		Type:   opc.RelTypeImage,
		Target: "media/preexisting.png",
	})

	// Add a header-scoped relationship: an image on a header paragraph.
	run := h.AddParagraph().AddRun()
	if _, err := run.AddImageFromBytes(minimalTransparentPNG, "image/png"); err != nil {
		t.Fatalf("AddImageFromBytes into header: %v", err)
	}

	seen := map[string]bool{}
	for _, rel := range d.relationships[hdrPart] {
		if seen[rel.ID] {
			t.Fatalf("duplicate relationship Id %q in header .rels %s", rel.ID, hdrPart)
		}
		seen[rel.ID] = true
	}
}
