package crypto

// This file implements the ECMA-376 "standard" encryption scheme
// ([MS-OFFCRYPTO] §2.3.4.5 through §2.3.4.9), the AES-based password protection
// that Office 2007 wrote before the agile scheme (agile.go) became the default.
// Unlike agile it uses SHA-1 key derivation and AES in ECB mode with no
// per-segment IVs, and it stores its parameters in a fixed binary
// EncryptionInfo header rather than XML.
//
// As with agile.go, only the Go standard library's audited primitives are used
// (crypto/aes, crypto/sha1). ECB is not a hand-rolled cipher: it is the trivial
// block mode — each 16-byte block is transformed independently by the stdlib AES
// block cipher — which the standard scheme mandates ([MS-OFFCRYPTO] §2.3.4.5,
// "the block cipher chaining mode MUST be ECB").

import (
	"crypto/aes"
	"crypto/hmac"
	//nolint:gosec // SHA-1 is the algorithm ECMA-376 standard encryption mandates.
	"crypto/sha1"
	"encoding/binary"
	"fmt"
)

// EncryptionHeader.Flags / EncryptionInfo.Flags bits ([MS-OFFCRYPTO] §2.3.1).
const (
	stdFlagCryptoAPI = 0x04 // fCryptoAPI
	stdFlagAES       = 0x20 // fAES
)

// AlgID values for the standard EncryptionHeader ([MS-OFFCRYPTO] §2.3.4.6).
const (
	stdAlgAES128 = 0x0000660E
	stdAlgAES192 = 0x0000660F
	stdAlgAES256 = 0x00006610
	stdAlgRC4    = 0x00006801
)

const (
	stdAlgIDHashSHA1  = 0x00008004 // SHA-1
	stdProviderAES    = 0x00000018 // PROV_RSA_AES
	stdSpinCount      = 50000      // fixed iteration count for standard key derivation
	stdSaltSize       = 16
	stdVerifierSize   = 16 // random verifier plaintext length
	stdSHA1Size       = 20
	stdCSPNameDefault = "Microsoft Enhanced RSA and AES Cryptographic Provider"
)

// standardEncryptionInfo is the parsed standard EncryptionInfo (the binary
// EncryptionHeader plus EncryptionVerifier), version header already consumed.
type standardEncryptionInfo struct {
	algID       uint32
	algIDHash   uint32
	keyBits     int
	salt        []byte
	encVerifier []byte // 16 bytes
	encVerHash  []byte // 32 bytes (AES)
}

// standardDecrypt recovers the plaintext package from a standard-encryption
// container. infoAfterVersion is the EncryptionInfo stream with only its 4-byte
// version prefix removed (the Flags/HeaderSize fields follow).
func standardDecrypt(infoAfterVersion, encryptedPackage []byte, password string) ([]byte, error) {
	info, err := parseStandardEncryptionInfo(infoAfterVersion)
	if err != nil {
		return nil, err
	}
	if info.algID == stdAlgRC4 || (info.algID&0xFF00) != 0x6600 {
		return nil, fmt.Errorf("%w: standard EncryptionHeader AlgID %#x is not AES", ErrUnsupportedEncryption, info.algID)
	}
	if info.algIDHash != 0 && info.algIDHash != stdAlgIDHashSHA1 {
		return nil, fmt.Errorf("%w: standard EncryptionHeader AlgIDHash %#x is not SHA-1", ErrUnsupportedEncryption, info.algIDHash)
	}
	keyBytes := info.keyBits / 8
	switch info.keyBits {
	case 128, 192, 256:
	default:
		return nil, fmt.Errorf("%w: standard AES key size %d bits", ErrUnsupportedEncryption, info.keyBits)
	}

	key := standardDeriveKey(passwordToUTF16LE(password), info.salt, keyBytes)

	// Verify the password against the stored verifier before trusting the key.
	verifier, err := aesECBDecrypt(key, info.encVerifier)
	if err != nil {
		return nil, err
	}
	verHash, err := aesECBDecrypt(key, info.encVerHash)
	if err != nil {
		return nil, err
	}
	//nolint:gosec // matching the document's SHA-1 verifier, not a security decision.
	h := sha1.Sum(verifier)
	if !hmac.Equal(h[:], truncate(verHash, stdSHA1Size)) {
		return nil, ErrWrongPassword
	}

	return standardDecryptPackage(key, encryptedPackage)
}

