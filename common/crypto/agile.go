package crypto

// This file implements the "agile" encryption scheme that modern Microsoft
// Office uses to password-protect OOXML documents ([MS-OFFCRYPTO] §2.3.4.10
// through §2.3.4.15). Unlike LegacyPasswordHash in legacy.go — a documented
// obfuscation that guards nothing — this is real cryptography: the document
// bytes are encrypted with AES and cannot be recovered without the password.
//
// The modern agile scheme (AES in CBC mode, SHA-512 key derivation, the
// Office 2010+ default) is implemented here; the older ECMA-376 "standard"
// scheme (§2.3.4.5, AES/SHA-1) lives in standard.go, and the legacy RC4
// CryptoAPI scheme (§2.3.5) in rc4.go. Decrypt auto-detects among them from the
// EncryptionInfo version. RC4 CryptoAPI exists here to open legacy files: the
// public save path never writes it, because RC4 is cryptographically broken
// (EncryptRC4CryptoAPI is exported only to cross-validate the decrypt path
// against a reference implementation, and says so). The extensible scheme and
// the obsolete version-1.1 binary-format RC4 (§2.3.6), which never wraps an
// OOXML package, are detected and rejected with ErrUnsupportedEncryption rather
// than decoded; see Decrypt.
//
// The implementation uses only the Go standard library's audited primitives
// (crypto/aes, crypto/cipher, crypto/sha512, crypto/hmac, crypto/rand). It
// never hand-rolls a cipher or hash.

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"hash"
	"io"
	"unicode/utf16"
	"unicode/utf8"
)

var (
	// ErrWrongPassword indicates the supplied password did not match the
	// document's stored verifier: the password is wrong (or the document is
	// corrupt). It carries no information about the correct password.
	ErrWrongPassword = errors.New("crypto: wrong password")

	// ErrUnsupportedEncryption indicates the document uses an encryption scheme
	// this package does not implement — the extensible scheme, the obsolete
	// version-1.1 binary-format RC4 scheme, or an agile/standard descriptor
	// requesting a cipher, chaining mode, or hash that is not supported. The
	// implemented schemes are agile AES-CBC (AES-128/192/256 with
	// SHA-1/256/384/512 on read; AES-256/SHA-512 on write), ECMA-376 standard
	// AES-ECB/SHA-1, and legacy RC4 CryptoAPI (§2.3.5).
	ErrUnsupportedEncryption = errors.New("crypto: unsupported encryption scheme")

	// ErrIntegrityCheckFailed indicates the encrypted package is not
	// authenticated: either its HMAC did not match — the ciphertext was
	// truncated or tampered with even though the password verified — or the
	// agile descriptor carries no usable dataIntegrity block to check it
	// against. Because the descriptor is not itself authenticated, a missing
	// dataIntegrity element is indistinguishable from one an attacker stripped,
	// so it is reported the same way; see DecryptOptions for the opt-in that
	// relaxes it. It is distinct from ErrWrongPassword: the password was right.
	ErrIntegrityCheckFailed = errors.New("crypto: encrypted package failed integrity check")

	// ErrMalformedEncryptionInfo indicates the EncryptionInfo stream could not
	// be parsed as a recognized descriptor.
	ErrMalformedEncryptionInfo = errors.New("crypto: malformed EncryptionInfo")

	// ErrInvalidPassword indicates a password handed to an encryption entry
	// point cannot be used to protect a document: it is empty, it is not valid
	// UTF-8, or it is longer than the 255 characters Office accepts. Only
	// Encrypt, EncryptStandard and EncryptRC4CryptoAPI return it; a password
	// that simply does not match an existing document returns ErrWrongPassword.
	ErrInvalidPassword = errors.New("crypto: invalid password")
)

// Block keys are fixed 8-byte constants defined by [MS-OFFCRYPTO] §2.3.4.12
// through §2.3.4.14. Each labels one derived sub-key so that a single password
// hash cannot be reused across purposes.
var (
	blockKeyVerifierHashInput = []byte{0xfe, 0xa7, 0xd2, 0x76, 0x3b, 0x4b, 0x9e, 0x79}
	blockKeyVerifierHashValue = []byte{0xd7, 0xaa, 0x0f, 0x6d, 0x30, 0x61, 0x34, 0x4e}
	blockKeyEncryptedKeyValue = []byte{0x14, 0x6e, 0x0b, 0xe7, 0xab, 0xac, 0xd0, 0xd6}
	blockKeyHmacKey           = []byte{0x5f, 0xb2, 0xad, 0x01, 0x0c, 0xb9, 0xe1, 0xf6}
	blockKeyHmacValue         = []byte{0xa0, 0x67, 0x7f, 0x02, 0xb2, 0x2c, 0x84, 0x33}
)

// packageSegmentSize is the fixed plaintext/ciphertext segment length used to
// encrypt EncryptedPackage: the stream is processed in independent AES-CBC
// segments of this many bytes, each with its own IV ([MS-OFFCRYPTO] §2.3.4.15).
const packageSegmentSize = 4096

// agileEncryptionInfo is the parsed EncryptionInfo XML descriptor.
type agileEncryptionInfo struct {
	XMLName       xml.Name           `xml:"encryption"`
	KeyData       agileKeyData       `xml:"keyData"`
	DataIntegrity agileDataIntegrity `xml:"dataIntegrity"`
	KeyEncryptors agileKeyEncryptors `xml:"keyEncryptors"`
}

type agileKeyData struct {
	SaltSize        int    `xml:"saltSize,attr"`
	BlockSize       int    `xml:"blockSize,attr"`
	KeyBits         int    `xml:"keyBits,attr"`
	HashSize        int    `xml:"hashSize,attr"`
	CipherAlgorithm string `xml:"cipherAlgorithm,attr"`
	CipherChaining  string `xml:"cipherChaining,attr"`
	HashAlgorithm   string `xml:"hashAlgorithm,attr"`
	SaltValue       string `xml:"saltValue,attr"`
}

