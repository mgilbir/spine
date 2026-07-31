package opc

// Fuzz target for XML digital signature verification.
//
// The signature part is the one part of a signed package an attacker can
// rewrite freely: everything outside SignedInfo is unprotected once the package
// has left the signer, and SignedInfo itself can be edited too — doing so just
// invalidates SignatureValue. C362 lived exactly there: manifest references were
// counted from every <Object>, not only from the Objects a verified SignedInfo
// reference covered, so appending an Object made CoveredParts (and Valid) claim
// coverage the certificate holder never granted.
//
// The oracle is therefore a coverage-containment property rather than "no
// panic": whatever bytes the fuzzer puts in the signature part, the reported
// coverage may never exceed what the genuine signature covers. A fuzzer holds no
// private key, so the only way past both gates — SignatureValue verifying over
// canonical SignedInfo, and an Object reference digest matching — is to leave
// both byte-identical, and a byte-identical Object carries the original
// manifest.

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	spinecrypto "github.com/mgilbir/spine/common/crypto"
	"github.com/mgilbir/spine/internal/fuzzbound"
	"github.com/mgilbir/spine/internal/fuzzseed"
)

// signatureBudget bounds one verification pass over a fuzzed signature part.
//
// It is by some way the loosest of these budgets, and deliberately so. This
// stage builds two in-memory trees from the XML — the unmarshalled model and
// the canonicalization tree — and a tree node costs far more than the text it
// came from: measured at 31x for a genuine signature, 39x for a large one (300
// signed parts), and up to 247x for an input that is nothing but empty elements
// (`<x/>`, four bytes of text per node). A rate below that would fail on inputs
// that are merely unusual rather than wrong. 512x with a 4 MiB floor is about
// twice the worst structural ratio measured.
//
// What survives at that rate is the class of bug worth catching: an allocation
// sized from a declared length is unbounded in the input, not a constant factor
// of it, so it lands orders of magnitude past any of these numbers. The time
// bound complements it — a pathological tree that is merely large stays in the
// tens of milliseconds (the 247x case above took 20ms).
var signatureBudget = fuzzbound.Budget{
	What:              "VerifySignatures",
	Bytes:             4 << 20,
	BytesPerInputByte: 512,
	Time:              15 * time.Second,
	TimePerMiB:        2 * time.Second,
}

// sigFixture is a validly signed package plus the coverage its genuine
// signature grants, which is the containment bound the fuzz body asserts.
type sigFixture struct {
	signed  []byte // the signed package
	sigXML  []byte // its signature part
	covered map[string]bool
}

// fuzzTestCert builds a self-signed certificate for signer's public key
// (testCert with a testing.TB receiver, so a fuzz target can call it).
func fuzzTestCert(tb testing.TB, signer crypto.Signer) *x509.Certificate {
	tb.Helper()
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Spine Fuzz Signer"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, signer.Public(), signer)
	if err != nil {
		tb.Fatalf("CreateCertificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		tb.Fatalf("ParseCertificate: %v", err)
	}
	return cert
}

// buildSigFixture signs a small multi-part package over one part only, so the
// fixture has both a covered part and an uncovered one to forge coverage for.
func buildSigFixture(tb testing.TB) sigFixture {
	tb.Helper()
	var pkgBuf bytes.Buffer
	w := NewWriter(&pkgBuf)
	if err := w.WritePart(fuzzSignedPart, ContentTypeDocument, []byte("<document>signed</document>")); err != nil {
		tb.Fatalf("WritePart: %v", err)
	}
	if err := w.WritePart(fuzzUnsignedPart, "", []byte("\x89PNG\r\n\x1a\nnot-signed")); err != nil {
		tb.Fatalf("WritePart: %v", err)
	}
	if _, err := w.AddRelationship(RelTypeOfficeDocument, "word/document.xml", TargetModeInternal); err != nil {
		tb.Fatalf("AddRelationship: %v", err)
	}
	if err := w.Close(); err != nil {
		tb.Fatalf("Close: %v", err)
	}
	pkg := pkgBuf.Bytes()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		tb.Fatalf("GenerateKey: %v", err)
	}
	src, err := NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		tb.Fatalf("NewReader: %v", err)
	}
	var signedBuf bytes.Buffer
	if err := SignPackage(src, &signedBuf, key, fuzzTestCert(tb, key), []string{fuzzSignedPart}); err != nil {
		tb.Fatalf("SignPackage: %v", err)
	}
	signed := signedBuf.Bytes()

	r, err := NewReader(bytes.NewReader(signed), int64(len(signed)))
	if err != nil {
		tb.Fatalf("NewReader(signed): %v", err)
	}
	infos, err := r.VerifySignatures()
	if err != nil || len(infos) != 1 || !infos[0].Valid {
		tb.Fatalf("fixture signature is not valid: err=%v infos=%+v", err, infos)
	}
	covered := make(map[string]bool, len(infos[0].CoveredParts))
	for _, p := range infos[0].CoveredParts {
		covered[p] = true
	}
	if !covered[fuzzSignedPart] || covered[fuzzUnsignedPart] {
		tb.Fatalf("fixture coverage is %v; want %s covered and %s not", infos[0].CoveredParts, fuzzSignedPart, fuzzUnsignedPart)
	}

	f := r.GetFile(signaturePartName)
	if f == nil {
		tb.Fatalf("signed package has no %s", signaturePartName)
	}
	sigXML, err := f.ReadAll()
	if err != nil {
		tb.Fatalf("reading signature part: %v", err)
	}
	return sigFixture{signed: signed, sigXML: sigXML, covered: covered}
}