// parseStandardEncryptionInfo decodes the binary EncryptionHeader and
// EncryptionVerifier ([MS-OFFCRYPTO] §2.3.4.5/§2.3.4.6/§2.3.4.7).
func parseStandardEncryptionInfo(b []byte) (standardEncryptionInfo, error) {
	var info standardEncryptionInfo
	// EncryptionInfo.Flags (4) + EncryptionHeaderSize (4).
	if len(b) < 8 {
		return info, fmt.Errorf("%w: standard EncryptionInfo is %d bytes, need at least 8", ErrMalformedEncryptionInfo, len(b))
	}
	headerSize := binary.LittleEndian.Uint32(b[4:8])
	// EncryptionHeader has eight fixed 4-byte fields (32 bytes) then the CSPName.
	if headerSize < 32 || uint64(headerSize)+8 > uint64(len(b)) {
		return info, fmt.Errorf("%w: standard EncryptionHeader size %d is out of range", ErrMalformedEncryptionInfo, headerSize)
	}
	header := b[8 : 8+headerSize]
	info.algID = binary.LittleEndian.Uint32(header[8:12])
	info.algIDHash = binary.LittleEndian.Uint32(header[12:16])
	info.keyBits = int(binary.LittleEndian.Uint32(header[16:20]))

	// EncryptionVerifier follows the header: SaltSize(4), Salt(16),
	// EncryptedVerifier(16), VerifierHashSize(4), EncryptedVerifierHash.
	v := b[8+headerSize:]
	if len(v) < 4+16+16+4 {
		return info, fmt.Errorf("%w: standard EncryptionVerifier is %d bytes, too short", ErrMalformedEncryptionInfo, len(v))
	}
	saltSize := binary.LittleEndian.Uint32(v[0:4])
	if saltSize != stdSaltSize {
		return info, fmt.Errorf("%w: standard verifier saltSize %d, expected %d", ErrMalformedEncryptionInfo, saltSize, stdSaltSize)
	}
	info.salt = append([]byte(nil), v[4:20]...)
	info.encVerifier = append([]byte(nil), v[20:36]...)
	// VerifierHashSize at v[36:40]; the encrypted hash occupies the remaining
	// bytes rounded to the AES block size (32 for AES/SHA-1).
	rest := v[40:]
	if len(rest) < 32 {
		return info, fmt.Errorf("%w: standard EncryptedVerifierHash is %d bytes, need 32", ErrMalformedEncryptionInfo, len(rest))
	}
	info.encVerHash = append([]byte(nil), rest[:32]...)
	return info, nil
}

// standardDecryptPackage ECB-decrypts the EncryptedPackage stream and truncates
// it to the plaintext length stored in the 8-byte little-endian prefix.
func standardDecryptPackage(key, encryptedPackage []byte) ([]byte, error) {
	if len(encryptedPackage) < 8 {
		return nil, fmt.Errorf("%w: EncryptedPackage is %d bytes, need at least 8", ErrMalformedEncryptionInfo, len(encryptedPackage))
	}
	totalSize := binary.LittleEndian.Uint64(encryptedPackage[:8])
	ciphertext := encryptedPackage[8:]
	if totalSize > uint64(len(ciphertext)) {
		return nil, fmt.Errorf("%w: declared plaintext size %d exceeds ciphertext length %d", ErrMalformedEncryptionInfo, totalSize, len(ciphertext))
	}
	if len(ciphertext) == 0 {
		return []byte{}, nil
	}
	plain, err := aesECBDecrypt(key, ciphertext)
	if err != nil {
		return nil, err
	}
	return plain[:totalSize], nil
}

