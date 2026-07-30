package opc

import "testing"

// TestRemoveOverrideCaseFallbackIsDeterministic pins the fallback path's choice.
//
// RemoveOverride matches part names case-insensitively, and packages that carry
// two overrides differing only in case are real — CheckDuplicateParts exists
// because of them. The fallback used to take whichever key the map range
// yielded first and stop, so which override survived, and therefore the bytes
// of the saved [Content_Types].xml, changed between runs. It now takes the
// lowest matching name.
func TestRemoveOverrideCaseFallbackIsDeterministic(t *testing.T) {
	const (
		lower = "/word/Document.xml"
		upper = "/word/document.xml"
	)
	// Repeated because the defect was a coin flip: one run of the old code had
	// an even chance of looking correct.
	for i := 0; i < 32; i++ {
		ct := NewContentTypes()
		ct.SetOverride(lower, ContentTypeDocument)
		ct.SetOverride(upper, ContentTypeDocumentTemplateMain)

		ct.RemoveOverride("/WORD/DOCUMENT.XML")

		if _, ok := ct.Overrides[lower]; ok {
			t.Fatalf("run %d: %q survived; the lower-sorting name should have been removed", i, lower)
		}
		if _, ok := ct.Overrides[upper]; !ok {
			t.Fatalf("run %d: %q was removed; only the lowest matching name should go", i, upper)
		}
	}
}
