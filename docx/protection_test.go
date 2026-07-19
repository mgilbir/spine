package docx

import (
	"bytes"
	"strings"
	"testing"
)

// reopenDoc saves the document to bytes and reopens it, doubling as a
// Create -> Save -> Open round-trip check.
func reopenDoc(t *testing.T, d *Document) *Document {
	t.Helper()
	data, err := d.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	rd, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	return rd
}

func TestDocumentProtection_DefaultRoundTrip(t *testing.T) {
	d := Create()
	d.AddParagraphWithText("hello")
	if d.Protection() != nil {
		t.Fatal("new document should have no protection")
	}
	d.Protect(DocumentProtectionOptions{})

	p := reopenDoc(t, d).Protection()
	if p == nil {
		t.Fatal("Protection() = nil after protect+reopen")
	}
	if p.Edit() != EditReadOnly {
		t.Errorf("Edit() = %q, want readOnly (zero-value default)", p.Edit())
	}
	if !p.Enforced() {
		t.Error("Enforced() = false, want true")
	}
	if p.HasPassword() {
		t.Error("HasPassword() = true, want false")
	}
}

func TestDocumentProtection_FormsWithPassword(t *testing.T) {
	d := Create()
	d.AddParagraphWithText("form")
	d.Protect(DocumentProtectionOptions{
		Edit:               EditForms,
		Password:           "hunter2",
		RestrictFormatting: true,
	})

	data, err := d.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if strings.Contains(string(data), "hunter2") {
		t.Error("cleartext password leaked into document bytes")
	}

	rd, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	p := rd.Protection()
	if p == nil {
		t.Fatal("Protection() = nil after reopen")
	}
	if p.Edit() != EditForms {
		t.Errorf("Edit() = %q, want forms", p.Edit())
	}
	if !p.RestrictFormatting() {
		t.Error("RestrictFormatting() = false, want true")
	}
	if !p.HasPassword() {
		t.Error("HasPassword() = false, want true")
	}
}

func TestDocumentProtection_ReadOnlyRecommended(t *testing.T) {
	d := Create()
	d.AddParagraphWithText("advice")
	d.Protect(DocumentProtectionOptions{Edit: EditReadOnly, ReadOnlyRecommended: true})

	p := reopenDoc(t, d).Protection()
	if p == nil {
		t.Fatal("Protection() = nil")
	}
	if !p.ReadOnlyRecommended() {
		t.Error("ReadOnlyRecommended() = false, want true")
	}
}

func TestDocumentProtection_Unprotect(t *testing.T) {
	d := Create()
	d.AddParagraphWithText("x")
	d.Protect(DocumentProtectionOptions{ReadOnlyRecommended: true})
	d.Unprotect()
	if d.Protection() != nil {
		t.Fatal("Protection() != nil after Unprotect")
	}
	if reopenDoc(t, d).Protection() != nil {
		t.Error("Protection() != nil after Unprotect+reopen")
	}
}
