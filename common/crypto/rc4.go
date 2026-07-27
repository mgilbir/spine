package crypto

// This file implements the legacy RC4 CryptoAPI encryption scheme
// ([MS-OFFCRYPTO] §2.3.5), the stream-cipher password protection Office wrote
// before the AES-based "standard" (standard.go) and "agile" (agile.go) schemes.
// Only decryption is meaningful for callers — it lets this library open the rare
// legacy-encrypted OOXML package — so that is the public surface; a small
// encryption helper is provided alongside it for round-trip and cross-validation
// testing of the decrypt path (see EncryptRC4CryptoAPI).
//
// SECURITY: RC4 is cryptographically broken and Office's RC4 CryptoAPI key
// derivation (a single un-iterated SHA-1 over salt‖password, §2.3.5.2) offers no
// meaningful work factor. This code exists ONLY to read obsolete documents. Do
// NOT protect new data with it; the public opc.SaveEncrypted path deliberately
// offers only the agile and standard schemes.
//
// The Go standard library omits RC4 (it was removed to discourage use) and
// external crypto modules are not permitted in this project, so the RC4 stream
// cipher itself (KSA + PRGA) is implemented here from its public specification.
// It is a ~20-line, well-known algorithm; the implementation is exercised
// against the published RFC 6229 test vectors in rc4_test.go. Everything above
// the cipher — SHA-1 key derivation, constant-time verifier comparison — uses the
// audited standard library.
//
// The older §2.3.6 "RC4 Encryption" scheme (EncryptionInfo version 1.1, MD5 key
// derivation) targets the legacy binary formats (.doc/.xls/.ppt) and never wraps
// an OOXML .zip package, so it is out of scope for the OOXML open path and is
// reported as unsupported by Decrypt rather than decoded here.

import (
	"crypto/hmac"
	//nolint:gosec // SHA-1 is the hash [MS-OFFCRYPTO] §2.3.5.2 mandates for RC4 CryptoAPI key derivation.
	"crypto/sha1"
	"encoding/binary"
	"fmt"
)

const (
	// rc4DefaultKeyBits is the RC4 CryptoAPI key size assumed when the header
	// records KeySize 0 ([MS-OFFCRYPTO] §2.3.4.5 / §2.3.5.1: 0x00000028 = 40).
	rc4DefaultKeyBits = 40
	// rc4VerifierHashSize is the length of the (unpadded) SHA-1 verifier hash;
	// RC4 is a stream cipher, so no block padding is applied.
	rc4VerifierHashSize = 20
	// rc4PackageBlockSize is the interval, in bytes, at which the EncryptedPackage
	// stream is re-keyed with an incrementing block number ([MS-OFFCRYPTO]
	// §2.3.5.2: 0x00000200 = 512).
	rc4PackageBlockSize = 512

	// stdProviderRC4 is PROV_RSA_FULL, the CryptoAPI provider type an RC4
	// CryptoAPI EncryptionHeader records ([MS-OFFCRYPTO] §2.3.5.1).
	stdProviderRC4 = 0x00000001
	// stdCSPNameRC4 is the default CSP name Office writes for RC4 CryptoAPI.
	stdCSPNameRC4 = "Microsoft Base Cryptographic Provider v1.0"
)

// rc4Cipher is a minimal RC4 stream cipher — the key-scheduling algorithm (KSA)
// and pseudo-random generation algorithm (PRGA) of Rivest's RC4, implemented
// from its public specification because the Go standard library no longer ships
// crypto/rc4 and external crypto packages are disallowed here. RC4 is symmetric:
// XORing the keystream decrypts or encrypts identically.
//
// SECURITY: broken cipher, legacy-read only. See the file header.
type rc4Cipher struct {
	s    [256]byte
	i, j uint8
}