type agileDataIntegrity struct {
	EncryptedHmacKey   string `xml:"encryptedHmacKey,attr"`
	EncryptedHmacValue string `xml:"encryptedHmacValue,attr"`
}

type agileKeyEncryptors struct {
	KeyEncryptor []agileKeyEncryptor `xml:"keyEncryptor"`
}

type agileKeyEncryptor struct {
	URI          string            `xml:"uri,attr"`
	EncryptedKey agileEncryptedKey `xml:"encryptedKey"`
}

// agileEncryptedKey is the p:encryptedKey element inside a password
// keyEncryptor. Its attributes describe the password-derived key wrapping.
type agileEncryptedKey struct {
	SpinCount                  int    `xml:"spinCount,attr"`
	SaltSize                   int    `xml:"saltSize,attr"`
	BlockSize                  int    `xml:"blockSize,attr"`
	KeyBits                    int    `xml:"keyBits,attr"`
	HashSize                   int    `xml:"hashSize,attr"`
	CipherAlgorithm            string `xml:"cipherAlgorithm,attr"`
	CipherChaining             string `xml:"cipherChaining,attr"`
	HashAlgorithm              string `xml:"hashAlgorithm,attr"`
	SaltValue                  string `xml:"saltValue,attr"`
	EncryptedVerifierHashInput string `xml:"encryptedVerifierHashInput,attr"`
	EncryptedVerifierHashValue string `xml:"encryptedVerifierHashValue,attr"`
	EncryptedKeyValue          string `xml:"encryptedKeyValue,attr"`
}

const passwordKeyEncryptorURI = "http://schemas.microsoft.com/office/2006/keyEncryptor/password"

// maxSpinCount caps the attacker-controlled spinCount read from EncryptionInfo.
// Each iteration is a SHA-512 (or shorter hash) invocation and deriveKey runs
// spinCount iterations three times before the password is verified, so a large
// value stalls a core: a few hundred descriptor bytes buy seconds of work that
// happens before anything about the caller's password is known. Office writes
// 100000; this ceiling allows ten times that — generous for any legitimate
// producer — while keeping the pre-check work under a second on current
// hardware.
const maxSpinCount = 1_000_000

// Scheme identifies which [MS-OFFCRYPTO] password-encryption scheme produced an
// encrypted container. DecryptWithOptions reports it so a caller can tell an
// authenticated agile package from the older schemes, which carry no integrity
// protection at all.
type Scheme int

const (
	// SchemeUnknown is the zero value. DecryptWithOptions never reports it
	// together with a nil error.
	SchemeUnknown Scheme = iota

	// SchemeAgile is agile encryption ([MS-OFFCRYPTO] §2.3.4.10–§2.3.4.15):
	// AES-CBC with per-segment IVs and an iterated key derivation. It is the
	// only scheme that authenticates the ciphertext (an HMAC over the
	// EncryptedPackage stream, keyed from the document key).
	SchemeAgile

	// SchemeStandard is ECMA-376 standard encryption (§2.3.4.5–§2.3.4.9):
	// AES-ECB with SHA-1 key derivation. It has no integrity protection, so a
	// successful decryption proves only that the password matched the stored
	// verifier — not that the bytes are the ones the author encrypted.
	SchemeStandard

	// SchemeRC4CryptoAPI is the obsolete RC4 CryptoAPI scheme (§2.3.5). The
	// cipher is broken, the key derivation is a single un-iterated SHA-1, and
	// there is no integrity protection. Decrypt-only; treat the plaintext as
	// unauthenticated.
	SchemeRC4CryptoAPI
)

// String renders the scheme name used in this package's documentation.
func (s Scheme) String() string {
	switch s {
	case SchemeAgile:
		return "agile"
	case SchemeStandard:
		return "ECMA-376 standard"
	case SchemeRC4CryptoAPI:
		return "RC4 CryptoAPI"
	default:
		return "unknown"
	}
}

// DecryptOptions configures DecryptWithOptions. The zero value is the strict,
// recommended behavior and matches Decrypt.
type DecryptOptions struct {
	// AllowMissingDataIntegrity accepts an agile-encrypted package whose
	// EncryptionInfo descriptor carries no dataIntegrity element, decrypting it
	// WITHOUT verifying the package HMAC.
	//
	// Leave it false unless you know you need it. The descriptor is plaintext
	// and is not covered by any MAC, verifier, or key derivation, so an
	// attacker who can modify the file can delete the dataIntegrity element as
	// easily as they can flip bits in the (malleable, CBC-mode) ciphertext.
	// Honoring its absence therefore turns an authenticated format into an
	// unauthenticated one at the attacker's option, which is why the default is
	// to reject it with ErrIntegrityCheckFailed. Office always emits
	// dataIntegrity; this exists only for a third-party producer that does not,
	// and it must be a deliberate caller decision — it is never inferred from
	// the file. It never relaxes a *failed* HMAC, and never accepts a
	// half-present dataIntegrity block (one of the two attributes missing),
	// which no honest producer writes.
	AllowMissingDataIntegrity bool
}

// DecryptResult is the outcome of DecryptWithOptions: the recovered package
// plus what is known about how it was protected.
type DecryptResult struct {
	// Package is the plaintext OOXML package (an ordinary .zip).
	Package []byte

	// Scheme is the encryption scheme the container used.
	Scheme Scheme

	// IntegrityVerified reports whether the returned bytes were authenticated
	// with a cryptographic checksum before being returned. It is true only for
	// SchemeAgile packages whose dataIntegrity HMAC verified. When it is false
	// the plaintext is unauthenticated: the password was right, but nothing
	// proves the bytes are the ones the author encrypted, so treat the package
	// as untrusted input.
	IntegrityVerified bool
}

