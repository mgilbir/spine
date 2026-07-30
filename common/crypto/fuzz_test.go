package crypto

// Fuzz targets for the encrypted-document descriptors: the agile EncryptionInfo
// XML and the binary standard/RC4 headers, driven together with the
// EncryptedPackage stream and the password.
//
// Both streams are entirely attacker-supplied — the descriptor is plaintext and
// is covered by no MAC, verifier or key derivation — which is what made C361
// possible: the package HMAC was verified only when the descriptor still
// carried a <dataIntegrity/> element, so deleting that element made tampered
// AES-CBC ciphertext decrypt as valid plaintext. These targets therefore assert
// the authentication contract on every accepted input, not just the absence of a
// panic:
//
//   - a strict decrypt that returns an agile package must have verified its
//     HMAC (the C361 invariant: the *absence* of an element must never weaken
//     verification);
//   - the schemes with no integrity protection must never report the plaintext
//     as authenticated;
//   - an error must never come back with plaintext, and the opt-in relaxation
//     must never change the bytes a strict decrypt already accepted.
//
// Every call is also bounded: a descriptor field must not drive an allocation
// or a running time out of proportion to the streams it describes.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mgilbir/spine/internal/fuzzbound"
)

// cryptoBudget bounds one decrypt attempt over an input of len(info)+len(pkg)
// bytes.
//
// Allocation: the decrypt path copies the ciphertext, then the plaintext, then
// works segment by segment. Measured on real containers: 22x on a 1.3 KB input
// (28 KB of constants — xml decoder buffers, cipher key schedules — dominating),
// falling to 2.3x at 1 MB. A 1 MiB floor absorbs the constants and a 16x rate
// covers the structural worst case, while still being some four orders of
// magnitude below a descriptor-driven blow-up, which is unbounded rather than
// merely large.
//
// Time: the slowest legitimate input is an agile descriptor at the maxSpinCount
// ceiling (1e6 iterations, up to three derivations), measured at 0.44s — see
// TestCryptoFuzzBudgetAllowsMaxSpinCount. 15s keeps ~34x headroom, enough to
// survive a -race run while still failing an unbounded loop as a finding rather
// than as a package timeout.
var cryptoBudget = fuzzbound.Budget{
	What:              "DecryptWithOptions",
	Bytes:             1 << 20,
	BytesPerInputByte: 16,
	Time:              15 * time.Second,
	TimePerMiB:        2 * time.Second,
}

const fuzzPassword = "hunter2"

// fuzzPlaintext is the payload the seed containers wrap: long enough to span
// more than one 4096-byte agile segment.
func fuzzPlaintext() []byte {
	p := make([]byte, 6000)
	for i := range p {
		p[i] = byte(i * 7)
	}
	return p
}

var fuzzDataIntegrity = regexp.MustCompile(`<dataIntegrity[^>]*/>`)

// withoutDataIntegrity deletes the whole <dataIntegrity/> element, the C361
// attack: it costs an attacker nothing, because the descriptor is not
// authenticated.
func withoutDataIntegrity(info []byte) []byte {
	return fuzzDataIntegrity.ReplaceAll(info, nil)
}

// assertDecryptContract checks the invariants every successful decrypt must
// satisfy, whatever the fuzzer fed in.
func assertDecryptContract(t *testing.T, res DecryptResult, err error, opts DecryptOptions) {
	t.Helper()
	if err != nil {
		if res.Package != nil {
			t.Fatalf("error %v returned with %d plaintext bytes", err, len(res.Package))
		}
		if res.Scheme != SchemeUnknown || res.IntegrityVerified {
			t.Fatalf("error %v returned with scheme %v / integrityVerified %v", err, res.Scheme, res.IntegrityVerified)
		}
		return
	}
	if res.Scheme == SchemeUnknown {
		t.Fatal("decrypt succeeded but reported no scheme")
	}
	if res.Scheme == SchemeAgile {
		// C361: agile is the only authenticated scheme, and the strict options
		// promise that an accepted package was authenticated. A descriptor that
		// declines to describe an HMAC is not a reason to skip one.
		if !opts.AllowMissingDataIntegrity && !res.IntegrityVerified {
			t.Fatal("strict decrypt accepted an agile package without verifying its HMAC")
		}
		return
	}
	// Standard and RC4 have no integrity protection; claiming otherwise would
	// tell a caller the bytes are authenticated when nothing checked them.
	if res.IntegrityVerified {
		t.Fatalf("scheme %v reported IntegrityVerified", res.Scheme)
	}
}

