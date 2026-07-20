package opc

// Password encryption for OOXML packages. A password-encrypted Office document
// is not a zip: it is a Compound File Binary (CFB) container (see cfb.go)
// holding two streams — EncryptionInfo, describing the scheme, and
// EncryptedPackage, the AES-encrypted inner .zip. This file bridges that
// container to the ordinary opc.Reader/Writer flow using the agile-encryption
// primitives in common/crypto.
//
// Unlike the legacy password *obfuscation* in common/crypto/legacy.go (which
// guards nothing), this is Office's real encryption: without the password the
// package bytes cannot be recovered. The modern "agile" scheme (AES-256,
// SHA-512) and the older ECMA-376 "standard" scheme (AES) are implemented for
// both open and save. Legacy RC4 CryptoAPI ([MS-OFFCRYPTO] §2.3.5) can be opened
// (decrypted) but not saved — it is obsolete and cryptographically broken, so
// SaveEncrypted never writes it. The version-1.1 binary-format RC4 scheme
// (§2.3.6) and the extensible scheme are detected and rejected — see
// crypto.ErrUnsupportedEncryption.

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/mgilbir/spine/common/crypto"
)

// ErrEncrypted indicates that the input is a password-encrypted (CFB-wrapped)
// OOXML document rather than a plain zip package. The normal open path returns
// it so callers can retry through OpenEncrypted with a password.
var ErrEncrypted = errors.New("opc: package is encrypted; open it with OpenEncrypted and a password")

// MaxEncryptedInputSize bounds how many bytes OpenEncrypted will read from its
// source before attempting to parse the CFB container, guarding against a
// hostile size argument that would otherwise drive an unbounded allocation.
// Raise it before opening a legitimately larger encrypted document. It is a
// plain package-level variable; mutate it during setup, not concurrently with
// OpenEncrypted.
var MaxEncryptedInputSize int64 = 2 << 30 // 2 GiB

// stream names inside the CFB container ([MS-OFFCRYPTO] §2.3.4.10).
const (
	cfbStreamEncryptionInfo   = "EncryptionInfo"
	cfbStreamEncryptedPackage = "EncryptedPackage"
)

// OpenEncrypted opens a password-encrypted OOXML package. It reads the CFB
// container from r, decrypts the inner package with the supplied password, and
// returns a Reader over the recovered (plain-zip) package bytes exactly as
// OpenReader would for an unencrypted file.
//
// It auto-detects the scheme (agile, ECMA-376 standard, or legacy RC4 CryptoAPI).
// A wrong password returns crypto.ErrWrongPassword. An unsupported encryption
// scheme (e.g. version-1.1 binary-format RC4, or the extensible scheme) returns
// crypto.ErrUnsupportedEncryption.
func OpenEncrypted(r io.ReaderAt, size int64, password string) (*Reader, error) {
	return OpenEncryptedWithOptions(r, size, password, ReaderOptions{})
}

// OpenEncryptedWithOptions is OpenEncrypted with per-Reader decompression
// options applied to the decrypted package (the same limits guard the inner
// zip against decompression bombs).
func OpenEncryptedWithOptions(r io.ReaderAt, size int64, password string, opts ReaderOptions) (*Reader, error) {
	if size < 0 {
		return nil, fmt.Errorf("%w: negative size %d", ErrCorruptedPackage, size)
	}
	if MaxEncryptedInputSize > 0 && size > MaxEncryptedInputSize {
		return nil, fmt.Errorf("opc: encrypted input is %d bytes, exceeding the %d-byte limit (raise MaxEncryptedInputSize before opening)", size, MaxEncryptedInputSize)
	}

	data := make([]byte, size)
	if _, err := io.ReadFull(io.NewSectionReader(r, 0, size), data); err != nil {
		return nil, fmt.Errorf("opc: reading encrypted input: %w", err)
	}

	plaintext, err := decryptCFBPackage(data, password)
	if err != nil {
		return nil, err
	}
	return NewReaderWithOptions(bytes.NewReader(plaintext), int64(len(plaintext)), opts)
}