// Decrypt recovers the plaintext OOXML package (an ordinary .zip) from the two
// streams of an encrypted document container: the EncryptionInfo descriptor
// (including its 8-byte version header) and the EncryptedPackage ciphertext.
//
// It dispatches on the EncryptionInfo version: agile encryption (major=4,
// minor=4), ECMA-376 standard encryption (minor=2, AES), and legacy RC4 CryptoAPI
// encryption (minor=2, RC4 — [MS-OFFCRYPTO] §2.3.5) are supported. The extensible
// scheme and the older version-1.1 binary-format RC4 (§2.3.6) return
// ErrUnsupportedEncryption. A wrong password returns ErrWrongPassword. An agile
// package whose HMAC does not verify — or whose descriptor carries no
// dataIntegrity block to verify against — returns ErrIntegrityCheckFailed.
//
// Only agile packages are authenticated; the standard and RC4 schemes have no
// integrity protection, so their plaintext is attacker-malleable even when the
// password is correct. Use DecryptWithOptions to learn which scheme produced the
// bytes and whether they were authenticated.
func Decrypt(encryptionInfo, encryptedPackage []byte, password string) ([]byte, error) {
	res, err := DecryptWithOptions(encryptionInfo, encryptedPackage, password, DecryptOptions{})
	if err != nil {
		return nil, err
	}
	return res.Package, nil
}

// DecryptWithOptions is Decrypt with control over how strictly the agile
// integrity block is required, and with the scheme and authentication status of
// the recovered bytes reported back.
func DecryptWithOptions(encryptionInfo, encryptedPackage []byte, password string, opts DecryptOptions) (DecryptResult, error) {
	if len(encryptionInfo) < 8 {
		return DecryptResult{}, fmt.Errorf("%w: stream is %d bytes, need at least 8", ErrMalformedEncryptionInfo, len(encryptionInfo))
	}
	major := binary.LittleEndian.Uint16(encryptionInfo[0:2])
	minor := binary.LittleEndian.Uint16(encryptionInfo[2:4])

	switch {
	case major == 4 && minor == 4:
		plain, verified, err := agileDecrypt(encryptionInfo[8:], encryptedPackage, password, opts)
		if err != nil {
			return DecryptResult{}, err
		}
		return DecryptResult{Package: plain, Scheme: SchemeAgile, IntegrityVerified: verified}, nil
	case (major == 2 || major == 3 || major == 4) && minor == 2:
		// Minor version 2 covers both the AES-based ECMA-376 "standard" scheme and
		// legacy RC4 CryptoAPI ([MS-OFFCRYPTO] §2.3.5); they share the binary
		// EncryptionHeader layout and are distinguished by its AlgID (and, when
		// that is zero, by the fAES flag). The 4-byte version prefix is followed
		// directly by that header.
		info := encryptionInfo[4:]
		if isRC4CryptoAPI(info) {
			plain, err := rc4CryptoAPIDecrypt(info, encryptedPackage, password)
			if err != nil {
				return DecryptResult{}, err
			}
			return DecryptResult{Package: plain, Scheme: SchemeRC4CryptoAPI}, nil
		}
		plain, err := standardDecrypt(info, encryptedPackage, password)
		if err != nil {
			return DecryptResult{}, err
		}
		return DecryptResult{Package: plain, Scheme: SchemeStandard}, nil
	case (major == 3 || major == 4) && minor == 3:
		return DecryptResult{}, fmt.Errorf("%w: extensible encryption (version %d.%d) is not supported", ErrUnsupportedEncryption, major, minor)
	case minor == 1:
		// Version-1.1 RC4 ([MS-OFFCRYPTO] §2.3.6) is the obsolete binary-format
		// (.doc/.xls/.ppt) scheme; it never wraps an OOXML .zip package, so it is
		// identified but not decoded here (RC4 CryptoAPI, the scheme that can wrap
		// OOXML, is handled by the minor==2 case above).
		return DecryptResult{}, fmt.Errorf("%w: version-1.1 binary-format RC4 encryption (version %d.%d) does not wrap OOXML packages and is not supported", ErrUnsupportedEncryption, major, minor)
	default:
		return DecryptResult{}, fmt.Errorf("%w: unrecognized EncryptionInfo version %d.%d", ErrUnsupportedEncryption, major, minor)
	}
}

// Encrypt produces the two streams of an agile-encrypted document container
// from a plaintext OOXML package (an ordinary .zip): the EncryptionInfo
// descriptor (with its 8-byte version header) and the EncryptedPackage
// ciphertext. It always uses the modern Office defaults — AES-256 in CBC mode
// with SHA-512 key derivation — and generates fresh random salts and keys with
// crypto/rand on every call, so encrypting the same package twice yields
// different ciphertext. The descriptor always carries a dataIntegrity block, so
// the result is authenticated.
//
// The password must be a non-empty, valid-UTF-8 string of at most 255
// characters (see ErrInvalidPassword): those are the passwords Office can
// represent, and encrypting under any other string would produce a document no
// Office build could open.
func Encrypt(packageData []byte, password string) (encryptionInfo, encryptedPackage []byte, err error) {
	if err := validatePassword(password); err != nil {
		return nil, nil, err
	}
	return agileEncrypt(packageData, password)
}

// maxPasswordUTF16Units is the longest password Office accepts, in UTF-16 code
// units — the units its password fields are counted and hashed in. Office's
// password prompts cap input at 255 characters, so a longer string cannot be
// entered on the other side of the interop boundary and is rejected here rather
// than silently producing an unopenable document.
const maxPasswordUTF16Units = 255