// newRC4Cipher runs the RC4 KSA to initialize the permutation state from key.
// key must be non-empty (RC4 CryptoAPI keys are 5–16 bytes here).
func newRC4Cipher(key []byte) *rc4Cipher {
	c := &rc4Cipher{}
	for i := 0; i < 256; i++ {
		c.s[i] = byte(i)
	}
	var j uint8
	for i := 0; i < 256; i++ {
		j += c.s[i] + key[i%len(key)]
		c.s[i], c.s[j] = c.s[j], c.s[i]
	}
	return c
}

// xorKeyStream XORs the RC4 keystream (PRGA) into src, writing to dst and
// advancing the cipher state. dst must be at least len(src); dst and src may
// alias. Calling it again continues the same keystream.
func (c *rc4Cipher) xorKeyStream(dst, src []byte) {
	for k, b := range src {
		c.i++
		c.j += c.s[c.i]
		c.s[c.i], c.s[c.j] = c.s[c.j], c.s[c.i]
		dst[k] = b ^ c.s[c.s[c.i]+c.s[c.j]]
	}
}

// rc4CryptoAPIInfo is the parsed RC4 CryptoAPI EncryptionInfo (the binary
// EncryptionHeader plus EncryptionVerifier), with the 4-byte version prefix
// already consumed.
type rc4CryptoAPIInfo struct {
	keyBits     int
	salt        []byte // 16 bytes
	encVerifier []byte // 16 bytes
	encVerHash  []byte // 20 bytes (SHA-1, unpadded)
}

// rc4CryptoAPIDecrypt recovers the plaintext package from an RC4 CryptoAPI
// container. infoAfterVersion is the EncryptionInfo stream with only its 4-byte
// version prefix removed (the Flags/HeaderSize fields follow), matching the shape
// standardDecrypt receives. A wrong password returns ErrWrongPassword.
func rc4CryptoAPIDecrypt(infoAfterVersion, encryptedPackage []byte, password string) ([]byte, error) {
	info, err := parseRC4CryptoAPIInfo(infoAfterVersion)
	if err != nil {
		return nil, err
	}
	pw := passwordToUTF16LE(password)

	// Verify the password before trusting the key: RC4 with the block-0 key runs
	// as one continuous keystream over EncryptedVerifier then EncryptedVerifierHash
	// ([MS-OFFCRYPTO] §2.3.5.6); SHA-1 of the recovered verifier must equal the
	// recovered hash.
	c := newRC4Cipher(rc4CryptoAPIDeriveKey(pw, info.salt, info.keyBits, 0))
	verifier := make([]byte, len(info.encVerifier))
	c.xorKeyStream(verifier, info.encVerifier)
	verHash := make([]byte, len(info.encVerHash))
	c.xorKeyStream(verHash, info.encVerHash)
	//nolint:gosec // matching the document's SHA-1 verifier, not a security decision.
	h := sha1.Sum(verifier)
	if !hmac.Equal(h[:], verHash) {
		return nil, ErrWrongPassword
	}

	return rc4CryptoAPIDecryptPackage(pw, info.salt, info.keyBits, encryptedPackage)
}