// EncryptStandard produces the two streams of a standard-encryption container
// (EncryptionInfo with its version header, and EncryptedPackage) from a
// plaintext OOXML package. keyBits selects the AES key size (128, 192, or 256).
// It uses SHA-1 key derivation and AES-ECB, as the standard scheme requires, and
// generates a fresh random salt and verifier on every call.
//
// The standard scheme is weaker than agile (SHA-1, no per-block IV, no package
// integrity HMAC) and is offered only for compatibility with tools and older
// Office builds that expect it; new documents should prefer agile Encrypt.
func EncryptStandard(packageData []byte, password string, keyBits int) (encryptionInfo, encryptedPackage []byte, err error) {
	var algID uint32
	switch keyBits {
	case 128:
		algID = stdAlgAES128
	case 192:
		algID = stdAlgAES192
	case 256:
		algID = stdAlgAES256
	default:
		return nil, nil, fmt.Errorf("%w: standard AES key size %d bits", ErrUnsupportedEncryption, keyBits)
	}
	keyBytes := keyBits / 8

	salt, err := randomBytes(stdSaltSize)
	if err != nil {
		return nil, nil, err
	}
	verifier, err := randomBytes(stdVerifierSize)
	if err != nil {
		return nil, nil, err
	}

	key := standardDeriveKey(passwordToUTF16LE(password), salt, keyBytes)

	encVerifier, err := aesECBEncrypt(key, verifier)
	if err != nil {
		return nil, nil, err
	}
	//nolint:gosec // SHA-1 verifier is mandated by the standard scheme.
	vhash := sha1.Sum(verifier)
	encVerHash, err := aesECBEncrypt(key, pad(vhash[:], aes.BlockSize)) // 20 -> 32
	if err != nil {
		return nil, nil, err
	}

	encPkg, err := standardEncryptPackage(key, packageData)
	if err != nil {
		return nil, nil, err
	}

	encryptionInfo = buildStandardEncryptionInfo(algID, keyBits, salt, encVerifier, encVerHash)
	return encryptionInfo, encPkg, nil
}

// standardEncryptPackage prepends the 8-byte plaintext length and ECB-encrypts
// the zero-padded package.
func standardEncryptPackage(key, packageData []byte) ([]byte, error) {
	padded := pad(packageData, aes.BlockSize)
	ct, err := aesECBEncrypt(key, padded)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 8, 8+len(ct))
	binary.LittleEndian.PutUint64(out, uint64(len(packageData)))
	return append(out, ct...), nil
}

// buildStandardEncryptionInfo serializes the version header, EncryptionHeader,
// and EncryptionVerifier ([MS-OFFCRYPTO] §2.3.4.5).
func buildStandardEncryptionInfo(algID uint32, keyBits int, salt, encVerifier, encVerHash []byte) []byte {
	flags := uint32(stdFlagCryptoAPI | stdFlagAES)

	// CSPName: null-terminated UTF-16LE.
	csp := utf16LEWithNull(stdCSPNameDefault)

	// EncryptionHeader: 8 fixed 4-byte fields + CSPName.
	header := make([]byte, 32+len(csp))
	binary.LittleEndian.PutUint32(header[0:4], flags)  // Flags
	binary.LittleEndian.PutUint32(header[4:8], 0)      // SizeExtra
	binary.LittleEndian.PutUint32(header[8:12], algID) // AlgID
	binary.LittleEndian.PutUint32(header[12:16], stdAlgIDHashSHA1)
	binary.LittleEndian.PutUint32(header[16:20], uint32(keyBits)) // KeySize
	binary.LittleEndian.PutUint32(header[20:24], stdProviderAES)  // ProviderType
	// Reserved1/Reserved2 (24:32) stay zero.
	copy(header[32:], csp)

	// EncryptionVerifier.
	verifier := make([]byte, 0, 4+16+16+4+len(encVerHash))
	verifier = appendUint32(verifier, stdSaltSize)
	verifier = append(verifier, salt...)
	verifier = append(verifier, encVerifier...)
	verifier = appendUint32(verifier, stdSHA1Size) // VerifierHashSize (pre-encryption)
	verifier = append(verifier, encVerHash...)

	out := make([]byte, 0, 8+8+len(header)+len(verifier))
	// Version header: major=3, minor=2 (ECMA-376 standard AES, Office 2007 SP2).
	out = append(out, 0x03, 0x00, 0x02, 0x00)
	out = appendUint32(out, flags)               // EncryptionInfo.Flags
	out = appendUint32(out, uint32(len(header))) // EncryptionHeaderSize
	out = append(out, header...)
	out = append(out, verifier...)
	return out
}

