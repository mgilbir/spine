package docx

import (
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/validate"
	"github.com/mgilbir/spine/opc"
)

// TestValidate_AddedImageNoFalsePositive verifies that Validate does not report
// a rel-target-missing finding for an image queued this session (C290). The
// image part is written on save, so its relationship target is not dangling;
// partExists must know about d.imageParts or the check contradicts its own
// never-a-false-positive contract.
func TestValidate_AddedImageNoFalsePositive(t *testing.T) {
	doc, err := Open("testdata/minimal.docx")
	if err != nil {
		t.Fatal(err)
	}
	img, err := doc.AddParagraph().AddRun().AddImageFromBytes(minimalPNG(), opc.ContentTypePNG)
	if err != nil {
		t.Fatal(err)
	}
	part := img.PartName()
	if part == "" {
		t.Fatal("added image has no resolved part name")
	}

	rep := doc.Validate()
	for _, f := range rep {
		if f.Code == validate.CodeRelTargetMissing && strings.Contains(f.Detail+" "+f.Part, "image") {
			t.Errorf("false-positive rel-target-missing for session-added image: %s", f.Error())
		}
	}

	// And the document must still save (the pre-save Validate gate must not
	// refuse to write over its own false positive).
	if _, err := doc.SaveBytes(); err != nil {
		t.Fatalf("save after AddImage: %v", err)
	}
}
