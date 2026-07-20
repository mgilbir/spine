package docx

import (
	"bytes"
	"errors"
	"os"
	"testing"

	"github.com/mgilbir/spine/common/crypto"
)

// TestEncryptedDocxRoundTrip opens a plain fixture, saves it encrypted, then
// reopens it with the password and confirms the document content survives.
func TestEncryptedDocxRoundTrip(t *testing.T) {
	plain, err := os.ReadFile("testdata/minimal.docx")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := OpenReader(bytes.NewReader(plain), int64(len(plain)))
	if err != nil {
		t.Fatal(err)
	}
	wantText := doc.Body()

	var enc bytes.Buffer
	if err := doc.SaveEncryptedTo(&enc, "s3cret!"); err != nil {
		t.Fatalf("SaveEncryptedTo: %v", err)
	}

	// Wrong password is rejected.
	if _, err := OpenEncryptedReader(bytes.NewReader(enc.Bytes()), int64(enc.Len()), "nope"); !errors.Is(err, crypto.ErrWrongPassword) {
		t.Fatalf("wrong password: got %v, want crypto.ErrWrongPassword", err)
	}

	// Correct password recovers the document.
	back, err := OpenEncryptedReader(bytes.NewReader(enc.Bytes()), int64(enc.Len()), "s3cret!")
	if err != nil {
		t.Fatalf("OpenEncryptedReader: %v", err)
	}
	if got := back.Body(); got != wantText {
		t.Fatalf("text mismatch after encrypted round-trip:\n got %q\nwant %q", got, wantText)
	}
}