// validatePassword rejects passwords that cannot round-trip through Office.
// It guards the encryption entry points only: the decrypt path must accept
// whatever the caller has, and reports a non-matching password as
// ErrWrongPassword.
func validatePassword(password string) error {
	if password == "" {
		return fmt.Errorf("%w: the password is empty; encrypting with an empty password protects nothing", ErrInvalidPassword)
	}
	if !utf8.ValidString(password) {
		return fmt.Errorf("%w: the password is not valid UTF-8; its invalid bytes would be hashed as U+FFFD replacement characters and no other implementation would derive the same key", ErrInvalidPassword)
	}
	if n := len(utf16.Encode([]rune(password))); n > maxPasswordUTF16Units {
		return fmt.Errorf("%w: the password is %d UTF-16 characters, exceeding the %d Office accepts", ErrInvalidPassword, n, maxPasswordUTF16Units)
	}
	return nil
}

// hashFactory returns a constructor for the named hash algorithm and its digest
// size. Agile descriptors name the algorithm in the XML; SHA-512 is the modern
// default, but SHA-1/256/384 appear in older agile files and cost nothing to
// support on the read path.
func hashFactory(name string) (func() hash.Hash, int, error) {
	switch name {
	case "SHA512", "SHA-512":
		return sha512.New, 64, nil
	case "SHA384", "SHA-384":
		return sha512.New384, 48, nil
	case "SHA256", "SHA-256":
		return sha256.New, 32, nil
	case "SHA1", "SHA-1":
		return sha1.New, 20, nil
	default:
		return nil, 0, fmt.Errorf("%w: hash algorithm %q", ErrUnsupportedEncryption, name)
	}
}

// agileParams holds the validated numeric/cipher parameters shared by the
// keyData and p:encryptedKey blocks after their algorithm names are checked.
type agileParams struct {
	newHash   func() hash.Hash
	hashSize  int
	keyBytes  int // AES key length in bytes (keyBits/8)
	blockSize int
}

// validateCommon checks the cipher, chaining, and hash names and resolves the
// numeric parameters. Only AES in CBC mode is supported.
func validateCommon(cipherAlg, chaining, hashAlg string, keyBits, blockSize, hashSize int) (agileParams, error) {
	if cipherAlg != "AES" {
		return agileParams{}, fmt.Errorf("%w: cipher algorithm %q (only AES is supported)", ErrUnsupportedEncryption, cipherAlg)
	}
	if chaining != "ChainingModeCBC" {
		return agileParams{}, fmt.Errorf("%w: cipher chaining %q (only ChainingModeCBC is supported)", ErrUnsupportedEncryption, chaining)
	}
	newHash, digestSize, err := hashFactory(hashAlg)
	if err != nil {
		return agileParams{}, err
	}
	switch keyBits {
	case 128, 192, 256:
	default:
		return agileParams{}, fmt.Errorf("%w: AES key size %d bits", ErrUnsupportedEncryption, keyBits)
	}
	if blockSize != aes.BlockSize {
		return agileParams{}, fmt.Errorf("%w: block size %d (AES uses %d)", ErrUnsupportedEncryption, blockSize, aes.BlockSize)
	}
	if hashSize != 0 && hashSize != digestSize {
		return agileParams{}, fmt.Errorf("%w: declared hashSize %d does not match %s digest size %d", ErrMalformedEncryptionInfo, hashSize, hashAlg, digestSize)
	}
	return agileParams{newHash: newHash, hashSize: digestSize, keyBytes: keyBits / 8, blockSize: blockSize}, nil
}

// agileDecrypt implements the agile decryption pipeline over the XML portion of
// EncryptionInfo (version header already stripped) and the EncryptedPackage. It
// reports whether the returned plaintext was authenticated by the package HMAC,
// which is true for every descriptor it accepts unless the caller opted out of
// requiring one.
func agileDecrypt(infoXML, encryptedPackage []byte, password string, opts DecryptOptions) ([]byte, bool, error) {
	var info agileEncryptionInfo
	//xmlguard:lenient EncryptionInfo is not an OPC part; it is the descriptor read out of the encrypted container, and its handling has its own budgets and checks
	if err := xml.Unmarshal(infoXML, &info); err != nil {
		return nil, false, fmt.Errorf("%w: %v", ErrMalformedEncryptionInfo, err)
	}

	enc, err := findPasswordEncryptor(&info)
	if err != nil {
		return nil, false, err
	}

	// Decide up front whether this descriptor is one we are willing to trust at
	// all. The dataIntegrity block is the only thing that authenticates the
	// ciphertext, and nothing authenticates the descriptor itself, so a missing
	// block must be refused rather than treated as "integrity not requested" —
	// see checkDataIntegrity.
	checkHmac, err := checkDataIntegrity(info.DataIntegrity, opts.AllowMissingDataIntegrity)
	if err != nil {
		return nil, false, err
	}

	// Reject an over-large spinCount before any key derivation runs: deriveKey
	// hashes spinCount times and is called three times below, all before the
	// password is checked.
	if enc.SpinCount < 0 || enc.SpinCount > maxSpinCount {
		return nil, false, fmt.Errorf("%w: spinCount %d out of range [0, %d]", ErrMalformedEncryptionInfo, enc.SpinCount, maxSpinCount)
	}

	kp, err := validateCommon(enc.CipherAlgorithm, enc.CipherChaining, enc.HashAlgorithm, enc.KeyBits, enc.BlockSize, enc.HashSize)
	if err != nil {
		return nil, false, err
	}
	dp, err := validateCommon(info.KeyData.CipherAlgorithm, info.KeyData.CipherChaining, info.KeyData.HashAlgorithm, info.KeyData.KeyBits, info.KeyData.BlockSize, info.KeyData.HashSize)
	if err != nil {
		return nil, false, err
	}

	pkSalt, err := decodeB64(enc.SaltValue, "encryptedKey saltValue")
	if err != nil {
		return nil, false, err
	}
	keyDataSalt, err := decodeB64(info.KeyData.SaltValue, "keyData saltValue")
	if err != nil {
		return nil, false, err
	}
	encVerInput, err := decodeB64(enc.EncryptedVerifierHashInput, "encryptedVerifierHashInput")
	if err != nil {
		return nil, false, err
	}
	encVerValue, err := decodeB64(enc.EncryptedVerifierHashValue, "encryptedVerifierHashValue")
	if err != nil {
		return nil, false, err
	}
	encKeyValue, err := decodeB64(enc.EncryptedKeyValue, "encryptedKeyValue")
	if err != nil {
		return nil, false, err
	}

	pwUTF16 := passwordToUTF16LE(password)

	// Verify the password against the stored verifier before trusting any key.
	verInputKey, err := deriveKey(kp, pkSalt, pwUTF16, enc.SpinCount, blockKeyVerifierHashInput)
	if err != nil {
		return nil, false, err
	}
	verInput, err := aesCBCDecrypt(verInputKey, pkSalt, encVerInput)
	if err != nil {
		return nil, false, err
	}
	verValueKey, err := deriveKey(kp, pkSalt, pwUTF16, enc.SpinCount, blockKeyVerifierHashValue)
	if err != nil {
		return nil, false, err
	}
	verValue, err := aesCBCDecrypt(verValueKey, pkSalt, encVerValue)
	if err != nil {
		return nil, false, err
	}
	h := kp.newHash()
	h.Write(verInput)
	if !hmac.Equal(h.Sum(nil), truncate(verValue, kp.hashSize)) {
		return nil, false, ErrWrongPassword
	}

	// Decrypt the intermediate key that actually protects the package.
	keyValueKey, err := deriveKey(kp, pkSalt, pwUTF16, enc.SpinCount, blockKeyEncryptedKeyValue)
	if err != nil {
		return nil, false, err
	}
	secretKey, err := aesCBCDecrypt(keyValueKey, pkSalt, encKeyValue)
	if err != nil {
		return nil, false, err
	}
	secretKey = truncate(secretKey, dp.keyBytes)

	// Authenticate the ciphertext before decrypting it: AES-CBC is malleable and
	// the plaintext is handed on as a package, so tampering or truncation must be
	// caught here, not by whatever parses the result.
	if checkHmac {
		if err := verifyIntegrity(&info, dp, secretKey, keyDataSalt, encryptedPackage); err != nil {
			return nil, false, err
		}
	}

	plain, err := decryptPackage(dp, secretKey, keyDataSalt, encryptedPackage)
	if err != nil {
		return nil, false, err
	}
	return plain, checkHmac, nil
}

