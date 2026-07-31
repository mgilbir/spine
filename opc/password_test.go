package opc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/crypto"
	"github.com/mgilbir/spine/common/options"
)

// A password is a secret, and a configuration value is exactly the kind of
// thing that ends up in a debug log or an error message. ReaderOptions holds it
// as an unexported *string for that reason: fmt walks unexported struct fields
// as readily as exported ones, so a plain string field would print in full from
// any %v, %+v or %#v of a value containing a ReaderOptions, at any nesting
// depth. A pointer prints as its address instead.
//
// This test is the guard on that decision. It fails if the field is ever
// changed to a string, or a String method is added that renders it.
func TestPasswordDoesNotLeakThroughFormatting(t *testing.T) {
	const password = "correct horse battery staple"

	opts := options.Resolve(DefaultReaderOptions(), WithPassword(password))
	if opts.password == nil || *opts.password != password {
		t.Fatal("WithPassword did not record the password, so this test would pass vacuously")
	}

	// Everything a caller might plausibly print, including the shapes that put
	// the options behind an unexported field (where fmt cannot call a String
	// method and falls back to walking the struct).
	type wrapperUnexported struct{ o ReaderOptions }
	type wrapperExported struct{ O ReaderOptions }

	subjects := map[string]any{
		"value":                opts,
		"pointer":              &opts,
		"unexported field":     wrapperUnexported{opts},
		"exported field":       wrapperExported{opts},
		"slice":                []ReaderOptions{opts},
		"map":                  map[string]ReaderOptions{"cfg": opts},
		"error wrapping value": fmt.Errorf("open failed with %+v", opts),
	}
	for name, subject := range subjects {
		for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x"} {
			got := fmt.Sprintf(verb, subject)
			if strings.Contains(got, password) {
				t.Errorf("%s formatted with %s leaks the password: %s", name, verb, got)
			}
			// %x would render the bytes rather than the text, so check that
			// spelling of it too.
			if strings.Contains(got, fmt.Sprintf("%x", password)) {
				t.Errorf("%s formatted with %s leaks the password in hex: %s", name, verb, got)
			}
		}
	}

	// Nor through serialization: an unexported field is invisible to
	// encoding/json, so a config dump cannot carry it either.
	encoded, err := json.Marshal(opts)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if bytes.Contains(encoded, []byte(password)) {
		t.Errorf("json.Marshal leaks the password: %s", encoded)
	}
}

// The password must not reach an error message either — not from a wrong
// password, and not from a bound rejecting the input before the decrypt.
func TestPasswordDoesNotLeakThroughErrors(t *testing.T) {
	const password = "correct horse battery staple"
	plain := buildPlainPackage(t, 500)
	var enc bytes.Buffer
	if err := SaveEncrypted(&enc, plain, "the real one"); err != nil {
		t.Fatal(err)
	}
	container := enc.Bytes()

	for name, err := range map[string]error{
		"wrong password": errFrom(NewReader(bytes.NewReader(container), int64(len(container)),
			WithPassword(password))),
		"input over the size bound": errFrom(NewReader(bytes.NewReader(container), int64(len(container)),
			WithPassword(password), WithMaxEncryptedInputSize(10))),
		"not a container at all": errFrom(NewReader(bytes.NewReader([]byte("not a package")), 13,
			WithPassword(password))),
	} {
		if err == nil {
			t.Fatalf("%s: expected an error", name)
		}
		if strings.Contains(err.Error(), password) {
			t.Errorf("%s: the error names the password: %v", name, err)
		}
	}
}