const (
	fuzzSignedPart   = "/word/document.xml"
	fuzzUnsignedPart = "/word/media/image1.png"
)

// forgedManifestObject is the C362 payload: an <Object> whose Manifest claims a
// correct digest for a part the signature does not cover. Appending it leaves
// SignedInfo — the only thing the private key covers — untouched.
func forgedManifestObject(tb testing.TB, fix sigFixture, id string) string {
	tb.Helper()
	r, err := NewReader(bytes.NewReader(fix.signed), int64(len(fix.signed)))
	if err != nil {
		tb.Fatalf("NewReader: %v", err)
	}
	f := r.GetFile(fuzzUnsignedPart)
	if f == nil {
		tb.Fatalf("package has no %s", fuzzUnsignedPart)
	}
	data, err := f.ReadAll()
	if err != nil {
		tb.Fatalf("reading %s: %v", fuzzUnsignedPart, err)
	}
	sum := sha256.Sum256(data)
	idAttr := ""
	if id != "" {
		idAttr = ` Id="` + id + `"`
	}
	return `<Object xmlns="` + nsXMLDSig + `"` + idAttr + `><Manifest><Reference URI="` +
		fuzzUnsignedPart + `"><DigestMethod Algorithm="` + spinecrypto.AlgSHA256 + `"/><DigestValue>` +
		base64.StdEncoding.EncodeToString(sum[:]) + `</DigestValue></Reference></Manifest></Object>`
}

// insertBefore returns the signature XML with xml spliced in before the first
// occurrence of mark.
func insertBefore(tb testing.TB, sigXML []byte, mark, xml string) []byte {
	tb.Helper()
	s := string(sigXML)
	i := strings.Index(s, mark)
	if i < 0 {
		tb.Fatalf("signature XML has no %q", mark)
	}
	return []byte(s[:i] + xml + s[i:])
}

// FuzzSignatureXML replaces the signature part of a validly signed package with
// fuzzed bytes and verifies the result.
//
// Beyond "no panic" and the resource budget it asserts the coverage contract:
//
//   - coverage is never claimed when SignatureValue did not verify;
//   - coverage is never claimed without an <Object> reference that verified,
//     which is the only thing that binds a manifest into the signature;
//   - a signature reported Valid has a verified SignedInfo and at least one
//     reference, all of them valid;
//   - and the containment property above: no fuzzed signature may report
//     coverage of a part the genuine signature does not cover (C362).
func FuzzSignatureXML(f *testing.F) {
	fix := buildSigFixture(f)
	entry := strings.TrimPrefix(signaturePartName, "/")

	f.Add(fix.sigXML)

	// The C362 forgery, with and without an Id, and doubled.
	forged := forgedManifestObject(f, fix, "idEvilObject")
	f.Add(insertBefore(f, fix.sigXML, "</Signature>", forged))
	f.Add(insertBefore(f, fix.sigXML, "</Signature>", forgedManifestObject(f, fix, "")))
	f.Add(insertBefore(f, fix.sigXML, "</Signature>", forged+forged))

	// The same payload smuggled into the covered Object's own manifest, which
	// breaks that Object's digest, and one that reuses the covered Object's Id.
	f.Add(insertBefore(f, fix.sigXML, "</Manifest>", `<Reference URI="`+fuzzUnsignedPart+
		`"><DigestMethod Algorithm="`+spinecrypto.AlgSHA256+`"/><DigestValue></DigestValue></Reference>`))
	f.Add(insertBefore(f, fix.sigXML, "</Signature>", forgedManifestObject(f, fix, idPackageObject)))

	// Structural damage: truncations, a broken SignatureValue, no XML at all,
	// and a deeply nested document.
	f.Add(fix.sigXML[:len(fix.sigXML)/2])
	f.Add(bytes.Replace(fix.sigXML, []byte("<SignatureValue"), []byte("<SignatureXalue"), 1))
	f.Add([]byte(nil))
	f.Add([]byte("<Signature/>"))
	f.Add([]byte(`<Signature xmlns="` + nsXMLDSig + `"><SignedInfo/><SignatureValue>!!!</SignatureValue><KeyInfo/></Signature>`))
	f.Add([]byte(strings.Repeat("<a>", 400) + strings.Repeat("</a>", 400)))

	f.Fuzz(func(t *testing.T, sigXML []byte) {
		pkg := fuzzseed.ReplaceZipEntry(fix.signed, entry, sigXML)
		if pkg == nil {
			return
		}

		var (
			infos []SignatureInfo
			err   error
		)
		signatureBudget.Check(t, len(sigXML), func() {
			r, rerr := NewReader(bytes.NewReader(pkg), int64(len(pkg)), WithReaderOptions(fuzzReaderOptions()))
			if rerr != nil {
				infos, err = nil, nil
				return
			}
			infos, err = r.VerifySignatures()
		})
		if err != nil {
			if infos != nil {
				t.Fatalf("VerifySignatures returned %d infos with error %v", len(infos), err)
			}
			return
		}

		for _, info := range infos {
			if !info.SignedInfoValid && len(info.CoveredParts) > 0 {
				t.Fatalf("coverage %v claimed although SignatureValue did not verify", info.CoveredParts)
			}
			if len(info.CoveredParts) > 0 && !hasValidObjectReference(info) {
				t.Fatalf("coverage %v claimed with no verified Object reference: %+v", info.CoveredParts, info.References)
			}
			if info.Valid {
				if !info.SignedInfoValid {
					t.Fatal("signature reported Valid without a verified SignedInfo")
				}
				if len(info.References) == 0 {
					t.Fatal("signature reported Valid with no references checked")
				}
				for _, ref := range info.References {
					if !ref.Valid {
						t.Fatalf("signature reported Valid with a failing reference: %+v", ref)
					}
				}
			}
			for _, part := range info.CoveredParts {
				if !fix.covered[part] {
					t.Fatalf("signature claims coverage of %q, which the genuine signature does not cover (covered: %v)", part, info.CoveredParts)
				}
			}
		}
	})
}

