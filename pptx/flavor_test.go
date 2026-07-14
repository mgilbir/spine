package pptx

import (
	"bytes"
	"errors"
	"testing"

	"github.com/mgilbir/spine/opc"
)

// retypedDeck builds a minimal deck and rewrites its main-part content-type
// override to the given PresentationML flavor.
func retypedDeck(t *testing.T, flavor string) []byte {
	t.Helper()
	p := Create()
	p.AddSlide()
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	return rewriteZipPart(t, data, "[Content_Types].xml", func(ct []byte) []byte {
		if !bytes.Contains(ct, []byte(opc.ContentTypePresentationMain)) {
			t.Fatal("[Content_Types].xml has no presentation main-part override")
		}
		return bytes.Replace(ct, []byte(opc.ContentTypePresentationMain), []byte(flavor), 1)
	})
}

// TestOpenFlavorVariants opens each PresentationML main-part flavor, checks
// the reported flavor, and verifies a zero-modification save keeps the flavor
// (no silent retype to a regular presentation).
func TestOpenFlavorVariants(t *testing.T) {
	flavors := []string{
		opc.ContentTypePresentationMain,
		opc.ContentTypeSlideshowMain,
		opc.ContentTypePresentationTemplateMain,
		opc.ContentTypePresentationMacroMain,
		opc.ContentTypeSlideshowMacroMain,
		opc.ContentTypePresentationTemplateMacroMain,
	}
	for _, flavor := range flavors {
		t.Run(flavor, func(t *testing.T) {
			data := retypedDeck(t, flavor)
			p, err := OpenReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatalf("OpenReader: %v", err)
			}
			defer func() { _ = p.Close() }()
			if got := p.Flavor(); got != flavor {
				t.Fatalf("Flavor() = %q, want %q", got, flavor)
			}

			saved, err := p.SaveBytes()
			if err != nil {
				t.Fatalf("SaveBytes: %v", err)
			}
			ct, ok := zipPartIfExists(t, saved, "[Content_Types].xml")
			if !ok {
				t.Fatal("saved package has no [Content_Types].xml")
			}
			if !bytes.Contains(ct, []byte(flavor)) {
				t.Fatalf("saved [Content_Types].xml lost the %q flavor:\n%s", flavor, ct)
			}
			if flavor != opc.ContentTypePresentationMain && bytes.Contains(ct, []byte(opc.ContentTypePresentationMain)) {
				t.Fatalf("saved [Content_Types].xml retyped the main part to a regular presentation:\n%s", ct)
			}

			reopened, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
			if err != nil {
				t.Fatalf("reopening saved package: %v", err)
			}
			defer func() { _ = reopened.Close() }()
			if got := reopened.Flavor(); got != flavor {
				t.Fatalf("reopened Flavor() = %q, want %q", got, flavor)
			}
		})
	}
}

// TestOpenRejectsUnknownMainPartContentType keeps the strict rejection for
// content types outside the PresentationML family.
func TestOpenRejectsUnknownMainPartContentType(t *testing.T) {
	data := retypedDeck(t, "application/x-not-a-presentation")
	if _, err := OpenReader(bytes.NewReader(data), int64(len(data))); !errors.Is(err, ErrNotPPTX) {
		t.Fatalf("OpenReader error = %v, want ErrNotPPTX", err)
	}
}

// TestCreatedPresentationFlavor pins the default flavor for created decks.
func TestCreatedPresentationFlavor(t *testing.T) {
	if got := Create().Flavor(); got != opc.ContentTypePresentationMain {
		t.Fatalf("Create().Flavor() = %q, want %q", got, opc.ContentTypePresentationMain)
	}
}
