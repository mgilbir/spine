package crypto

// Tests for the agile integrity guarantee: that a package this library decrypts
// without error is a package whose bytes were authenticated. The interesting
// cases are the ones where an attacker edits the EncryptionInfo descriptor,
// which is plaintext and covered by nothing.

import (
	"bytes"
	"errors"
	"regexp"
	"testing"
)

// agilePlaintext is a recognizable payload: any accepted tampering shows up as a
// difference against it.
func agilePlaintext() []byte {
	p := make([]byte, 6000) // spans more than one 4096-byte segment
	for i := range p {
		p[i] = byte(i * 3)
	}
	return p
}

// flipCiphertextBit returns a copy of an EncryptedPackage stream with one bit of
// its ciphertext flipped, leaving the 8-byte size prefix intact. AES-CBC is
// malleable, so this is exactly the edit an attacker with write access makes.
func flipCiphertextBit(encryptedPackage []byte) []byte {
	t := append([]byte(nil), encryptedPackage...)
	t[8+37] ^= 0x08
	return t
}

var dataIntegrityElement = regexp.MustCompile(`<dataIntegrity[^>]*/>`)

// stripDataIntegrity deletes the whole <dataIntegrity/> element from an agile
// EncryptionInfo descriptor.
func stripDataIntegrity(t *testing.T, info []byte) []byte {
	t.Helper()
	out := dataIntegrityElement.ReplaceAll(info, nil)
	if len(out) == len(info) {
		t.Fatalf("descriptor has no <dataIntegrity/> element to strip:\n%s", info[8:])
	}
	return out
}

// renameAttr rewrites one attribute name in the descriptor, which is how an
// attacker makes a "both attributes present?" check fall through without
// removing the element.
func renameAttr(t *testing.T, info []byte, from, to string) []byte {
	t.Helper()
	out := bytes.Replace(info, []byte(from+`="`), []byte(to+`="`), 1)
	if bytes.Equal(out, info) {
		t.Fatalf("descriptor does not contain attribute %q", from)
	}
	return out
}

// TestAgileTamperedPackageIsRejectedWhenDataIntegrityIsStripped is the
// regression test for the authentication bypass: the package HMAC used to be
// verified only when the descriptor still advertised one, so deleting the
// unauthenticated <dataIntegrity/> element disabled the check and Decrypt
// returned attacker-modified plaintext with a nil error.
func TestAgileTamperedPackageIsRejectedWhenDataIntegrityIsStripped(t *testing.T) {
	plain := agilePlaintext()
	const password = "integrity-pw"

	info, enc, err := Encrypt(plain, password)
	if err != nil {
		t.Fatal(err)
	}

	// Baseline: the untouched container decrypts and is reported as authenticated.
	res, err := DecryptWithOptions(info, enc, password, DecryptOptions{})
	if err != nil {
		t.Fatalf("baseline decrypt: %v", err)
	}
	if !bytes.Equal(res.Package, plain) {
		t.Fatal("baseline decrypt returned different bytes")
	}
	if res.Scheme != SchemeAgile || !res.IntegrityVerified {
		t.Fatalf("baseline: scheme=%v integrityVerified=%v, want agile/true", res.Scheme, res.IntegrityVerified)
	}

	tampered := flipCiphertextBit(enc)

	// Control: with the descriptor untouched the HMAC catches the edit.
	if _, err := Decrypt(info, tampered, password); !errors.Is(err, ErrIntegrityCheckFailed) {
		t.Fatalf("tampered ciphertext, descriptor intact: got %v, want ErrIntegrityCheckFailed", err)
	}

	// The attack: delete the element that requests the check. It is plaintext
	// and authenticated by nothing, so it costs the attacker nothing.
	stripped := stripDataIntegrity(t, info)
	got, err := Decrypt(stripped, tampered, password)
	if !errors.Is(err, ErrIntegrityCheckFailed) {
		t.Fatalf("tampered ciphertext with dataIntegrity stripped: got err=%v (%d plaintext bytes), want ErrIntegrityCheckFailed", err, len(got))
	}
	if got != nil {
		t.Fatal("rejected decrypt still returned plaintext")
	}

	// An untampered package with a stripped descriptor is refused just the same:
	// missing and stripped are indistinguishable, so both are unauthenticated.
	if _, err := Decrypt(stripped, enc, password); !errors.Is(err, ErrIntegrityCheckFailed) {
		t.Fatalf("intact ciphertext with dataIntegrity stripped: got %v, want ErrIntegrityCheckFailed", err)
	}

	// A wrong password on a stripped descriptor must not be reported as an
	// integrity problem's opposite either — the descriptor is refused first.
	if _, err := Decrypt(stripped, enc, "not-the-password"); err == nil {
		t.Fatal("stripped descriptor with a wrong password decrypted successfully")
	}
}