// decryptUnderBudget runs one decrypt inside the resource budget.
func decryptUnderBudget(t *testing.T, info, pkg []byte, password string, opts DecryptOptions) (DecryptResult, error) {
	t.Helper()
	var (
		res DecryptResult
		err error
	)
	cryptoBudget.Check(t, len(info)+len(pkg), func() {
		res, err = DecryptWithOptions(info, pkg, password, opts)
	})
	return res, err
}

// fuzzDecrypt is the body shared by both descriptor targets: strict decrypt
// under budget, contract assertions, and — only where the relaxation can change
// the answer — a second decrypt with AllowMissingDataIntegrity to check that it
// never turns an accepted package into different bytes.
func fuzzDecrypt(t *testing.T, info, pkg []byte, password string) {
	strict, err := decryptUnderBudget(t, info, pkg, password, DecryptOptions{})
	assertDecryptContract(t, strict, err, DecryptOptions{})

	// The opt-in only matters when the strict path either accepted the package
	// or rejected it for want of an integrity block; any other error repeats
	// identically, and re-deriving the key is the expensive part.
	if err != nil && !errors.Is(err, ErrIntegrityCheckFailed) {
		return
	}
	opts := DecryptOptions{AllowMissingDataIntegrity: true}
	relaxed, rerr := decryptUnderBudget(t, info, pkg, password, opts)
	assertDecryptContract(t, relaxed, rerr, opts)
	if err != nil {
		return
	}
	if rerr != nil {
		t.Fatalf("AllowMissingDataIntegrity rejected (%v) a package the strict decrypt accepted", rerr)
	}
	if !bytes.Equal(strict.Package, relaxed.Package) {
		t.Fatalf("AllowMissingDataIntegrity changed the recovered bytes (%d vs %d)", len(strict.Package), len(relaxed.Package))
	}
}

// FuzzAgileEncryptionInfo drives the agile descriptor parser, key derivation,
// AES-CBC decryption and HMAC verification with a fuzzed EncryptionInfo XML
// stream, a fuzzed EncryptedPackage stream and a fuzzed password.
//
// The seeds are derived from real agile containers — Encrypt output, plus the
// descriptor edits an attacker actually makes — because a fuzzer starting from
// random bytes would spend its whole budget failing the 8-byte version header.
func FuzzAgileEncryptionInfo(f *testing.F) {
	plain := fuzzPlaintext()
	info, pkg, err := Encrypt(plain, fuzzPassword)
	if err != nil {
		f.Fatalf("building agile seed: %v", err)
	}
	tampered := flipCiphertextBit(pkg)

	f.Add(info, pkg, fuzzPassword)
	f.Add(info, pkg, "wrong")
	f.Add(info, tampered, fuzzPassword)

	// The C361 family: the element deleted outright, and each of its two
	// attributes renamed so a "both present?" guard falls through.
	stripped := withoutDataIntegrity(info)
	f.Add(stripped, pkg, fuzzPassword)
	f.Add(stripped, tampered, fuzzPassword)
	f.Add(bytes.Replace(info, []byte(`encryptedHmacValue="`), []byte(`encryptedHmacXalue="`), 1), tampered, fuzzPassword)
	f.Add(bytes.Replace(info, []byte(`encryptedHmacKey="`), []byte(`encryptedHmacKez="`), 1), tampered, fuzzPassword)

	// Numeric and algorithm fields an attacker would inflate: the spinCount
	// ceiling, a saltSize/keyBits/hashSize that does not match the data, and a
	// truncated stream.
	f.Add(bytes.Replace(info, []byte(`spinCount="100000"`), []byte(`spinCount="1000001"`), 1), pkg, fuzzPassword)
	f.Add(bytes.Replace(info, []byte(`spinCount="100000"`), []byte(`spinCount="-1"`), 1), pkg, fuzzPassword)
	f.Add(bytes.Replace(info, []byte(`saltSize="16"`), []byte(`saltSize="4294967295"`), 1), pkg, fuzzPassword)
	f.Add(bytes.Replace(info, []byte(`keyBits="256"`), []byte(`keyBits="1073741824"`), 1), pkg, fuzzPassword)
	f.Add(bytes.Replace(info, []byte(`hashAlgorithm="SHA512"`), []byte(`hashAlgorithm="MD5"`), 1), pkg, fuzzPassword)
	f.Add(info[:len(info)/2], pkg, fuzzPassword)
	f.Add(info, pkg[:8], fuzzPassword)

	// An EncryptedPackage whose 8-byte size prefix lies about the plaintext
	// length.
	lying := append([]byte(nil), pkg...)
	binary.LittleEndian.PutUint64(lying[:8], 1<<62)
	f.Add(info, lying, fuzzPassword)

	// Hand-written descriptors: the version header alone, an empty document, a
	// deeply nested one, and a huge base64 attribute.
	agileHeader := "\x04\x00\x04\x00\x40\x00\x00\x00"
	f.Add([]byte(agileHeader), []byte(nil), fuzzPassword)
	f.Add([]byte(agileHeader+`<encryption/>`), pkg, fuzzPassword)
	f.Add([]byte(agileHeader+strings.Repeat("<a>", 300)+strings.Repeat("</a>", 300)), pkg, fuzzPassword)
	f.Add([]byte(agileHeader+`<encryption><keyData saltValue="`+strings.Repeat("QUJD", 4000)+`"/></encryption>`), pkg, fuzzPassword)
	f.Add([]byte(nil), []byte(nil), "")

	f.Fuzz(func(t *testing.T, info, pkg []byte, password string) {
		fuzzDecrypt(t, info, pkg, password)
	})
}

