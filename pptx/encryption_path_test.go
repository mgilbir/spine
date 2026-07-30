package pptx

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/mgilbir/spine/common/crypto"
)

// SaveEncrypted and OpenEncrypted are the file-path wrappers around
// SaveEncryptedTo / OpenEncryptedReader. They are thin, but a wrong file mode,
// a truncated write, or a read that never reaches the reader form would not be
// caught anywhere else, so this is a real temp-file round trip rather than a
// call for coverage.
func TestEncryptedFileRoundTrip(t *testing.T) {
	const password = "correct horse battery staple"

	p := Create()
	s := p.AddSlide()
	tb := NewTextBox()
	tb.SetText("encrypted content marker")
	if err := s.AddShape(tb); err != nil {
		t.Fatalf("AddShape: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "secret.pptx")
	if err := p.SaveEncrypted(path, password); err != nil {
		t.Fatalf("SaveEncrypted: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("SaveEncrypted wrote an empty file")
	}
	// 0o644 before umask; the file must at minimum be owner-readable and not
	// executable.
	if mode := info.Mode().Perm(); mode&0o400 == 0 || mode&0o111 != 0 {
		t.Errorf("SaveEncrypted wrote mode %v, want a readable non-executable regular file", mode)
	}

	// The bytes on disk must be an encrypted CFB container, not a plain zip:
	// a wrapper that fell through to the plaintext writer would start with PK.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if bytes.HasPrefix(raw, []byte("PK")) {
		t.Fatal("SaveEncrypted wrote a plain zip, not an encrypted container")
	}
	if !bytes.HasPrefix(raw, []byte{0xd0, 0xcf, 0x11, 0xe0}) {
		t.Errorf("file does not start with the CFB signature: % x", raw[:8])
	}
	// Opening it as a normal presentation must fail.
	if _, err := Open(path); err == nil {
		t.Error("Open succeeded on an encrypted file")
	}

	back, err := OpenEncrypted(path, password)
	if err != nil {
		t.Fatalf("OpenEncrypted: %v", err)
	}
	if got := len(back.Slides()); got != 1 {
		t.Fatalf("reopened deck has %d slides, want 1", got)
	}
	rs, err := back.Slide(0)
	if err != nil {
		t.Fatalf("Slide(0): %v", err)
	}
	var text string
	for _, sh := range rs.Shapes() {
		if b, ok := sh.(*TextBox); ok {
			text = b.Text()
		}
	}
	if text != "encrypted content marker" {
		t.Errorf("round-tripped text = %q, want %q", text, "encrypted content marker")
	}

	// A wrong password must be reported as such, not as a corrupt file.
	if _, err := OpenEncrypted(path, "wrong"); !errors.Is(err, crypto.ErrWrongPassword) {
		t.Errorf("OpenEncrypted with a wrong password = %v, want ErrWrongPassword", err)
	}
}

// TestOpenEncryptedMissingFile checks the path wrapper surfaces the OS error
// rather than swallowing it into a decryption failure.
func TestOpenEncryptedMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.pptx")
	p, err := OpenEncrypted(missing, "pw")
	if err == nil {
		t.Fatal("OpenEncrypted on a missing file returned no error")
	}
	if p != nil {
		t.Errorf("OpenEncrypted returned a non-nil presentation alongside %v", err)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("error = %v, want fs.ErrNotExist", err)
	}
}

// TestSaveEncryptedRejectsEmptyPassword confirms the wrapper runs the same
// validation as SaveEncryptedTo and leaves no half-written file behind.
func TestSaveEncryptedRejectsEmptyPassword(t *testing.T) {
	p := Create()
	p.AddSlide()
	path := filepath.Join(t.TempDir(), "out.pptx")
	if err := p.SaveEncrypted(path, ""); err == nil {
		t.Fatal("SaveEncrypted accepted an empty password")
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a file was created despite the failure: stat err = %v", err)
	}
}