// checkDataIntegrity decides whether an agile descriptor's dataIntegrity block
// can and must be verified, and reports the descriptor as unauthenticated when
// it cannot be.
//
// The block is what binds the EncryptedPackage bytes to the document key. It is
// not itself authenticated — the descriptor is plaintext and no MAC, verifier
// or key derivation covers it — so an attacker who can flip bits in the
// (CBC-mode, malleable) ciphertext can equally delete the element or rename one
// of its attributes. Verifying "when present" therefore means verifying only
// when the attacker permits it, which is no protection at all: it is refused
// instead. Office always writes dataIntegrity; a caller who must read a
// third-party producer that does not can opt out explicitly, per file, with
// DecryptOptions.AllowMissingDataIntegrity — never inferred from the bytes.
//
// A half-present block (exactly one of the two attributes) is refused even
// under the opt-out: no producer writes one, but stripping one attribute is the
// cheapest way to make an "if both present" check skip.
func checkDataIntegrity(di agileDataIntegrity, allowMissing bool) (verify bool, err error) {
	haveKey := di.EncryptedHmacKey != ""
	haveValue := di.EncryptedHmacValue != ""
	switch {
	case haveKey && haveValue:
		return true, nil
	case haveKey != haveValue:
		return false, fmt.Errorf("%w: the dataIntegrity block declares only %s, so the package HMAC cannot be checked",
			ErrIntegrityCheckFailed, presentHmacAttr(haveKey))
	case allowMissing:
		return false, nil
	default:
		return false, fmt.Errorf("%w: the descriptor carries no dataIntegrity block, so the package HMAC cannot be checked; "+
			"an agile encryptor always writes one and stripping it is how tampered ciphertext is made to look valid "+
			"(set DecryptOptions.AllowMissingDataIntegrity to accept unauthenticated agile packages anyway)",
			ErrIntegrityCheckFailed)
	}
}

// presentHmacAttr names the one dataIntegrity attribute that was supplied.
func presentHmacAttr(haveKey bool) string {
	if haveKey {
		return "encryptedHmacKey"
	}
	return "encryptedHmacValue"
}

// findPasswordEncryptor returns the password keyEncryptor, the only kind this
// package handles (certificate keyEncryptors are rejected).
func findPasswordEncryptor(info *agileEncryptionInfo) (*agileEncryptedKey, error) {
	for i := range info.KeyEncryptors.KeyEncryptor {
		if info.KeyEncryptors.KeyEncryptor[i].URI == passwordKeyEncryptorURI {
			return &info.KeyEncryptors.KeyEncryptor[i].EncryptedKey, nil
		}
	}
	return nil, fmt.Errorf("%w: no password keyEncryptor present (certificate-only protection is not supported)", ErrUnsupportedEncryption)
}

