package docx

import "testing"

// TestProtect_PreservesExistingWriteProtection verifies that a Protect call
// which does not set ReadOnlyRecommended leaves an existing w:writeProtection
// intact (C289). The else-branch used to remove it unconditionally, so an
// unrelated edit-restriction change destroyed an independent write-protection
// guard.
func TestProtect_PreservesExistingWriteProtection(t *testing.T) {
	d := Create()
	d.AddParagraphWithText("hello")

	// Establish a read-only recommendation (writes w:writeProtection).
	d.Protect(DocumentProtectionOptions{Edit: EditReadOnly, ReadOnlyRecommended: true})
	if p := d.Protection(); p == nil || !p.ReadOnlyRecommended() {
		t.Fatal("setup: ReadOnlyRecommended not set")
	}

	// An unrelated edit-restriction change (ReadOnlyRecommended left false).
	d.Protect(DocumentProtectionOptions{Edit: EditComments})

	p := reopenDoc(t, d).Protection()
	if p == nil {
		t.Fatal("Protection() = nil after second Protect")
	}
	if !p.ReadOnlyRecommended() {
		t.Error("ReadOnlyRecommended() = false: existing writeProtection was destroyed")
	}
	if p.Edit() != EditComments {
		t.Errorf("Edit() = %q, want comments", p.Edit())
	}
}
