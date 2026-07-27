// This file adds real, standards-based cryptographic primitives for W3C
// XML Digital Signatures (used by OPC package signatures, ECMA-376 Part 2
// §13). Unlike LegacyPasswordHash in legacy.go — a deliberately weak
// obfuscation that guards nothing — the functions here perform genuine RSA
// (PKCS#1 v1.5) and ECDSA signing and verification over SHA-2 digests, using
// only the Go standard library.

package crypto

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/asn1"
	"errors"
	"fmt"
	"hash"
	"math/big"

	// sha1 is imported for verification interoperability only: legacy Office
	// signatures use RSA-SHA1. New signatures this package produces use SHA-256.
	//nolint:gosec // SHA-1 is accepted for verifying pre-existing signatures, never emitted.
	"crypto/sha1"
)

// XML-DSig / XML-Enc algorithm identifier URIs.
const (
	// Canonicalization: inclusive Canonical XML 1.0 (without comments).
	AlgC14N = "http://www.w3.org/TR/2001/REC-xml-c14n-20010315"

	// Digest methods.
	AlgSHA1   = "http://www.w3.org/2000/09/xmldsig#sha1"
	AlgSHA256 = "http://www.w3.org/2001/04/xmlenc#sha256"
	AlgSHA384 = "http://www.w3.org/2001/04/xmldsig-more#sha384"
	AlgSHA512 = "http://www.w3.org/2001/04/xmlenc#sha512"

	// Signature methods.
	AlgRSASHA1     = "http://www.w3.org/2000/09/xmldsig#rsa-sha1"
	AlgRSASHA256   = "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"
	AlgRSASHA384   = "http://www.w3.org/2001/04/xmldsig-more#rsa-sha384"
	AlgRSASHA512   = "http://www.w3.org/2001/04/xmldsig-more#rsa-sha512"
	AlgECDSASHA1   = "http://www.w3.org/2001/04/xmldsig-more#ecdsa-sha1"
	AlgECDSASHA256 = "http://www.w3.org/2001/04/xmldsig-more#ecdsa-sha256"
	AlgECDSASHA384 = "http://www.w3.org/2001/04/xmldsig-more#ecdsa-sha384"
	AlgECDSASHA512 = "http://www.w3.org/2001/04/xmldsig-more#ecdsa-sha512"
)

// ErrUnsupportedAlgorithm is returned for an algorithm URI this package does
// not implement.
var ErrUnsupportedAlgorithm = errors.New("crypto: unsupported XML-DSig algorithm")

// keyKind distinguishes the public-key families this package signs with.
type keyKind int

const (
	keyRSA keyKind = iota
	keyECDSA
)

// NewDigest returns a fresh hash for the given XML-DSig DigestMethod URI. The
// boolean reports whether the URI is recognized.
func NewDigest(digestURI string) (hash.Hash, bool) {
	switch digestURI {
	case AlgSHA1:
		//nolint:gosec // verification-only, see import note.
		return sha1.New(), true
	case AlgSHA256:
		return sha256.New(), true
	case AlgSHA384:
		return sha512.New384(), true
	case AlgSHA512:
		return sha512.New(), true
	default:
		return nil, false
	}
}

// Digest computes the digest of data under the given DigestMethod URI.
func Digest(digestURI string, data []byte) ([]byte, error) {
	h, ok := NewDigest(digestURI)
	if !ok {
		return nil, fmt.Errorf("%w: digest %q", ErrUnsupportedAlgorithm, digestURI)
	}
	h.Write(data)
	return h.Sum(nil), nil
}

// signatureMethod resolves a SignatureMethod URI to its hash and key family.
func signatureMethod(sigURI string) (crypto.Hash, keyKind, error) {
	switch sigURI {
	case AlgRSASHA1:
		return crypto.SHA1, keyRSA, nil
	case AlgRSASHA256:
		return crypto.SHA256, keyRSA, nil
	case AlgRSASHA384:
		return crypto.SHA384, keyRSA, nil
	case AlgRSASHA512:
		return crypto.SHA512, keyRSA, nil
	case AlgECDSASHA1:
		return crypto.SHA1, keyECDSA, nil
	case AlgECDSASHA256:
		return crypto.SHA256, keyECDSA, nil
	case AlgECDSASHA384:
		return crypto.SHA384, keyECDSA, nil
	case AlgECDSASHA512:
		return crypto.SHA512, keyECDSA, nil
	default:
		return 0, 0, fmt.Errorf("%w: signature %q", ErrUnsupportedAlgorithm, sigURI)
	}
}

// SignatureMethodForKey returns the SHA-256 SignatureMethod URI appropriate for
// the signer's public key, and the matching SHA-256 DigestMethod URI. New
// signatures always use SHA-256; SHA-1 is only ever accepted on verify.
func SignatureMethodForKey(pub crypto.PublicKey) (sigURI, digestURI string, err error) {
	switch pub.(type) {
	case *rsa.PublicKey:
		return AlgRSASHA256, AlgSHA256, nil
	case *ecdsa.PublicKey:
		return AlgECDSASHA256, AlgSHA256, nil
	default:
		return "", "", fmt.Errorf("%w: unsupported key type %T", ErrUnsupportedAlgorithm, pub)
	}
}

