package crypto

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/asn1"
	"io"
	"math/big"
	"strings"
	"testing"
)

// brokenECDSASigner is a crypto.Signer whose Sign returns a DER structure with
// out-of-range r/s values. Signers are caller-supplied — hardware tokens,
// remote signing services, test doubles — so this package must not assume the
// bytes coming back are well-formed.
type brokenECDSASigner struct {
	pub  *ecdsa.PublicKey
	r, s *big.Int
}

func (b brokenECDSASigner) Public() crypto.PublicKey { return b.pub }

func (b brokenECDSASigner) Sign(io.Reader, []byte, crypto.SignerOpts) ([]byte, error) {
	return asn1.Marshal(struct{ R, S *big.Int }{b.r, b.s})
}

// TestSignRejectsOutOfRangeECDSASignature is the regression test for the panic
// path: big.Int.FillBytes panics on a negative value and on one too large for
// the destination buffer, so a misbehaving signer took down the caller's
// goroutine instead of getting an error back.
func TestSignRejectsOutOfRangeECDSASignature(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	order := key.Curve.Params().N

	cases := []struct {
		name string
		r, s *big.Int
	}{
		{"r-too-large", new(big.Int).Lsh(big.NewInt(1), 300), big.NewInt(1)},
		{"s-too-large", big.NewInt(1), new(big.Int).Add(order, big.NewInt(1))},
		{"r-negative", big.NewInt(-1), big.NewInt(1)},
		{"s-negative", big.NewInt(1), big.NewInt(-7)},
		{"r-zero", big.NewInt(0), big.NewInt(1)},
		{"s-zero", big.NewInt(1), big.NewInt(0)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			signer := brokenECDSASigner{pub: &key.PublicKey, r: c.r, s: c.s}
			sig, err := Sign(signer, AlgECDSASHA256, []byte("<SignedInfo/>"))
			if err == nil {
				t.Fatalf("Sign accepted an out-of-range ECDSA signature (%d bytes)", len(sig))
			}
			if !strings.Contains(err.Error(), "out of range") {
				t.Fatalf("error %q does not explain the range problem", err)
			}
		})
	}
}

// TestSignAcceptsWellFormedECDSASignature keeps the happy path honest: the
// range check must not reject the signatures a real key produces.
func TestSignAcceptsWellFormedECDSASignature(t *testing.T) {
	for _, curve := range []elliptic.Curve{elliptic.P256(), elliptic.P384()} {
		key, err := ecdsa.GenerateKey(curve, rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		signedInfo := []byte("<SignedInfo>canonical</SignedInfo>")
		sig, err := Sign(key, AlgECDSASHA256, signedInfo)
		if err != nil {
			t.Fatalf("%s: Sign: %v", curve.Params().Name, err)
		}
		if want := 2 * ((curve.Params().BitSize + 7) / 8); len(sig) != want {
			t.Fatalf("%s: signature is %d bytes, want %d", curve.Params().Name, len(sig), want)
		}
		if err := Verify(&key.PublicKey, AlgECDSASHA256, signedInfo, sig); err != nil {
			t.Fatalf("%s: Verify: %v", curve.Params().Name, err)
		}
	}
}
