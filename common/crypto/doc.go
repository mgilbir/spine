// Package crypto is spine's encryption and digital-signature engine for Office
// documents: the cryptography behind OPC's encrypted-container and
// package-signature support. It uses only the Go standard library's audited
// primitives (crypto/aes, crypto/cipher, crypto/sha1, crypto/sha256,
// crypto/sha512, crypto/hmac, crypto/rsa, crypto/ecdsa, crypto/rand) and pulls
// in no external crypto modules; the sole exception is the obsolete RC4 stream
// cipher, which the standard library no longer ships and which is implemented
// here from its public specification for the read path only.
//
// # Password-based encryption ([MS-OFFCRYPTO])
//
// Decrypt recovers the plaintext OOXML package from an encrypted container,
// auto-detecting the scheme from the EncryptionInfo version:
//
//   - Agile encryption (§2.3.4.10–§2.3.4.15): AES-256 in CBC mode with SHA-512
//     key derivation, the Office 2010+ default. Read and write — Encrypt
//     produces it and it is the scheme the public save path uses.
//   - ECMA-376 standard encryption (§2.3.4.5–§2.3.4.9): AES in ECB mode with
//     SHA-1 key derivation, written by Office 2007. Read and write —
//     EncryptStandard produces it.
//   - RC4 CryptoAPI (§2.3.5): the obsolete RC4 stream-cipher scheme. Decrypt
//     only. This package can open legacy RC4-encrypted packages, but the public
//     save path deliberately does not offer RC4 because the cipher and its
//     single un-iterated SHA-1 key derivation are cryptographically broken.
//     EncryptRC4CryptoAPI exists solely to cross-validate the decrypt path in
//     tests and is intentionally not wired into opc.SaveEncrypted.
//
// The extensible scheme and the version-1.1 binary-format RC4 scheme (§2.3.6,
// used by the legacy .doc/.xls/.ppt formats, which never wrap an OOXML .zip)
// are recognized and rejected with ErrUnsupportedEncryption rather than decoded.
//
// # XML digital signatures
//
// Sign and Verify perform W3C XML Digital Signature operations (used by OPC
// package signatures, ECMA-376 Part 2 §13) over RSA (PKCS#1 v1.5) and ECDSA
// keys with SHA-1/256/384/512 digests; Digest, NewDigest, and
// SignatureMethodForKey are the supporting helpers. New signatures use SHA-256;
// SHA-1 is accepted only when verifying pre-existing signatures.
//
// # Legacy obfuscation helpers
//
// LegacyPasswordHash computes Office's legacy 16-bit password "hash"
// (ECMA-376 §18.3.1.75 / [MS-OFFCRYPTO] §2.3.7.1) used by the non-cryptographic
// worksheet-protection, workbook-structure-protection, and document-edit
// enforcement UI features. These are deliberately weak, fully documented
// obfuscation schemes, not encryption: the values they produce guard nothing —
// any tool can clear the corresponding protection element without knowing the
// password. They exist only so that files this library writes interoperate with
// the same UI guards that Word and Excel present.
//
// # Errors
//
// Callers can match these sentinels with errors.Is: ErrWrongPassword (the
// supplied password did not match the document's stored verifier) and
// ErrUnsupportedEncryption (the document uses an encryption scheme this package
// does not implement).
package crypto
