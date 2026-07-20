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
// package bytes cannot be recovered. Only the modern "agile" scheme (AES-256,
// SHA-512) is implemented for both open and save; the older ECMA-376 standard
// scheme and legacy RC4 schemes are detected and rejected — see
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
// A wrong password returns crypto.ErrWrongPassword. An unsupported encryption
// scheme (ECMA-376 standard, legacy RC4) returns crypto.ErrUnsupportedEncryption.
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
	if password == "" {
		return errors.New("opc: SaveEncrypted requires a non-empty password")
	}
	encInfo, encPkg, err := crypto.Encrypt(packageData, password)
	if err != nil {
		return err
	}
	return writeCFB(w, []cfbStream{
		{name: cfbStreamEncryptionInfo, data: encInfo},
		{name: cfbStreamEncryptedPackage, data: encPkg},
	})
}
