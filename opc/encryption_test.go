package opc

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/crypto"
)

// buildPlainPackage returns the bytes of a small valid OPC package plus a body
// part of the requested size, so tests can exercise both single-segment and
// multi-segment (>4096-byte) encryption.
func buildPlainPackage(t *testing.T, bodyLen int) []byte {
	t.Helper()
	body := bytes.Repeat([]byte("A"), bodyLen)
	parts := map[string][]byte{
		"/ppt/presentation.xml": []byte("<presentation/>"),
		"/ppt/body.bin":         body,
	}
	cts := map[string]string{
		"/ppt/presentation.xml": ContentTypePresentationMain,
		"/ppt/body.bin":         "application/octet-stream",
	}
	return createTestPackage(t, parts, cts)
}

func TestEncryptedRoundTripRecoversPackageBytes(t *testing.T) {
	for _, bodyLen := range []int{0, 100, 4096, 4097, 5000, 20000} {
		plain := buildPlainPackage(t, bodyLen)

		var enc bytes.Buffer
		if err := SaveEncrypted(&enc, plain, "hunter2"); err != nil {
			t.Fatalf("bodyLen=%d SaveEncrypted: %v", bodyLen, err)
		}

		// The encrypted output must be a CFB container, not a zip.
		if !isCFB(enc.Bytes()) {
			t.Fatalf("bodyLen=%d encrypted output is not a CFB container", bodyLen)
		}

		// Decrypt straight back to the exact inner package bytes.
		got, err := decryptCFBPackage(enc.Bytes(), "hunter2", ReaderOptions{})
		if err != nil {
			t.Fatalf("bodyLen=%d decrypt: %v", bodyLen, err)
		}
		if !bytes.Equal(got, plain) {
			t.Fatalf("bodyLen=%d inner package not recovered exactly: got %d bytes want %d", bodyLen, len(got), len(plain))
		}

		// And the decrypted package opens and reads correctly through OpenEncrypted.
		r, err := OpenEncrypted(bytes.NewReader(enc.Bytes()), int64(enc.Len()), "hunter2")
		if err != nil {
			t.Fatalf("bodyLen=%d OpenEncrypted: %v", bodyLen, err)
		}
		f := r.GetFile("/ppt/body.bin")
		if f == nil {
			t.Fatalf("bodyLen=%d decrypted package missing body part", bodyLen)
		}
		bodyBack, err := f.ReadAll()
		if err != nil {
			t.Fatalf("bodyLen=%d reading body: %v", bodyLen, err)
		}
		if len(bodyBack) != bodyLen {
			t.Fatalf("bodyLen=%d body length mismatch: got %d", bodyLen, len(bodyBack))
		}
	}
}

func TestEncryptedSaveOptionsRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		opts EncryptOptions
	}{
		{"agile", EncryptOptions{Scheme: SchemeAgile}},
		{"agile+dataspaces", EncryptOptions{Scheme: SchemeAgile, IncludeDataSpaces: true}},
		{"standard-default", EncryptOptions{Scheme: SchemeStandard}},
		{"standard-128", EncryptOptions{Scheme: SchemeStandard, StandardKeyBits: 128}},
		{"standard-192", EncryptOptions{Scheme: SchemeStandard, StandardKeyBits: 192}},
		{"standard-256+dataspaces", EncryptOptions{Scheme: SchemeStandard, StandardKeyBits: 256, IncludeDataSpaces: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, bodyLen := range []int{0, 4096, 5000} {
				plain := buildPlainPackage(t, bodyLen)
				var enc bytes.Buffer
				if err := SaveEncryptedWithOptions(&enc, plain, "pw!", c.opts); err != nil {
					t.Fatalf("bodyLen=%d SaveEncryptedWithOptions: %v", bodyLen, err)
				}
				if !isCFB(enc.Bytes()) {
					t.Fatalf("bodyLen=%d output is not a CFB container", bodyLen)
				}
				got, err := decryptCFBPackage(enc.Bytes(), "pw!", ReaderOptions{})
				if err != nil {
					t.Fatalf("bodyLen=%d decrypt: %v", bodyLen, err)
				}
				if !bytes.Equal(got, plain) {
					t.Fatalf("bodyLen=%d not recovered exactly: got %d want %d", bodyLen, len(got), len(plain))
				}
				// Wrong password must be rejected regardless of scheme.
				if _, err := decryptCFBPackage(enc.Bytes(), "nope", ReaderOptions{}); !errors.Is(err, crypto.ErrWrongPassword) {
					t.Fatalf("bodyLen=%d wrong password: got %v, want ErrWrongPassword", bodyLen, err)
				}
			}
		})
	}
}