// decryptCFBPackage parses the CFB container in data and returns the decrypted
// inner package bytes.
func decryptCFBPackage(data []byte, password string) ([]byte, error) {
	cf, err := readCFB(data)
	if err != nil {
		return nil, err
	}
	encInfo, err := cf.stream(cfbStreamEncryptionInfo)
	if err != nil {
		return nil, err
	}
	encPkg, err := cf.stream(cfbStreamEncryptedPackage)
	if err != nil {
		return nil, err
	}
	return crypto.Decrypt(encInfo, encPkg, password)
}

// SaveEncrypted encrypts a plain OOXML package (the zip bytes produced by a
// normal save) with the supplied password and writes the resulting CFB
// container to w. It uses agile encryption (AES-256, SHA-512) with a freshly
// generated random salt, so the same package encrypted twice yields different
// ciphertext.
//
// packageData must be the complete bytes of an unencrypted .zip package (for
// example docx.Document.SaveBytes()). The password must not be empty.
func SaveEncrypted(w io.Writer, packageData []byte, password string) error {
	return SaveEncryptedWithOptions(w, packageData, password, EncryptOptions{})
}

// EncryptionScheme selects the password-encryption algorithm SaveEncryptedWithOptions
// writes.
type EncryptionScheme int

const (
	// SchemeAgile is the modern Office 2010+ scheme (AES-256 CBC, SHA-512 key
	// derivation, per-segment IVs, package HMAC integrity). It is the default and
	// the recommended choice.
	SchemeAgile EncryptionScheme = iota

	// SchemeStandard is the older ECMA-376 "standard" scheme (AES ECB, SHA-1 key
	// derivation; [MS-OFFCRYPTO] §2.3.4.5). It is weaker than agile — no
	// per-block IV and no integrity HMAC — and is offered only for compatibility
	// with tools or older Office builds that expect it.
	SchemeStandard
)

// EncryptOptions configures SaveEncryptedWithOptions. The zero value selects the
// agile scheme without DataSpaces streams, matching SaveEncrypted.
type EncryptOptions struct {
	// Scheme selects the encryption algorithm. The zero value is SchemeAgile.
	Scheme EncryptionScheme

	// StandardKeyBits is the AES key size for SchemeStandard: 128, 192, or 256.
	// Zero means 256. It is ignored for SchemeAgile (which is always AES-256).
	StandardKeyBits int

	// IncludeDataSpaces emits the optional \x06DataSpaces metadata streams
	// ([MS-OFFCRYPTO] §2.1) into the container. They are not required to decrypt
	// the document, but some Office builds expect them; they are omitted by
	// default so the output stays minimal.
	IncludeDataSpaces bool
}

// SaveEncryptedWithOptions is SaveEncrypted with control over the encryption
// scheme, AES key size, and whether the optional DataSpaces metadata streams are
// emitted. The password must not be empty.
func SaveEncryptedWithOptions(w io.Writer, packageData []byte, password string, opts EncryptOptions) error {
	if password == "" {
		return errors.New("opc: SaveEncrypted requires a non-empty password")
	}
	var (
		encInfo, encPkg []byte
		err             error
	)
	switch opts.Scheme {
	case SchemeAgile:
		encInfo, encPkg, err = crypto.Encrypt(packageData, password)
	case SchemeStandard:
		keyBits := opts.StandardKeyBits
		if keyBits == 0 {
			keyBits = 256
		}
		encInfo, encPkg, err = crypto.EncryptStandard(packageData, password, keyBits)
	default:
		return fmt.Errorf("opc: unknown encryption scheme %d", opts.Scheme)
	}
	if err != nil {
		return err
	}

	streams := []cfbStream{
		{name: cfbStreamEncryptionInfo, data: encInfo},
		{name: cfbStreamEncryptedPackage, data: encPkg},
	}
	var storages []cfbStorage
	if opts.IncludeDataSpaces {
		storages = dataSpacesStorages()
	}
	return writeCFBWithStorages(w, streams, storages)
}