// verifyIntegrity checks the EncryptedPackage HMAC recorded in the
// dataIntegrity block.
func verifyIntegrity(info *agileEncryptionInfo, dp agileParams, secretKey, keyDataSalt, encryptedPackage []byte) error {
	encHmacKey, err := decodeB64(info.DataIntegrity.EncryptedHmacKey, "encryptedHmacKey")
	if err != nil {
		return err
	}
	encHmacValue, err := decodeB64(info.DataIntegrity.EncryptedHmacValue, "encryptedHmacValue")
	if err != nil {
		return err
	}
	ivKey, err := ivFromSalt(dp, keyDataSalt, blockKeyHmacKey)
	if err != nil {
		return err
	}
	hmacKey, err := aesCBCDecrypt(secretKey, ivKey, encHmacKey)
	if err != nil {
		return err
	}
	hmacKey = truncate(hmacKey, dp.hashSize)

	ivValue, err := ivFromSalt(dp, keyDataSalt, blockKeyHmacValue)
	if err != nil {
		return err
	}
	storedHmac, err := aesCBCDecrypt(secretKey, ivValue, encHmacValue)
	if err != nil {
		return err
	}
	storedHmac = truncate(storedHmac, dp.hashSize)

	mac := hmac.New(dp.newHash, hmacKey)
	mac.Write(encryptedPackage)
	if !hmac.Equal(mac.Sum(nil), storedHmac) {
		return ErrIntegrityCheckFailed
	}
	return nil
}

// decryptPackage AES-CBC-decrypts the EncryptedPackage stream segment by
// segment and truncates the result to the plaintext length stored in the
// stream's 8-byte little-endian prefix.
func decryptPackage(dp agileParams, secretKey, keyDataSalt, encryptedPackage []byte) ([]byte, error) {
	if len(encryptedPackage) < 8 {
		return nil, fmt.Errorf("%w: EncryptedPackage is %d bytes, need at least 8", ErrMalformedEncryptionInfo, len(encryptedPackage))
	}
	totalSize := binary.LittleEndian.Uint64(encryptedPackage[:8])
	ciphertext := encryptedPackage[8:]
	if totalSize > uint64(len(ciphertext)) {
		return nil, fmt.Errorf("%w: declared plaintext size %d exceeds ciphertext length %d", ErrMalformedEncryptionInfo, totalSize, len(ciphertext))
	}
	if len(ciphertext)%dp.blockSize != 0 {
		return nil, fmt.Errorf("%w: EncryptedPackage ciphertext length %d is not a multiple of the %d-byte block size", ErrMalformedEncryptionInfo, len(ciphertext), dp.blockSize)
	}

	out := make([]byte, 0, len(ciphertext))
	for segIndex := 0; segIndex*packageSegmentSize < len(ciphertext); segIndex++ {
		start := segIndex * packageSegmentSize
		end := start + packageSegmentSize
		if end > len(ciphertext) {
			end = len(ciphertext)
		}
		iv, err := ivFromSalt(dp, keyDataSalt, segmentBlockKey(segIndex))
		if err != nil {
			return nil, err
		}
		plain, err := aesCBCDecrypt(secretKey, iv, ciphertext[start:end])
		if err != nil {
			return nil, err
		}
		out = append(out, plain...)
	}
	return out[:totalSize], nil
}

// agileEncrypt implements the agile encryption pipeline with the modern Office
// defaults (AES-256, SHA-512).
func agileEncrypt(packageData []byte, password string) (encryptionInfo, encryptedPackage []byte, err error) {
	const (
		keyBits   = 256
		keyBytes  = keyBits / 8
		blockSize = aes.BlockSize
		saltSize  = 16
		spinCount = 100000
	)
	dp := agileParams{newHash: sha512.New, hashSize: 64, keyBytes: keyBytes, blockSize: blockSize}
	kp := dp

	keyDataSalt, err := randomBytes(saltSize)
	if err != nil {
		return nil, nil, err
	}
	pkSalt, err := randomBytes(saltSize)
	if err != nil {
		return nil, nil, err
	}
	secretKey, err := randomBytes(keyBytes)
	if err != nil {
		return nil, nil, err
	}
	verifierInput, err := randomBytes(saltSize)
	if err != nil {
		return nil, nil, err
	}

	pwUTF16 := passwordToUTF16LE(password)

	// Password verifier: store the verifier bytes and their hash, each wrapped
	// under a distinct password-derived key.
	verInputKey, err := deriveKey(kp, pkSalt, pwUTF16, spinCount, blockKeyVerifierHashInput)
	if err != nil {
		return nil, nil, err
	}
	encVerInput, err := aesCBCEncrypt(verInputKey, pkSalt, pad(verifierInput, blockSize))
	if err != nil {
		return nil, nil, err
	}
	hv := dp.newHash()
	hv.Write(verifierInput)
	verifierHash := hv.Sum(nil)
	verValueKey, err := deriveKey(kp, pkSalt, pwUTF16, spinCount, blockKeyVerifierHashValue)
	if err != nil {
		return nil, nil, err
	}
	encVerValue, err := aesCBCEncrypt(verValueKey, pkSalt, pad(verifierHash, blockSize))
	if err != nil {
		return nil, nil, err
	}

	// Wrap the intermediate key with a third password-derived key.
	keyValueKey, err := deriveKey(kp, pkSalt, pwUTF16, spinCount, blockKeyEncryptedKeyValue)
	if err != nil {
		return nil, nil, err
	}
	encKeyValue, err := aesCBCEncrypt(keyValueKey, pkSalt, pad(secretKey, blockSize))
	if err != nil {
		return nil, nil, err
	}

	// Encrypt the package itself.
	encryptedPackage, err = encryptPackage(dp, secretKey, keyDataSalt, packageData)
	if err != nil {
		return nil, nil, err
	}

	// Compute the dataIntegrity HMAC over the finished EncryptedPackage stream.
	hmacKey, err := randomBytes(dp.hashSize)
	if err != nil {
		return nil, nil, err
	}
	mac := hmac.New(dp.newHash, hmacKey)
	mac.Write(encryptedPackage)
	hmacValue := mac.Sum(nil)

	ivHmacKey, err := ivFromSalt(dp, keyDataSalt, blockKeyHmacKey)
	if err != nil {
		return nil, nil, err
	}
	encHmacKey, err := aesCBCEncrypt(secretKey, ivHmacKey, pad(hmacKey, blockSize))
	if err != nil {
		return nil, nil, err
	}
	ivHmacValue, err := ivFromSalt(dp, keyDataSalt, blockKeyHmacValue)
	if err != nil {
		return nil, nil, err
	}
	encHmacValue, err := aesCBCEncrypt(secretKey, ivHmacValue, pad(hmacValue, blockSize))
	if err != nil {
		return nil, nil, err
	}

	info := agileEncryptionInfo{
		KeyData: agileKeyData{
			SaltSize:        saltSize,
			BlockSize:       blockSize,
			KeyBits:         keyBits,
			HashSize:        dp.hashSize,
			CipherAlgorithm: "AES",
			CipherChaining:  "ChainingModeCBC",
			HashAlgorithm:   "SHA512",
			SaltValue:       encodeB64(keyDataSalt),
		},
		DataIntegrity: agileDataIntegrity{
			EncryptedHmacKey:   encodeB64(encHmacKey),
			EncryptedHmacValue: encodeB64(encHmacValue),
		},
		KeyEncryptors: agileKeyEncryptors{
			KeyEncryptor: []agileKeyEncryptor{{
				URI: passwordKeyEncryptorURI,
				EncryptedKey: agileEncryptedKey{
					SpinCount:                  spinCount,
					SaltSize:                   saltSize,
					BlockSize:                  blockSize,
					KeyBits:                    keyBits,
					HashSize:                   dp.hashSize,
					CipherAlgorithm:            "AES",
					CipherChaining:             "ChainingModeCBC",
					HashAlgorithm:              "SHA512",
					SaltValue:                  encodeB64(pkSalt),
					EncryptedVerifierHashInput: encodeB64(encVerInput),
					EncryptedVerifierHashValue: encodeB64(encVerValue),
					EncryptedKeyValue:          encodeB64(encKeyValue),
				},
			}},
		},
	}

	encryptionInfo, err = marshalEncryptionInfo(&info)
	if err != nil {
		return nil, nil, err
	}
	return encryptionInfo, encryptedPackage, nil
}