// parseRC4CryptoAPIInfo decodes the binary EncryptionHeader and EncryptionVerifier
// ([MS-OFFCRYPTO] §2.3.5.1). The layout matches the standard scheme except the
// EncryptedVerifierHash is an unpadded 20-byte SHA-1 digest (RC4 is a stream
// cipher) rather than a 32-byte AES-padded block.
func parseRC4CryptoAPIInfo(b []byte) (rc4CryptoAPIInfo, error) {
	var info rc4CryptoAPIInfo
	if len(b) < 8 {
		return info, fmt.Errorf("%w: RC4 CryptoAPI EncryptionInfo is %d bytes, need at least 8", ErrMalformedEncryptionInfo, len(b))
	}
	headerSize := binary.LittleEndian.Uint32(b[4:8])
	if headerSize < 32 || uint64(headerSize)+8 > uint64(len(b)) {
		return info, fmt.Errorf("%w: RC4 CryptoAPI EncryptionHeader size %d is out of range", ErrMalformedEncryptionInfo, headerSize)
	}
	header := b[8 : 8+headerSize]
	algID := binary.LittleEndian.Uint32(header[8:12])
	if algID != 0 && algID != stdAlgRC4 {
		return info, fmt.Errorf("%w: RC4 CryptoAPI header AlgID %#x is not RC4", ErrMalformedEncryptionInfo, algID)
	}
	keyBits := int(binary.LittleEndian.Uint32(header[16:20]))
	if keyBits == 0 {
		keyBits = rc4DefaultKeyBits
	}
	if keyBits < 40 || keyBits > 128 || keyBits%8 != 0 {
		return info, fmt.Errorf("%w: RC4 CryptoAPI key size %d bits (want 40–128, multiple of 8)", ErrUnsupportedEncryption, keyBits)
	}
	info.keyBits = keyBits

	v := b[8+headerSize:]
	if len(v) < 4+16+16+4+rc4VerifierHashSize {
		return info, fmt.Errorf("%w: RC4 CryptoAPI EncryptionVerifier is %d bytes, too short", ErrMalformedEncryptionInfo, len(v))
	}
	saltSize := binary.LittleEndian.Uint32(v[0:4])
	if saltSize != stdSaltSize {
		return info, fmt.Errorf("%w: RC4 CryptoAPI verifier saltSize %d, expected %d", ErrMalformedEncryptionInfo, saltSize, stdSaltSize)
	}
	info.salt = append([]byte(nil), v[4:20]...)
	info.encVerifier = append([]byte(nil), v[20:36]...)
	// v[36:40] is VerifierHashSize; the encrypted hash is the following 20 bytes.
	info.encVerHash = append([]byte(nil), v[40:40+rc4VerifierHashSize]...)
	return info, nil
}

// rc4CryptoAPIDecryptPackage RC4-decrypts the EncryptedPackage stream and
// truncates it to the plaintext length stored in the 8-byte little-endian prefix.
// The prefix is not encrypted; the ciphertext that follows is re-keyed every
// rc4PackageBlockSize bytes with an incrementing block number.
func rc4CryptoAPIDecryptPackage(passwordUTF16, salt []byte, keyBits int, encryptedPackage []byte) ([]byte, error) {
	if len(encryptedPackage) < 8 {
		return nil, fmt.Errorf("%w: EncryptedPackage is %d bytes, need at least 8", ErrMalformedEncryptionInfo, len(encryptedPackage))
	}
	totalSize := binary.LittleEndian.Uint64(encryptedPackage[:8])
	ciphertext := encryptedPackage[8:]
	if totalSize > uint64(len(ciphertext)) {
		return nil, fmt.Errorf("%w: declared plaintext size %d exceeds ciphertext length %d", ErrMalformedEncryptionInfo, totalSize, len(ciphertext))
	}
	out := make([]byte, len(ciphertext))
	rc4CryptoAPIXORPackage(passwordUTF16, salt, keyBits, out, ciphertext)
	return out[:totalSize], nil
}

// rc4CryptoAPIXORPackage applies the RC4 CryptoAPI keystream to src (writing to
// dst), re-keying at every rc4PackageBlockSize boundary with the next block
// number. Because RC4 is symmetric this both decrypts and encrypts the package
// body. dst must be at least len(src).
func rc4CryptoAPIXORPackage(passwordUTF16, salt []byte, keyBits int, dst, src []byte) {
	for block, off := 0, 0; off < len(src); block, off = block+1, off+rc4PackageBlockSize {
		end := off + rc4PackageBlockSize
		if end > len(src) {
			end = len(src)
		}
		key := rc4CryptoAPIDeriveKey(passwordUTF16, salt, keyBits, block)
		newRC4Cipher(key).xorKeyStream(dst[off:end], src[off:end])
	}
}