func TestEncryptedDataSpacesStreamsPresent(t *testing.T) {
	plain := buildPlainPackage(t, 3000)
	var enc bytes.Buffer
	if err := SaveEncryptedWithOptions(&enc, plain, "pw", EncryptOptions{IncludeDataSpaces: true}); err != nil {
		t.Fatal(err)
	}
	cf, err := readCFB(enc.Bytes())
	if err != nil {
		t.Fatalf("readCFB: %v", err)
	}
	// The nested DataSpaces streams must all be reachable and carry the canonical
	// reference bytes.
	checks := map[string][]byte{
		dsStreamVersion:      dsVersionContent,
		dsStreamDataSpaceMap: dsDataSpaceMapContent,
		dsStreamStrongDS:     dsStrongDSContent,
		dsStreamPrimary:      dsPrimaryContent,
	}
	for name, want := range checks {
		got, err := cf.stream(name)
		if err != nil {
			t.Fatalf("stream %q: %v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("stream %q content mismatch", name)
		}
	}
	// And the encrypted payload still round-trips with the extra streams present.
	if got, err := decryptCFBPackage(enc.Bytes(), "pw", ReaderOptions{}); err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("round-trip with DataSpaces failed: err=%v", err)
	}
}

func TestEncryptedWrongPassword(t *testing.T) {
	plain := buildPlainPackage(t, 2000)
	var enc bytes.Buffer
	if err := SaveEncrypted(&enc, plain, "correct horse"); err != nil {
		t.Fatal(err)
	}
	_, err := OpenEncrypted(bytes.NewReader(enc.Bytes()), int64(enc.Len()), "wrong password")
	if !errors.Is(err, crypto.ErrWrongPassword) {
		t.Fatalf("got %v, want crypto.ErrWrongPassword", err)
	}
}

func TestEncryptedTamperFailsIntegrity(t *testing.T) {
	plain := buildPlainPackage(t, 8000)
	var enc bytes.Buffer
	if err := SaveEncrypted(&enc, plain, "pw"); err != nil {
		t.Fatal(err)
	}

	// Pull the two streams back out, flip a ciphertext byte in EncryptedPackage
	// (past its 8-byte size prefix), and repackage. Working through the CFB
	// streams keeps the test independent of the container's sector layout.
	cf, err := readCFB(enc.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	info, err := cf.stream(cfbStreamEncryptionInfo)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := cf.stream(cfbStreamEncryptedPackage)
	if err != nil {
		t.Fatal(err)
	}
	pkg[len(pkg)-1] ^= 0xFF

	_, err = crypto.Decrypt(info, pkg, "pw")
	if !errors.Is(err, crypto.ErrIntegrityCheckFailed) {
		t.Fatalf("tampered package: got %v, want ErrIntegrityCheckFailed", err)
	}
}

func TestOpenReaderDetectsEncrypted(t *testing.T) {
	plain := buildPlainPackage(t, 500)
	var enc bytes.Buffer
	if err := SaveEncrypted(&enc, plain, "pw"); err != nil {
		t.Fatal(err)
	}
	_, err := NewReader(bytes.NewReader(enc.Bytes()), int64(enc.Len()))
	if !errors.Is(err, ErrEncrypted) {
		t.Fatalf("NewReader on encrypted input: got %v, want ErrEncrypted", err)
	}
	if !strings.Contains(err.Error(), "OpenEncrypted") {
		t.Fatalf("ErrEncrypted message should point to OpenEncrypted, got %q", err.Error())
	}
}

func TestSaveEncryptedRejectsEmptyPassword(t *testing.T) {
	if err := SaveEncrypted(&bytes.Buffer{}, []byte("x"), ""); err == nil {
		t.Fatal("expected error for empty password")
	}
}

func TestSaveEncryptedProducesFreshSalt(t *testing.T) {
	plain := buildPlainPackage(t, 1000)
	var a, b bytes.Buffer
	if err := SaveEncrypted(&a, plain, "pw"); err != nil {
		t.Fatal(err)
	}
	if err := SaveEncrypted(&b, plain, "pw"); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("two encryptions of the same package produced identical bytes (salt not randomized)")
	}
}

func TestEncryptedLargeIncompressiblePackage(t *testing.T) {
	// An incompressible body forces the inner zip — and therefore the
	// EncryptedPackage stream — above the 4096-byte mini-stream cutoff, so it is
	// stored in the regular FAT across many sectors (multiple FAT sectors here).
	body := make([]byte, 300*1024)
	for i := range body {
		body[i] = byte(i*2654435761 + i>>3) // cheap incompressible-ish fill
	}
	parts := map[string][]byte{
		"/ppt/presentation.xml": []byte("<presentation/>"),
		"/ppt/body.bin":         body,
	}
	cts := map[string]string{
		"/ppt/presentation.xml": ContentTypePresentationMain,
		"/ppt/body.bin":         "application/octet-stream",
	}
	plain := createTestPackage(t, parts, cts)

	var enc bytes.Buffer
	if err := SaveEncrypted(&enc, plain, "pw"); err != nil {
		t.Fatal(err)
	}
	got, err := decryptCFBPackage(enc.Bytes(), "pw", ReaderOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("large package not recovered exactly: got %d want %d", len(got), len(plain))
	}
}
