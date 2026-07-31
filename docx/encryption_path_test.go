package docx

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mgilbir/spine/common/crypto"
	"github.com/mgilbir/spine/opc"
)

// SaveEncrypted and Open are the file-path forms of SaveEncryptedTo and
// OpenReader. They are thin, but they are also the only place the package
// chooses a file mode and decides how a read error surfaces, and neither of
// those is visible through the reader/writer forms.
func TestEncryptedDocx_PathRoundTrip(t *testing.T) {
	const password = "correct horse battery staple"

	doc := Create()
	doc.AddParagraph().SetText("classified")
	doc.AddParagraph().SetText("second line")
	want := doc.Body()
	if want == "" {
		t.Fatal("fixture has no body text, so a round trip could not fail")
	}

	path := filepath.Join(t.TempDir(), "secret.docx")
	if err := doc.SaveEncrypted(path, password); err != nil {
		t.Fatalf("SaveEncrypted: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the encrypted file: %v", err)
	}
	// The bytes on disk must be an OLE/CFB container, not the plain zip. If
	// SaveEncrypted ever wrote the unencrypted package the round trip below
	// would still pass through Open, so this is the assertion that the file is
	// actually encrypted.
	if bytes.HasPrefix(raw, []byte("PK\x03\x04")) {
		t.Fatal("SaveEncrypted wrote a plain zip: the document is not encrypted")
	}
	if !bytes.HasPrefix(raw, []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}) {
		t.Errorf("the encrypted file does not start with the CFB signature; got % x", raw[:min(8, len(raw))])
	}
	// And the plaintext must not be sitting in the file.
	if bytes.Contains(raw, []byte("classified")) {
		t.Error("the plaintext body text appears verbatim in the encrypted file")
	}

	// Same bytes as the writer form, so the wrapper is not re-encoding.
	var viaWriter bytes.Buffer
	if err := doc.SaveEncryptedTo(&viaWriter, password); err != nil {
		t.Fatalf("SaveEncryptedTo: %v", err)
	}
	if len(raw) != viaWriter.Len() {
		t.Errorf("SaveEncrypted wrote %d bytes but SaveEncryptedTo produced %d", len(raw), viaWriter.Len())
	}

	back, err := Open(path, opc.WithPassword(password))
	if err != nil {
		t.Fatalf("Open with a password: %v", err)
	}
	if got := back.Body(); got != want {
		t.Errorf("body after the path round trip = %q, want %q", got, want)
	}
}

// TestEncryptedDocx_PathErrors covers the two failure modes the wrappers own.
func TestEncryptedDocx_PathErrors(t *testing.T) {
	const password = "hunter2"
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.docx")

	doc := Create()
	doc.AddParagraph().SetText("classified")
	if err := doc.SaveEncrypted(path, password); err != nil {
		t.Fatalf("SaveEncrypted: %v", err)
	}

	t.Run("wrong password", func(t *testing.T) {
		if _, err := Open(path, opc.WithPassword("not it")); !errors.Is(err, crypto.ErrWrongPassword) {
			t.Errorf("Open with a wrong password = %v, want crypto.ErrWrongPassword", err)
		}
	})

	t.Run("no password", func(t *testing.T) {
		// Without the option the same call reports what the input is, so a
		// caller can prompt and retry rather than guess from a zip error.
		if _, err := Open(path); !errors.Is(err, opc.ErrEncrypted) {
			t.Errorf("Open of an encrypted file without a password = %v, want opc.ErrEncrypted", err)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := Open(filepath.Join(dir, "absent.docx"), opc.WithPassword(password))
		if err == nil {
			t.Fatal("Open on a missing file returned no error")
		}
		// The path wrapper must surface the OS error, not swallow it into a
		// generic decryption failure — a caller distinguishes "no such file"
		// from "wrong password" by errors.Is.
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Open on a missing file = %v, want an fs.ErrNotExist", err)
		}
	})

	t.Run("unwritable destination", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("directory permissions do not block writes the same way on Windows")
		}
		if os.Geteuid() == 0 {
			t.Skip("running as root: an unwritable directory is still writable")
		}
		locked := filepath.Join(dir, "locked")
		if err := os.Mkdir(locked, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
		if err := doc.SaveEncrypted(filepath.Join(locked, "x.docx"), password); err == nil {
			t.Error("SaveEncrypted into an unwritable directory returned no error")
		}
	})
}

// TestEncryptedDocx_FileMode pins the mode SaveEncrypted creates the file with.
// An encrypted document is by definition something the caller wanted kept, so a
// world-writable mode would be a real defect and nothing else in the package
// looks at it.
func TestEncryptedDocx_FileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes are not meaningful on Windows")
	}
	path := filepath.Join(t.TempDir(), "secret.docx")
	doc := Create()
	doc.AddParagraph().SetText("x")
	if err := doc.SaveEncrypted(path, "pw"); err != nil {
		t.Fatalf("SaveEncrypted: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// os.WriteFile applies the process umask, so assert on the bits that must
	// *not* be set rather than on an exact mode.
	if perm := info.Mode().Perm(); perm&0o022 != 0 {
		t.Errorf("SaveEncrypted created the file with mode %#o: it must not be group- or world-writable", perm)
	}
	if perm := info.Mode().Perm(); perm&0o400 == 0 {
		t.Errorf("SaveEncrypted created the file with mode %#o: it must be readable by its owner", perm)
	}
}