// rc4CryptoAPIDeriveKey derives the RC4 key for a given block ([MS-OFFCRYPTO]
// §2.3.5.2): H0 = SHA1(salt‖password); Hfinal = SHA1(H0‖LE32(block)). A 40-bit
// key is Hfinal's first 5 bytes zero-extended to 16 bytes; a larger key is the
// first keyBits/8 bytes of Hfinal.
func rc4CryptoAPIDeriveKey(passwordUTF16, salt []byte, keyBits, block int) []byte {
	//nolint:gosec // SHA-1 is mandated by RC4 CryptoAPI key derivation.
	h := sha1.New()
	h.Write(salt)
	h.Write(passwordUTF16)
	h0 := h.Sum(nil)

	var blk [4]byte
	binary.LittleEndian.PutUint32(blk[:], uint32(block))
	h.Reset()
	h.Write(h0)
	h.Write(blk[:])
	hfinal := h.Sum(nil)

	if keyBits == 40 {
		key := make([]byte, 16)
		copy(key, hfinal[:5])
		return key
	}
	return hfinal[:keyBits/8]
}

// isRC4CryptoAPI reports whether a version-2/3/4, minor-2 EncryptionInfo stream
// (with the 4-byte version prefix already removed) describes RC4 CryptoAPI rather
// than the AES-based standard scheme, distinguished by the EncryptionHeader
// AlgID. Standard AES uses AlgIDs in the 0x6600 family; RC4 uses 0x6801.
//
// AlgID 0 means "determined by Flags" ([MS-OFFCRYPTO] §2.3.4.5), so it is *not*
// on its own an RC4 marker: the fAES bit decides, and a conformant AES file may
// well carry AlgID 0. Reading the AlgID alone routed such a file into the RC4
// path, where its verifier could never match and the caller was told the
// password was wrong — the one error that invites retrying passwords forever on
// a file that decrypts fine.
func isRC4CryptoAPI(infoAfterVersion []byte) bool {
	if len(infoAfterVersion) < 20 {
		return false
	}
	headerSize := binary.LittleEndian.Uint32(infoAfterVersion[4:8])
	if headerSize < 12 || uint64(headerSize)+8 > uint64(len(infoAfterVersion)) {
		return false
	}
	algID := binary.LittleEndian.Uint32(infoAfterVersion[16:20])
	switch algID {
	case stdAlgRC4:
		return true
	case 0:
		// EncryptionInfo.Flags at [0:4] and its EncryptionHeader.Flags copy at
		// [8:12] must agree (§2.3.4.4); accept fAES from either.
		flags := binary.LittleEndian.Uint32(infoAfterVersion[0:4]) | binary.LittleEndian.Uint32(infoAfterVersion[8:12])
		return flags&stdFlagAES == 0
	default:
		return false
	}
}

