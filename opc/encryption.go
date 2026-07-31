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
// OOXML document rather than a plain zip package, and that no password was
// supplied. Every open takes options, so the retry is the same call with
// WithPassword added — opc.OpenReader, opc.NewReader, and each format
// package's Open and OpenReader all accept it.
var ErrEncrypted = errors.New("opc: package is encrypted; pass opc.WithPassword to the same open to decrypt it")

// stream names inside the CFB container ([MS-OFFCRYPTO] §2.3.4.10).
const (
	cfbStreamEncryptionInfo   = "EncryptionInfo"
	cfbStreamEncryptedPackage = "EncryptedPackage"
)

// openEncrypted opens the password-encrypted OOXML package in r, which the
// open path has already recognised as a CFB container from its leading bytes.
// It decrypts the inner package with the password carried by opts and returns a
// Reader over the recovered (plain-zip) bytes, exactly as the open would have
// returned for an unencrypted input. With no password it reports ErrEncrypted,
// which is what makes the password an ordinary option rather than a separate
// entry point: one open serves both kinds of input.
//
// The scheme is auto-detected (agile, ECMA-376 standard, or legacy RC4
// CryptoAPI). A wrong password returns crypto.ErrWrongPassword. An unsupported
// scheme (e.g. version-1.1 binary-format RC4, or the extensible scheme) returns
// crypto.ErrUnsupportedEncryption. An agile package that fails — or cannot
// perform — its integrity check returns crypto.ErrIntegrityCheckFailed; only
// the agile scheme authenticates its ciphertext at all, so a package opened
// from a standard or RC4 container is unauthenticated by construction (see
// crypto.DecryptWithOptions to learn which you got).
//
// The decompression bounds in opts apply to the decrypted package, since the
// inner zip needs the same guarding as any other, and
// ReaderOptions.AllowMissingDataIntegrity is passed through to the agile
// decryptor — the only way to reach crypto.DecryptOptions from an OPC open.
func openEncrypted(r io.ReaderAt, size int64, opts ReaderOptions) (*Reader, error) {
	if opts.password == nil {
		return nil, ErrEncrypted
	}
	if size < 0 {
		// Unreachable through the detecting open, which needs the CFB
		// signature's worth of bytes before it gets here; kept so the make
		// below cannot be handed a negative length if that ever changes.
		return nil, fmt.Errorf("%w: negative size %d", ErrCorruptedPackage, size)
	}
	if max := opts.MaxEncryptedInputSize; max > 0 && size > max {
		return nil, fmt.Errorf("opc: encrypted input is %d bytes, exceeding the %d-byte limit (pass WithMaxEncryptedInputSize to allow it)", size, max)
	}

	data := make([]byte, size)
	if _, err := io.ReadFull(io.NewSectionReader(r, 0, size), data); err != nil {
		return nil, fmt.Errorf("opc: reading encrypted input: %w", err)
	}

	plaintext, err := decryptCFBPackage(data, *opts.password, opts)
	if err != nil {
		return nil, err
	}

	// Read the plaintext as a zip directly rather than back through the
	// detecting open: a package whose plaintext is itself a CFB container would
	// otherwise be decrypted again, and again, letting a crafted file drive
	// recursion as deep as it has layers. One layer of encryption is what the
	// format defines, so a second one is a corrupt package, not a nesting.
	return newReaderFromZip(bytes.NewReader(plaintext), int64(len(plaintext)), opts)
}

// decryptCFBPackage parses the CFB container in data and returns the decrypted
// inner package bytes. Only the decrypt-relevant fields of opts are consulted;
// the decompression bounds are applied later, by the Reader over the plaintext.
func decryptCFBPackage(data []byte, password string, opts ReaderOptions) ([]byte, error) {
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
	res, err := crypto.DecryptWithOptions(encInfo, encPkg, password, crypto.DecryptOptions{
		AllowMissingDataIntegrity: opts.AllowMissingDataIntegrity,
	})
	if err != nil {
		return nil, err
	}
	return res.Package, nil
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
		// The crypto entry points enforce this too; keep the check here so the
		// message names the save path, and wrap the sentinel so callers can
		// match either layer with errors.Is.
		return fmt.Errorf("opc: SaveEncrypted requires a non-empty password: %w", crypto.ErrInvalidPassword)
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
