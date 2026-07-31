package opc

import (
	"bytes"
	"errors"
	"regexp"
	"testing"

	"github.com/mgilbir/spine/common/crypto"
)

// C386, integrity half: common/crypto grew DecryptOptions.AllowMissingDataIntegrity
// but opc could not reach it, so an OPC-level caller had no way to open an agile
// package from a third-party producer that omits dataIntegrity — short of
// re-implementing the CFB parse, which is private to this package. The option
// now rides on ReaderOptions, and the default must stay strict (C361).

var testDataIntegrityElement = regexp.MustCompile(`<dataIntegrity[^>]*/>`)

// encryptedWithoutDataIntegrity builds a CFB container whose agile descriptor
// has had its <dataIntegrity/> element deleted, exactly as a producer that
// never writes one would leave it.
func encryptedWithoutDataIntegrity(t *testing.T, plain []byte, password string) []byte {
	t.Helper()
	info, pkg, err := crypto.Encrypt(plain, password)
	if err != nil {
		t.Fatalf("crypto.Encrypt: %v", err)
	}
	stripped := testDataIntegrityElement.ReplaceAll(info, nil)
	if len(stripped) == len(info) {
		t.Fatalf("descriptor carries no <dataIntegrity/> element to strip")
	}
	var buf bytes.Buffer
	if err := writeCFBWithStorages(&buf, []cfbStream{
		{name: cfbStreamEncryptionInfo, data: stripped},
		{name: cfbStreamEncryptedPackage, data: pkg},
	}, nil); err != nil {
		t.Fatalf("writeCFB: %v", err)
	}
	return buf.Bytes()
}

func TestOpenPlumbsAllowMissingDataIntegrity(t *testing.T) {
	plain := buildPlainPackage(t, 1000)
	const password = "hunter2"
	container := encryptedWithoutDataIntegrity(t, plain, password)

	// Default: strict. A missing integrity block is a hard failure, however the
	// options reach the open — one at a time, or as a prepared ReaderOptions.
	for name, opts := range map[string][]ReaderOption{
		"WithPassword":                     {WithPassword(password)},
		"WithReaderOptions+WithPassword":   {WithReaderOptions(DefaultReaderOptions()), WithPassword(password)},
		"WithPassword+unrelated overrides": {WithPassword(password), WithMaxNestingDepth(50)},
	} {
		_, err := NewReader(bytes.NewReader(container), int64(len(container)), opts...)
		if !errors.Is(err, crypto.ErrIntegrityCheckFailed) {
			t.Fatalf("%s: got %v, want ErrIntegrityCheckFailed", name, err)
		}
	}

	// Explicit opt-in: the package opens, and its bytes are the originals.
	r, err := NewReader(bytes.NewReader(container), int64(len(container)),
		WithPassword(password), WithAllowMissingDataIntegrity(true))
	if err != nil {
		t.Fatalf("opted-in open: %v", err)
	}
	f := r.GetFile("/ppt/body.bin")
	if f == nil {
		t.Fatal("decrypted package is missing /ppt/body.bin")
	}
	got, err := f.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(got) != 1000 {
		t.Fatalf("body is %d bytes, want 1000", len(got))
	}
}

// The opt-in must not become a general "skip integrity" switch: a descriptor
// that DOES carry a dataIntegrity block whose HMAC fails still fails.
func TestAllowMissingDataIntegrityDoesNotRelaxAFailedHMAC(t *testing.T) {
	plain := buildPlainPackage(t, 200)
	const password = "pw"
	info, pkg, err := crypto.Encrypt(plain, password)
	if err != nil {
		t.Fatalf("crypto.Encrypt: %v", err)
	}
	pkg[len(pkg)-1] ^= 0x01 // flip a ciphertext bit; the HMAC no longer matches

	var buf bytes.Buffer
	if err := writeCFBWithStorages(&buf, []cfbStream{
		{name: cfbStreamEncryptionInfo, data: info},
		{name: cfbStreamEncryptedPackage, data: pkg},
	}, nil); err != nil {
		t.Fatalf("writeCFB: %v", err)
	}
	container := buf.Bytes()

	if _, err := NewReader(bytes.NewReader(container), int64(len(container)),
		WithPassword(password), WithAllowMissingDataIntegrity(true)); !errors.Is(err, crypto.ErrIntegrityCheckFailed) {
		t.Fatalf("tampered ciphertext with the opt-in set: got %v, want ErrIntegrityCheckFailed", err)
	}
}
