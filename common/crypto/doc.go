// Package crypto is spine's encryption and digital-signature engine for Office
// documents: the cryptography behind OPC's encrypted-container and
// package-signature support. It uses only the Go standard library's audited
// primitives (crypto/aes, crypto/cipher, crypto/sha1, crypto/sha256,
// crypto/sha512, crypto/hmac, crypto/rsa, crypto/ecdsa, crypto/rand) and pulls
// in no external crypto modules; the sole exception is the obsolete RC4 stream
// cipher, which the standard library no longer ships and which is implemented
// here from its public specification.
//
// # Password-based encryption ([MS-OFFCRYPTO])
//
// Decrypt recovers the plaintext OOXML package from an encrypted container,
// auto-detecting the scheme from the EncryptionInfo version;
// DecryptWithOptions additionally reports which scheme it was and whether the
// bytes were authenticated:
//
//   - Agile encryption (§2.3.4.10–§2.3.4.15), the Office 2010+ default. Read and
//     write. The read path accepts AES-128/192/256 in CBC mode with
//     SHA-1/256/384/512 key derivation; Encrypt writes the modern Office
//     defaults, AES-256 with SHA-512, and it is the scheme the public save path
//     uses.
//   - ECMA-376 standard encryption (§2.3.4.5–§2.3.4.9): AES in ECB mode with
//     SHA-1 key derivation, written by Office 2007. Read and write —
//     EncryptStandard produces it.
//   - RC4 CryptoAPI (§2.3.5): the obsolete RC4 stream-cipher scheme. This
//     package can open legacy RC4-encrypted packages; the public save path
//     (opc.SaveEncrypted) deliberately does not offer RC4, because the cipher
//     and its single un-iterated SHA-1 key derivation are cryptographically
//     broken. The exported EncryptRC4CryptoAPI exists to exercise and
//     cross-validate that decrypt path against a reference implementation, and
//     is documented as not for protecting data.
//
// The extensible scheme and the version-1.1 binary-format RC4 scheme (§2.3.6,
// used by the legacy .doc/.xls/.ppt formats, which never wrap an OOXML .zip)
// are recognized and rejected with ErrUnsupportedEncryption rather than decoded.
//
// # What a successful decryption proves
//
// Only agile encryption authenticates the ciphertext: its descriptor carries a
// dataIntegrity block holding an HMAC over the EncryptedPackage stream, keyed
// from the document key, and Decrypt requires it — a descriptor without one is
// rejected with ErrIntegrityCheckFailed, because the descriptor is plaintext
// and an attacker who can tamper with the ciphertext can equally delete the
// element that asks for the check (see DecryptOptions for the explicit,
// per-call opt-out). For the standard and RC4 schemes there is no such check
// anywhere in the format: a successful decryption proves only that the password
// matched a stored verifier, not that the bytes are the ones the author
// encrypted. DecryptResult.IntegrityVerified reports which of the two you got;
// when it is false, treat the recovered package as untrusted input.
//
// The encryption entry points (Encrypt, EncryptStandard, EncryptRC4CryptoAPI)
// require a non-empty, valid-UTF-8 password of at most 255 characters — the
// passwords Office can represent — and return ErrInvalidPassword otherwise.
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
// Callers can match these sentinels with errors.Is:
//
//   - ErrWrongPassword — the supplied password did not match the document's
//     stored verifier. Prompt again; nothing is known about the document's
//     contents.
//   - ErrIntegrityCheckFailed — the password was right but the package is not
//     authenticated: its HMAC did not match, or the agile descriptor carries no
//     usable dataIntegrity block. Do not retry with another password and do not
//     use the bytes; the file has been truncated, corrupted or tampered with.
//   - ErrUnsupportedEncryption — the document uses an encryption scheme, cipher,
//     chaining mode or hash this package does not implement.
//   - ErrMalformedEncryptionInfo — the EncryptionInfo stream could not be parsed
//     as a recognized descriptor (or declares out-of-range parameters, such as a
//     spinCount far beyond what any producer writes).
//   - ErrInvalidPassword — the password handed to an encryption entry point
//     cannot protect a document: empty, not valid UTF-8, or over 255 characters.
//   - ErrUnsupportedAlgorithm — the signing side was asked for a digest,
//     signature method or key type it does not implement.
package crypto
