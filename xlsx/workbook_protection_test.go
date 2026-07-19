package xlsx

import (
	"bytes"
	"strings"
	"testing"
)

func TestWorkbookProtection_DefaultRoundTrip(t *testing.T) {
	w := Create()
	w.AddSheet("Sheet1")
	if w.Protection() != nil {
		t.Fatal("new workbook should have no protection")
	}
	w.Protect(WorkbookProtectionOptions{})

	rw := reopen(t, w)
	p := rw.Protection()
	if p == nil {
		t.Fatal("Protection() = nil after protect+reopen")
	}
	if !p.LockStructure() {
		t.Error("LockStructure() = false, want true for zero-value Protect")
	}
	if p.LockWindows() {
		t.Error("LockWindows() = true, want false")
	}
	if p.HasPassword() {
		t.Error("HasPassword() = true, want false for password-less Protect")
	}
}

func TestWorkbookProtection_PasswordAndWindows(t *testing.T) {
	w := Create()
	w.AddSheet("Sheet1")
	w.Protect(WorkbookProtectionOptions{Password: "secret", LockStructure: true, LockWindows: true})

	// The password must be stored as the legacy 16-bit hash, never in cleartext.
	data, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if strings.Contains(string(data), "secret") {
		t.Error("cleartext password leaked into workbook bytes")
	}

	rw, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	p := rw.Protection()
	if p == nil {
		t.Fatal("Protection() = nil after reopen")
	}
	if !p.LockStructure() || !p.LockWindows() {
		t.Errorf("LockStructure=%v LockWindows=%v, want both true", p.LockStructure(), p.LockWindows())
	}
	if !p.HasPassword() {
		t.Error("HasPassword() = false, want true")
	}
}

func TestWorkbookProtection_WindowsOnly(t *testing.T) {
	w := Create()
	w.AddSheet("Sheet1")
	w.Protect(WorkbookProtectionOptions{LockWindows: true})

	p := reopen(t, w).Protection()
	if p == nil {
		t.Fatal("Protection() = nil")
	}
	if p.LockStructure() {
		t.Error("LockStructure() = true, want false when only LockWindows requested")
	}
	if !p.LockWindows() {
		t.Error("LockWindows() = false, want true")
	}
}

func TestWorkbookProtection_Unprotect(t *testing.T) {
	w := Create()
	w.AddSheet("Sheet1")
	w.Protect(WorkbookProtectionOptions{})
	w.Unprotect()
	if w.Protection() != nil {
		t.Fatal("Protection() != nil after Unprotect")
	}
	if reopen(t, w).Protection() != nil {
		t.Error("Protection() != nil after Unprotect+reopen")
	}
}