// EncryptRC4CryptoAPI produces the two streams of an RC4 CryptoAPI container
// (EncryptionInfo with its version header, and EncryptedPackage) from a plaintext
// OOXML package. keyBits selects the RC4 key size (40–128, a multiple of 8; 40 is
// the historical default). A fresh random salt and verifier are generated on
// every call.
//
// SECURITY: RC4 CryptoAPI is a broken, obsolete scheme (see the file header).
// This function exists to exercise and cross-validate the decrypt path against a
// reference implementation; it is intentionally NOT wired into opc.SaveEncrypted.
// Never use it to protect data — encrypt new documents with the agile scheme.
//
// The password must be a non-empty, valid-UTF-8 string of at most 255
// characters; see ErrInvalidPassword.
func EncryptRC4CryptoAPI(packageData []byte, password string, keyBits int) (encryptionInfo, encryptedPackage []byte, err error) {
	if err := validatePassword(password); err != nil {
		return nil, nil, err
	}
	if keyBits == 0 {
		keyBits = rc4DefaultKeyBits
	}
	if keyBits < 40 || keyBits > 128 || keyBits%8 != 0 {
		return nil, nil, fmt.Errorf("%w: RC4 CryptoAPI key size %d bits (want 40–128, multiple of 8)", ErrUnsupportedEncryption, keyBits)
	}

	salt, err := randomBytes(stdSaltSize)
	if err != nil {
		return nil, nil, err
	}
	verifier, err := randomBytes(stdVerifierSize)
	if err != nil {
		return nil, nil, err
	}
	pw := passwordToUTF16LE(password)

	// Encrypt the verifier and its SHA-1 hash with the block-0 keystream,
	// continuously (mirroring the decrypt-side verification).
	//nolint:gosec // SHA-1 verifier is mandated by the RC4 CryptoAPI scheme.
	vhash := sha1.Sum(verifier)
	c := newRC4Cipher(rc4CryptoAPIDeriveKey(pw, salt, keyBits, 0))
	encVerifier := make([]byte, len(verifier))
	c.xorKeyStream(encVerifier, verifier)
	encVerHash := make([]byte, rc4VerifierHashSize)
	c.xorKeyStream(encVerHash, vhash[:])

	encPkg := make([]byte, 8+len(packageData))
	binary.LittleEndian.PutUint64(encPkg[:8], uint64(len(packageData)))
	rc4CryptoAPIXORPackage(pw, salt, keyBits, encPkg[8:], packageData)

	encryptionInfo = buildRC4CryptoAPIEncryptionInfo(keyBits, salt, encVerifier, encVerHash)
	return encryptionInfo, encPkg, nil
}

// buildRC4CryptoAPIEncryptionInfo serializes the version header, EncryptionHeader,
// and EncryptionVerifier for an RC4 CryptoAPI container ([MS-OFFCRYPTO] §2.3.5.1).
func buildRC4CryptoAPIEncryptionInfo(keyBits int, salt, encVerifier, encVerHash []byte) []byte {
	flags := uint32(stdFlagCryptoAPI) // fCryptoAPI set, fAES clear (RC4).

	// CSPName: null-terminated UTF-16LE.
	csp := utf16LEWithNull(stdCSPNameRC4)

	// EncryptionHeader: 8 fixed 4-byte fields + CSPName.
	header := make([]byte, 32+len(csp))
	binary.LittleEndian.PutUint32(header[0:4], flags)      // Flags
	binary.LittleEndian.PutUint32(header[4:8], 0)          // SizeExtra
	binary.LittleEndian.PutUint32(header[8:12], stdAlgRC4) // AlgID (RC4)
	binary.LittleEndian.PutUint32(header[12:16], stdAlgIDHashSHA1)
	binary.LittleEndian.PutUint32(header[16:20], uint32(keyBits)) // KeySize
	binary.LittleEndian.PutUint32(header[20:24], stdProviderRC4)  // ProviderType
	// Reserved1/Reserved2 (24:32) stay zero.
	copy(header[32:], csp)

	// EncryptionVerifier.
	verifier := make([]byte, 0, 4+16+16+4+len(encVerHash))
	verifier = appendUint32(verifier, stdSaltSize)
	verifier = append(verifier, salt...)
	verifier = append(verifier, encVerifier...)
	verifier = appendUint32(verifier, rc4VerifierHashSize) // VerifierHashSize (pre-encryption)
	verifier = append(verifier, encVerHash...)

	out := make([]byte, 0, 8+8+len(header)+len(verifier))
	// Version header: major=3, minor=2 (CryptoAPI, Office 2003+ compatible).
	out = append(out, 0x03, 0x00, 0x02, 0x00)
	out = appendUint32(out, flags)               // EncryptionInfo.Flags
	out = appendUint32(out, uint32(len(header))) // EncryptionHeaderSize
	out = append(out, header...)
	out = append(out, verifier...)
	return out
}