// TestAgileHalfPresentDataIntegrityIsRejected covers the cheaper variant of the
// same attack: rename one of the two attributes and an "if key != \"\" && value
// != \"\"" guard skips the check without the element ever disappearing.
func TestAgileHalfPresentDataIntegrityIsRejected(t *testing.T) {
	plain := agilePlaintext()
	const password = "integrity-pw"
	info, enc, err := Encrypt(plain, password)
	if err != nil {
		t.Fatal(err)
	}
	tampered := flipCiphertextBit(enc)

	for _, c := range []struct {
		name     string
		from, to string
	}{
		{"value-renamed", "encryptedHmacValue", "encryptedHmacXalue"},
		{"key-renamed", "encryptedHmacKey", "encryptedHmacKez"},
	} {
		t.Run(c.name, func(t *testing.T) {
			mangled := renameAttr(t, info, c.from, c.to)
			if _, err := Decrypt(mangled, tampered, password); !errors.Is(err, ErrIntegrityCheckFailed) {
				t.Fatalf("tampered: got %v, want ErrIntegrityCheckFailed", err)
			}
			if _, err := Decrypt(mangled, enc, password); !errors.Is(err, ErrIntegrityCheckFailed) {
				t.Fatalf("untampered: got %v, want ErrIntegrityCheckFailed", err)
			}
			// The opt-out is for producers that omit the block entirely; a
			// half-present block is never acceptable.
			opts := DecryptOptions{AllowMissingDataIntegrity: true}
			if _, err := DecryptWithOptions(mangled, enc, password, opts); !errors.Is(err, ErrIntegrityCheckFailed) {
				t.Fatalf("with AllowMissingDataIntegrity: got %v, want ErrIntegrityCheckFailed", err)
			}
		})
	}
}

// TestAgileAllowMissingDataIntegrityIsOptInOnly documents the escape hatch: it
// has to be asked for by the caller, it is reported in the result, and it never
// relaxes an HMAC that is present and wrong.
func TestAgileAllowMissingDataIntegrityIsOptInOnly(t *testing.T) {
	plain := agilePlaintext()
	const password = "integrity-pw"
	info, enc, err := Encrypt(plain, password)
	if err != nil {
		t.Fatal(err)
	}
	stripped := stripDataIntegrity(t, info)
	opts := DecryptOptions{AllowMissingDataIntegrity: true}

	res, err := DecryptWithOptions(stripped, enc, password, opts)
	if err != nil {
		t.Fatalf("opted-in decrypt of a descriptor without dataIntegrity: %v", err)
	}
	if !bytes.Equal(res.Package, plain) {
		t.Fatal("opted-in decrypt returned different bytes")
	}
	if res.IntegrityVerified {
		t.Fatal("IntegrityVerified is true for a package with no dataIntegrity block")
	}

	// The opt-in does not turn off a check that can be made: a present HMAC
	// still has to match.
	if _, err := DecryptWithOptions(info, flipCiphertextBit(enc), password, opts); !errors.Is(err, ErrIntegrityCheckFailed) {
		t.Fatalf("present-but-failing HMAC with AllowMissingDataIntegrity: got %v, want ErrIntegrityCheckFailed", err)
	}
}

// TestDecryptReportsScheme checks that a caller can tell an authenticated agile
// package from the unauthenticated legacy schemes without parsing the
// descriptor themselves.
func TestDecryptReportsScheme(t *testing.T) {
	plain := []byte("scheme reporting payload")
	const password = "scheme-pw"

	agileInfo, agilePkg, err := Encrypt(plain, password)
	if err != nil {
		t.Fatal(err)
	}
	stdInfo, stdPkg, err := EncryptStandard(plain, password, 256)
	if err != nil {
		t.Fatal(err)
	}
	rc4Info, rc4Pkg, err := EncryptRC4CryptoAPI(plain, password, 40)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name          string
		info, pkg     []byte
		wantScheme    Scheme
		wantIntegrity bool
	}{
		{"agile", agileInfo, agilePkg, SchemeAgile, true},
		{"standard", stdInfo, stdPkg, SchemeStandard, false},
		{"rc4", rc4Info, rc4Pkg, SchemeRC4CryptoAPI, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := DecryptWithOptions(c.info, c.pkg, password, DecryptOptions{})
			if err != nil {
				t.Fatalf("decrypt: %v", err)
			}
			if !bytes.Equal(res.Package, plain) {
				t.Fatal("round-trip mismatch")
			}
			if res.Scheme != c.wantScheme {
				t.Fatalf("Scheme = %v (%s), want %v", int(res.Scheme), res.Scheme, c.wantScheme)
			}
			if res.IntegrityVerified != c.wantIntegrity {
				t.Fatalf("IntegrityVerified = %v, want %v", res.IntegrityVerified, c.wantIntegrity)
			}
		})
	}

	if got := SchemeUnknown.String(); got != "unknown" {
		t.Fatalf("SchemeUnknown.String() = %q", got)
	}
}