// An encrypted package opens through the ordinary open, and the same call
// without a password reports what the input is rather than a zip parse failure.
func TestWithPasswordOpensAnEncryptedPackage(t *testing.T) {
	const password = "hunter2"
	plain := buildPlainPackage(t, 4000)
	var enc bytes.Buffer
	if err := SaveEncrypted(&enc, plain, password); err != nil {
		t.Fatal(err)
	}
	container := enc.Bytes()

	r, err := NewReader(bytes.NewReader(container), int64(len(container)), WithPassword(password))
	if err != nil {
		t.Fatalf("NewReader with the password: %v", err)
	}
	f := r.GetFile("/ppt/body.bin")
	if f == nil {
		t.Fatal("decrypted package is missing /ppt/body.bin")
	}
	body, err := f.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(body) != 4000 {
		t.Errorf("body is %d bytes, want 4000", len(body))
	}

	if _, err := NewReader(bytes.NewReader(container), int64(len(container))); !errors.Is(err, ErrEncrypted) {
		t.Errorf("no password: got %v, want ErrEncrypted", err)
	}

	// A password is simply unused when the input is a plain package, so a
	// caller that always passes one still reads unencrypted files.
	r, err = NewReader(bytes.NewReader(plain), int64(len(plain)), WithPassword(password))
	if err != nil {
		t.Fatalf("plain package opened with a password: %v", err)
	}
	if r.GetFile("/ppt/body.bin") == nil {
		t.Error("plain package opened with a password is missing /ppt/body.bin")
	}
}

// The bounds still apply to the package recovered from an encrypted container:
// decrypting must not hand the inner zip a reader with its guards off.
func TestBoundsApplyToTheDecryptedPackage(t *testing.T) {
	const password = "hunter2"
	plain := buildPlainPackage(t, 40000)
	var enc bytes.Buffer
	if err := SaveEncrypted(&enc, plain, password); err != nil {
		t.Fatal(err)
	}
	container := enc.Bytes()

	// A per-part bound below the body size must reject it, exactly as it would
	// for the same package unencrypted.
	r, err := NewReader(bytes.NewReader(container), int64(len(container)),
		WithPassword(password), WithMaxDecompressedPartSize(1000))
	if err != nil {
		t.Fatalf("opening with a small per-part bound: %v", err)
	}
	f := r.GetFile("/ppt/body.bin")
	if f == nil {
		t.Fatal("decrypted package is missing /ppt/body.bin")
	}
	if _, err := f.ReadAll(); err == nil {
		t.Error("a 40000-byte part read under a 1000-byte per-part bound returned no error")
	}

	// And the input bound is what stops an oversized container before it is
	// read into memory at all.
	_, err = NewReader(bytes.NewReader(container), int64(len(container)),
		WithPassword(password), WithMaxEncryptedInputSize(int64(len(container)-1)))
	if err == nil {
		t.Fatal("an input one byte over MaxEncryptedInputSize was accepted")
	}
	if !strings.Contains(err.Error(), "WithMaxEncryptedInputSize") {
		t.Errorf("the error should name the option that raises the bound: %v", err)
	}
}

// A plaintext that is itself a CFB container is a corrupt package, not a second
// layer to decrypt: the open must not recurse into it. Without the guard a
// crafted file recurses once per layer, each layer costing a full copy of the
// remaining bytes.
func TestNestedContainerIsNotDecryptedAgain(t *testing.T) {
	const password = "hunter2"
	plain := buildPlainPackage(t, 200)

	// Encrypt once, then encrypt the resulting container again with the same
	// password: decrypting the outer layer yields another CFB container.
	var inner bytes.Buffer
	if err := SaveEncrypted(&inner, plain, password); err != nil {
		t.Fatal(err)
	}
	var outer bytes.Buffer
	if err := SaveEncrypted(&outer, inner.Bytes(), password); err != nil {
		t.Fatal(err)
	}
	container := outer.Bytes()

	r, err := NewReader(bytes.NewReader(container), int64(len(container)), WithPassword(password))
	if err == nil {
		t.Fatal("a doubly-wrapped container opened; the open decrypted its own plaintext")
	}
	if r != nil {
		t.Error("an error came back with a non-nil Reader")
	}
	// It fails as a malformed package, which is what a CFB image is when read
	// as a zip — not as a wrong password, and not as ErrEncrypted.
	if errors.Is(err, crypto.ErrWrongPassword) || errors.Is(err, ErrEncrypted) {
		t.Errorf("got %v; want the zip-level failure of reading a container as a package", err)
	}
}

// errFrom discards a value and keeps the error, so error-only assertions can be
// written inline in a table.
func errFrom[T any](_ T, err error) error { return err }