// hasValidObjectReference reports whether any same-document Object reference
// verified. Nothing else can bind a manifest into the signature.
func hasValidObjectReference(info SignatureInfo) bool {
	for _, ref := range info.References {
		if ref.Kind == "object" && ref.Valid {
			return true
		}
	}
	return false
}

// TestSignatureFuzzBudgetAllowsLargeSignature is the legitimate-input evidence
// for signatureBudget: a package with 300 signed parts, whose signature carries
// a manifest reference for each. It is the largest signature this library
// produces without a contrived fixture, and the case that a rate tightened on
// the strength of a small signature alone would break.
func TestSignatureFuzzBudgetAllowsLargeSignature(t *testing.T) {
	var pkgBuf bytes.Buffer
	w := NewWriter(&pkgBuf)
	var parts []string
	for i := 0; i < 300; i++ {
		name := fmt.Sprintf("/word/part%03d.xml", i)
		if err := w.WritePart(name, "application/xml", []byte(strings.Repeat("<p>x</p>", 50))); err != nil {
			t.Fatalf("WritePart: %v", err)
		}
		parts = append(parts, name)
	}
	if _, err := w.AddRelationship(RelTypeOfficeDocument, "word/part000.xml", TargetModeInternal); err != nil {
		t.Fatalf("AddRelationship: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	pkg := pkgBuf.Bytes()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	src, err := NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	var signedBuf bytes.Buffer
	if err := SignPackage(src, &signedBuf, key, fuzzTestCert(t, key), parts); err != nil {
		t.Fatalf("SignPackage: %v", err)
	}
	signed := signedBuf.Bytes()

	r, err := NewReader(bytes.NewReader(signed), int64(len(signed)))
	if err != nil {
		t.Fatalf("NewReader(signed): %v", err)
	}
	sigFile := r.GetFile(signaturePartName)
	if sigFile == nil {
		t.Fatalf("signed package has no %s", signaturePartName)
	}
	sigXML, err := sigFile.ReadAll()
	if err != nil {
		t.Fatalf("reading signature part: %v", err)
	}

	var infos []SignatureInfo
	var verr error
	budgetHeadroom(t, signatureBudget, len(sigXML), func() {
		rr, rerr := NewReader(bytes.NewReader(signed), int64(len(signed)))
		if rerr != nil {
			verr = rerr
			return
		}
		infos, verr = rr.VerifySignatures()
	})
	if verr != nil {
		t.Fatalf("VerifySignatures: %v", verr)
	}
	if len(infos) != 1 || !infos[0].Valid {
		t.Fatalf("large signature is not valid: %+v", infos)
	}
	if len(infos[0].CoveredParts) < len(parts) {
		t.Fatalf("coverage lists %d parts, want at least %d", len(infos[0].CoveredParts), len(parts))
	}
}