// standardDeriveKey implements the ECMA-376 standard key derivation
// ([MS-OFFCRYPTO] §2.3.4.7): H0 = SHA1(salt‖password); iterate stdSpinCount
// times prepending the little-endian counter; Hfinal = SHA1(H‖0); then derive
// the key from the 0x36/0x5c padded X1‖X2 construction.
func standardDeriveKey(passwordUTF16, salt []byte, keyBytes int) []byte {
	//nolint:gosec // SHA-1 is mandated by ECMA-376 standard encryption.
	h := sha1.New()
	h.Write(salt)
	h.Write(passwordUTF16)
	digest := h.Sum(nil)

	var ctr [4]byte
	for i := 0; i < stdSpinCount; i++ {
		binary.LittleEndian.PutUint32(ctr[:], uint32(i))
		h.Reset()
		h.Write(ctr[:])
		h.Write(digest)
		digest = h.Sum(digest[:0])
	}

	// Hfinal = SHA1(digest ‖ block=0).
	var block [4]byte
	h.Reset()
	h.Write(digest)
	h.Write(block[:])
	hfinal := h.Sum(nil)

	x1 := standardDeriveHalf(hfinal, 0x36)
	x2 := standardDeriveHalf(hfinal, 0x5c)
	keyMaterial := append(x1, x2...)
	return keyMaterial[:keyBytes]
}

// standardDeriveHalf computes SHA1(buf) where buf is 64 bytes of the pad byte
// with the first 20 bytes XORed against hfinal ([MS-OFFCRYPTO] §2.3.4.7).
func standardDeriveHalf(hfinal []byte, padByte byte) []byte {
	buf := make([]byte, 64)
	for i := range buf {
		buf[i] = padByte
	}
	for i := 0; i < stdSHA1Size && i < len(hfinal); i++ {
		buf[i] ^= hfinal[i]
	}
	//nolint:gosec // SHA-1 is mandated by ECMA-376 standard encryption.
	sum := sha1.Sum(buf)
	return sum[:]
}

// aesECBDecrypt decrypts a block-aligned ciphertext with AES in ECB mode.
func aesECBDecrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	bs := block.BlockSize()
	if len(ciphertext) == 0 || len(ciphertext)%bs != 0 {
		return nil, fmt.Errorf("%w: ECB ciphertext length %d is not a positive multiple of the %d-byte block size", ErrMalformedEncryptionInfo, len(ciphertext), bs)
	}
	out := make([]byte, len(ciphertext))
	for i := 0; i < len(ciphertext); i += bs {
		block.Decrypt(out[i:i+bs], ciphertext[i:i+bs])
	}
	return out, nil
}

// aesECBEncrypt encrypts a block-aligned plaintext with AES in ECB mode.
func aesECBEncrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	bs := block.BlockSize()
	if len(plaintext)%bs != 0 {
		return nil, fmt.Errorf("crypto: ECB plaintext length %d is not a multiple of the %d-byte block size", len(plaintext), bs)
	}
	out := make([]byte, len(plaintext))
	for i := 0; i < len(plaintext); i += bs {
		block.Encrypt(out[i:i+bs], plaintext[i:i+bs])
	}
	return out, nil
}

func appendUint32(b []byte, v uint32) []byte {
	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], v)
	return append(b, tmp[:]...)
}

// utf16LEWithNull encodes s as UTF-16LE with a trailing null terminator.
func utf16LEWithNull(s string) []byte {
	out := passwordToUTF16LE(s)
	return append(out, 0x00, 0x00)
}