// FuzzLegacyEncryptionInfo drives the binary descriptors: the ECMA-376 standard
// EncryptionHeader and the legacy RC4 CryptoAPI header, which share a version
// prefix and are distinguished by AlgID. Their sizes (header size, salt size,
// key size) are raw little-endian fields, so they are the binary equivalent of
// the agile XML's numeric attributes.
func FuzzLegacyEncryptionInfo(f *testing.F) {
	plain := fuzzPlaintext()
	for _, bits := range []int{128, 192, 256} {
		info, pkg, err := EncryptStandard(plain, fuzzPassword, bits)
		if err != nil {
			f.Fatalf("building standard seed: %v", err)
		}
		f.Add(info, pkg, fuzzPassword)
		if bits == 256 {
			f.Add(info, pkg, "wrong")
			f.Add(info, flipCiphertextBit(pkg), fuzzPassword)

			// Inflate each 4-byte field of the header in turn: EncryptionInfo
			// flags, header size, and the AlgID/AlgIDHash/KeySize triple, plus
			// the verifier's salt size.
			for _, off := range []int{4, 8, 12, 20, 24, 28, 32} {
				if off+4 > len(info) {
					continue
				}
				bad := append([]byte(nil), info...)
				binary.LittleEndian.PutUint32(bad[off:off+4], 0xFFFFFFFF)
				f.Add(bad, pkg, fuzzPassword)
			}
			f.Add(info[:8], pkg, fuzzPassword)
			f.Add(info, pkg[:8], fuzzPassword)

			lying := append([]byte(nil), pkg...)
			binary.LittleEndian.PutUint64(lying[:8], 1<<62)
			f.Add(info, lying, fuzzPassword)
		}
	}
	for _, bits := range []int{40, 128} {
		info, pkg, err := EncryptRC4CryptoAPI(plain, fuzzPassword, bits)
		if err != nil {
			f.Fatalf("building RC4 seed: %v", err)
		}
		f.Add(info, pkg, fuzzPassword)
	}

	// Version prefixes that select the unsupported schemes (extensible, and the
	// version-1.1 binary-format RC4 that never wraps a package).
	for _, v := range [][2]uint16{{3, 3}, {4, 3}, {1, 1}, {2, 1}, {0, 0}, {0xFFFF, 0xFFFF}} {
		hdr := make([]byte, 8)
		binary.LittleEndian.PutUint16(hdr[0:2], v[0])
		binary.LittleEndian.PutUint16(hdr[2:4], v[1])
		f.Add(hdr, []byte("ciphertext"), fuzzPassword)
	}

	f.Fuzz(func(t *testing.T, info, pkg []byte, password string) {
		fuzzDecrypt(t, info, pkg, password)
	})
}