// Sign computes an XML-DSig SignatureValue over signedInfo (already
// canonicalized) using signer and the given SignatureMethod URI. For RSA the
// result is a PKCS#1 v1.5 signature; for ECDSA it is the IEEE P1363 fixed-width
// r‖s encoding XML-DSig mandates (not ASN.1 DER).
func Sign(signer crypto.Signer, sigURI string, signedInfo []byte) ([]byte, error) {
	h, kind, err := signatureMethod(sigURI)
	if err != nil {
		return nil, err
	}
	if !h.Available() {
		return nil, fmt.Errorf("%w: hash for %q unavailable", ErrUnsupportedAlgorithm, sigURI)
	}
	hh := h.New()
	hh.Write(signedInfo)
	digest := hh.Sum(nil)

	switch kind {
	case keyRSA:
		if _, ok := signer.Public().(*rsa.PublicKey); !ok {
			return nil, fmt.Errorf("crypto: signature method %q requires an RSA key, got %T", sigURI, signer.Public())
		}
		return signer.Sign(rand.Reader, digest, h)
	case keyECDSA:
		pub, ok := signer.Public().(*ecdsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("crypto: signature method %q requires an ECDSA key, got %T", sigURI, signer.Public())
		}
		der, err := signer.Sign(rand.Reader, digest, h)
		if err != nil {
			return nil, err
		}
		return ecdsaDERToP1363(der, pub.Curve)
	default:
		return nil, ErrUnsupportedAlgorithm
	}
}

// Verify checks an XML-DSig SignatureValue over signedInfo (already
// canonicalized) against pub, using the given SignatureMethod URI. It returns
// nil when the signature is valid.
func Verify(pub crypto.PublicKey, sigURI string, signedInfo, signature []byte) error {
	h, kind, err := signatureMethod(sigURI)
	if err != nil {
		return err
	}
	if !h.Available() {
		return fmt.Errorf("%w: hash for %q unavailable", ErrUnsupportedAlgorithm, sigURI)
	}
	hh := h.New()
	hh.Write(signedInfo)
	digest := hh.Sum(nil)

	switch kind {
	case keyRSA:
		rk, ok := pub.(*rsa.PublicKey)
		if !ok {
			return fmt.Errorf("crypto: signature method %q requires an RSA key, got %T", sigURI, pub)
		}
		return rsa.VerifyPKCS1v15(rk, h, digest, signature)
	case keyECDSA:
		ek, ok := pub.(*ecdsa.PublicKey)
		if !ok {
			return fmt.Errorf("crypto: signature method %q requires an ECDSA key, got %T", sigURI, pub)
		}
		r, s, err := ecdsaP1363Split(signature, ek.Curve)
		if err != nil {
			return err
		}
		if !ecdsa.Verify(ek, digest, r, s) {
			return errors.New("crypto: ECDSA signature verification failed")
		}
		return nil
	default:
		return ErrUnsupportedAlgorithm
	}
}

// ecdsaDERToP1363 converts an ASN.1 DER ECDSA signature (as crypto.Signer
// produces) into the fixed-width r‖s encoding XML-DSig uses.
//
// r and s are range-checked against the curve order before conversion: asn1
// happily unmarshals a negative or oversized INTEGER, and big.Int.FillBytes
// panics on both. A crypto.Signer is caller-supplied — it may be a hardware
// token, a remote service, or a stub — so a malformed signature has to come
// back as an error, not as a panic in the caller's goroutine.
func ecdsaDERToP1363(der []byte, curve elliptic.Curve) ([]byte, error) {
	var parsed struct{ R, S *big.Int }
	if _, err := asn1.Unmarshal(der, &parsed); err != nil {
		return nil, fmt.Errorf("crypto: parsing ECDSA DER signature: %w", err)
	}
	if parsed.R == nil || parsed.S == nil {
		return nil, errors.New("crypto: ECDSA DER signature is missing r or s")
	}
	order := curve.Params().N
	if parsed.R.Sign() <= 0 || parsed.R.Cmp(order) >= 0 {
		return nil, fmt.Errorf("crypto: ECDSA signature r is out of range for curve %s", curve.Params().Name)
	}
	if parsed.S.Sign() <= 0 || parsed.S.Cmp(order) >= 0 {
		return nil, fmt.Errorf("crypto: ECDSA signature s is out of range for curve %s", curve.Params().Name)
	}
	n := (curve.Params().BitSize + 7) / 8
	out := make([]byte, 2*n)
	parsed.R.FillBytes(out[:n])
	parsed.S.FillBytes(out[n:])
	return out, nil
}

// ecdsaP1363Split splits a fixed-width r‖s XML-DSig ECDSA signature into its
// integer components.
func ecdsaP1363Split(sig []byte, curve elliptic.Curve) (r, s *big.Int, err error) {
	n := (curve.Params().BitSize + 7) / 8
	if len(sig) != 2*n {
		return nil, nil, fmt.Errorf("crypto: ECDSA signature length %d, want %d", len(sig), 2*n)
	}
	r = new(big.Int).SetBytes(sig[:n])
	s = new(big.Int).SetBytes(sig[n:])
	return r, s, nil
}