// encryptPackage AES-CBC-encrypts packageData segment by segment and prepends
// the 8-byte little-endian plaintext length.
func encryptPackage(dp agileParams, secretKey, keyDataSalt, packageData []byte) ([]byte, error) {
	out := make([]byte, 8, 8+len(packageData)+dp.blockSize)
	binary.LittleEndian.PutUint64(out, uint64(len(packageData)))

	for segIndex := 0; segIndex*packageSegmentSize < len(packageData); segIndex++ {
		start := segIndex * packageSegmentSize
		end := start + packageSegmentSize
		if end > len(packageData) {
			end = len(packageData)
		}
		seg := pad(packageData[start:end], dp.blockSize)
		iv, err := ivFromSalt(dp, keyDataSalt, segmentBlockKey(segIndex))
		if err != nil {
			return nil, err
		}
		enc, err := aesCBCEncrypt(secretKey, iv, seg)
		if err != nil {
			return nil, err
		}
		out = append(out, enc...)
	}
	return out, nil
}

// marshalEncryptionInfo serializes the descriptor with the correct 8-byte
// agile version header and namespaced element/attribute names. The XML is
// hand-built rather than produced by encoding/xml so the p: prefix and the two
// namespace declarations match what Office writes.
func marshalEncryptionInfo(info *agileEncryptionInfo) ([]byte, error) {
	kd := info.KeyData
	di := info.DataIntegrity
	ek := info.KeyEncryptors.KeyEncryptor[0].EncryptedKey

	var b []byte
	// Version header: major=4, minor=4, reserved flags=0x40.
	b = append(b, 0x04, 0x00, 0x04, 0x00, 0x40, 0x00, 0x00, 0x00)
	b = append(b, []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)...)
	b = append(b, "\r\n"...)
	b = append(b, []byte(`<encryption xmlns="http://schemas.microsoft.com/office/2006/encryption" xmlns:p="http://schemas.microsoft.com/office/2006/keyEncryptor/password" xmlns:c="http://schemas.microsoft.com/office/2006/keyEncryptor/certificate">`)...)
	b = append(b, fmt.Sprintf(`<keyData saltSize="%d" blockSize="%d" keyBits="%d" hashSize="%d" cipherAlgorithm="%s" cipherChaining="%s" hashAlgorithm="%s" saltValue="%s"/>`,
		kd.SaltSize, kd.BlockSize, kd.KeyBits, kd.HashSize, kd.CipherAlgorithm, kd.CipherChaining, kd.HashAlgorithm, kd.SaltValue)...)
	b = append(b, fmt.Sprintf(`<dataIntegrity encryptedHmacKey="%s" encryptedHmacValue="%s"/>`,
		di.EncryptedHmacKey, di.EncryptedHmacValue)...)
	b = append(b, []byte(`<keyEncryptors><keyEncryptor uri="http://schemas.microsoft.com/office/2006/keyEncryptor/password">`)...)
	b = append(b, fmt.Sprintf(`<p:encryptedKey spinCount="%d" saltSize="%d" blockSize="%d" keyBits="%d" hashSize="%d" cipherAlgorithm="%s" cipherChaining="%s" hashAlgorithm="%s" saltValue="%s" encryptedVerifierHashInput="%s" encryptedVerifierHashValue="%s" encryptedKeyValue="%s"/>`,
		ek.SpinCount, ek.SaltSize, ek.BlockSize, ek.KeyBits, ek.HashSize, ek.CipherAlgorithm, ek.CipherChaining, ek.HashAlgorithm,
		ek.SaltValue, ek.EncryptedVerifierHashInput, ek.EncryptedVerifierHashValue, ek.EncryptedKeyValue)...)
	b = append(b, []byte(`</keyEncryptor></keyEncryptors></encryption>`)...)
	return b, nil
}

// deriveKey computes the agile password-derived key for one block key
// ([MS-OFFCRYPTO] §2.3.4.11): H0 = hash(salt‖password); iterate spinCount times
// prepending the little-endian counter; then hash once more with the block key
// and take the leading keyBytes, 0x36-padding a short digest.
func deriveKey(p agileParams, salt, passwordUTF16 []byte, spinCount int, blockKey []byte) ([]byte, error) {
	// Defense in depth: agileDecrypt bounds spinCount before calling in, but
	// guard here too so no caller can drive an unbounded derivation loop.
	if spinCount < 0 || spinCount > maxSpinCount {
		return nil, fmt.Errorf("%w: spinCount %d out of range [0, %d]", ErrMalformedEncryptionInfo, spinCount, maxSpinCount)
	}
	h := p.newHash()
	h.Write(salt)
	h.Write(passwordUTF16)
	digest := h.Sum(nil)

	var counter [4]byte
	for i := 0; i < spinCount; i++ {
		binary.LittleEndian.PutUint32(counter[:], uint32(i))
		h.Reset()
		h.Write(counter[:])
		h.Write(digest)
		digest = h.Sum(digest[:0])
	}

	h.Reset()
	h.Write(digest)
	h.Write(blockKey)
	final := h.Sum(nil)

	key := make([]byte, p.keyBytes)
	n := copy(key, final)
	for ; n < p.keyBytes; n++ {
		key[n] = 0x36
	}
	return key, nil
}

// ivFromSalt derives a per-block IV: hash(salt‖blockKey) truncated to the block
// size ([MS-OFFCRYPTO] §2.3.4.12).
func ivFromSalt(p agileParams, salt, blockKey []byte) ([]byte, error) {
	h := p.newHash()
	h.Write(salt)
	h.Write(blockKey)
	digest := h.Sum(nil)
	iv := make([]byte, p.blockSize)
	n := copy(iv, digest)
	for ; n < p.blockSize; n++ {
		iv[n] = 0x36
	}
	return iv, nil
}

// segmentBlockKey renders a package segment index as the 4-byte little-endian
// block key used to derive that segment's IV.
func segmentBlockKey(segIndex int) []byte {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(segIndex))
	return b[:]
}

// aesCBCDecrypt decrypts ciphertext with AES-CBC and no padding removal. It
// requires a full-block ciphertext and a block-length IV.
func aesCBCDecrypt(key, iv, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(iv) < block.BlockSize() {
		return nil, fmt.Errorf("%w: IV is %d bytes, need %d", ErrMalformedEncryptionInfo, len(iv), block.BlockSize())
	}
	if len(ciphertext) == 0 || len(ciphertext)%block.BlockSize() != 0 {
		return nil, fmt.Errorf("%w: ciphertext length %d is not a positive multiple of the %d-byte block size", ErrMalformedEncryptionInfo, len(ciphertext), block.BlockSize())
	}
	mode := cipher.NewCBCDecrypter(block, iv[:block.BlockSize()])
	out := make([]byte, len(ciphertext))
	mode.CryptBlocks(out, ciphertext)
	return out, nil
}

// aesCBCEncrypt encrypts a block-aligned plaintext with AES-CBC.
func aesCBCEncrypt(key, iv, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(iv) < block.BlockSize() {
		return nil, fmt.Errorf("crypto: IV is %d bytes, need %d", len(iv), block.BlockSize())
	}
	if len(plaintext)%block.BlockSize() != 0 {
		return nil, fmt.Errorf("crypto: plaintext length %d is not a multiple of the %d-byte block size", len(plaintext), block.BlockSize())
	}
	mode := cipher.NewCBCEncrypter(block, iv[:block.BlockSize()])
	out := make([]byte, len(plaintext))
	mode.CryptBlocks(out, plaintext)
	return out, nil
}

// pad zero-extends data up to the next multiple of blockSize. Agile encryption
// pads short trailing blocks with zero bytes; the true length is recorded
// separately (the package size prefix, or the fixed hash/key lengths), so the
// padding is never interpreted as data.
func pad(data []byte, blockSize int) []byte {
	if len(data)%blockSize == 0 {
		return data
	}
	padded := make([]byte, (len(data)/blockSize+1)*blockSize)
	copy(padded, data)
	return padded
}

// truncate returns the first n bytes of b, or all of b when it is shorter.
func truncate(b []byte, n int) []byte {
	if n >= len(b) {
		return b
	}
	return b[:n]
}

// passwordToUTF16LE encodes the password as little-endian UTF-16 without a BOM,
// the form every [MS-OFFCRYPTO] key derivation hashes. Non-BMP runes become
// surrogate pairs, exactly as they appear in the UTF-16 strings Office hashes.
//
// Two properties are deliberate and are why the encryption entry points
// validate their password first (see validatePassword):
//
//   - The whole string is encoded; there is no truncation. Office's password
//     prompts stop at 255 characters, so no Office-written document was keyed
//     from a truncated longer string, and silently dropping the tail here would
//     make two different passwords open the same file.
//   - Invalid UTF-8 yields one U+FFFD per bad byte (Go's []rune conversion).
//     That is a lossy, spine-specific reading of bytes Office cannot represent,
//     so it is rejected on the write path rather than baked into a key. On the
//     read path it is harmless: such a password simply fails the verifier and
//     Decrypt reports ErrWrongPassword.
func passwordToUTF16LE(password string) []byte {
	units := utf16.Encode([]rune(password))
	out := make([]byte, 0, len(units)*2)
	for _, u := range units {
		out = append(out, byte(u), byte(u>>8))
	}
	return out
}

func decodeB64(s, field string) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("%w: %s is not valid base64: %v", ErrMalformedEncryptionInfo, field, err)
	}
	return b, nil
}

func encodeB64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, fmt.Errorf("crypto: reading random bytes: %w", err)
	}
	return b, nil
}
