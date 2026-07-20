package opc

import (
	"archive/zip"
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"strings"
	"testing"
	"time"
)

// testCert builds a self-signed certificate for signer's public key.
func testCert(t *testing.T, signer crypto.Signer) *x509.Certificate {
	t.Helper()
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Spine Test Signer"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, signer.Public(), signer)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	return cert
}

// buildMinimalPackage produces a tiny but valid OPC package with a single
// document part, an image part, and package relationships.
func buildMinimalPackage(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.WritePart("/word/document.xml", "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml", []byte("<document>hello</document>")); err != nil {
		t.Fatalf("WritePart document: %v", err)
	}
	if err := w.WritePart("/word/media/image1.png", "", []byte("\x89PNG\r\n\x1a\nfake-image-bytes")); err != nil {
		t.Fatalf("WritePart image: %v", err)
	}
	if _, err := w.AddRelationship(RelTypeOfficeDocument, "word/document.xml", TargetModeInternal); err != nil {
		t.Fatalf("AddRelationship: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return buf.Bytes()
}

func openBytes(t *testing.T, data []byte) *Reader {
	t.Helper()
	r, err := NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	return r
}

func signAndReopen(t *testing.T, pkg []byte, signer crypto.Signer, cert *x509.Certificate, parts []string) *Reader {
	t.Helper()
	src := openBytes(t, pkg)
	var out bytes.Buffer
	if err := SignPackage(src, &out, signer, cert, parts); err != nil {
		t.Fatalf("SignPackage: %v", err)
	}
	return openBytes(t, out.Bytes())
}

func requireOneValidSignature(t *testing.T, r *Reader) SignatureInfo {
	t.Helper()
	infos, err := r.VerifySignatures()
	if err != nil {
		t.Fatalf("VerifySignatures: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("got %d signatures, want 1", len(infos))
	}
	info := infos[0]
	if !info.Valid {
		t.Fatalf("signature not valid: problems=%v", info.Problems)
	}
	if !info.SignedInfoValid {
		t.Fatalf("SignedInfo did not verify: %v", info.Problems)
	}
	return info
}

func TestSignVerifyRoundTripRSA(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	cert := testCert(t, key)
	pkg := buildMinimalPackage(t)

	r := signAndReopen(t, pkg, key, cert, nil)
	info := requireOneValidSignature(t, r)

	if info.SignerSubject == "" {
		t.Error("empty signer subject")
	}
	if info.SigningTime.IsZero() {
		t.Error("signing time not recorded")
	}
	// Every reference must have verified.
	for _, ref := range info.References {
		if !ref.Valid {
			t.Errorf("reference %s (%s) invalid: %s", ref.URI, ref.Kind, ref.Detail)
		}
	}
	// The document part and package relationships must be covered.
	covered := map[string]bool{}
	for _, p := range info.CoveredParts {
		covered[p] = true
	}
	if !covered["/word/document.xml"] {
		t.Errorf("document.xml not covered; covered=%v", info.CoveredParts)
	}
	if !covered["/word/media/image1.png"] {
		t.Errorf("image not covered; covered=%v", info.CoveredParts)
	}
	var sawRels bool
	for _, ref := range info.References {
		if ref.Kind == "relationships" {
			sawRels = true
		}
	}
	if !sawRels {
		t.Error("no relationships reference was verified")
	}
}

func TestSignVerifyRoundTripECDSA(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	cert := testCert(t, key)
	pkg := buildMinimalPackage(t)

	r := signAndReopen(t, pkg, key, cert, nil)
	requireOneValidSignature(t, r)
}

func TestSignExplicitParts(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	cert := testCert(t, key)
	pkg := buildMinimalPackage(t)

	// Sign only the document; the image should not be covered.
	r := signAndReopen(t, pkg, key, cert, []string{"/word/document.xml"})
	info := requireOneValidSignature(t, r)

	for _, p := range info.CoveredParts {
		if p == "/word/media/image1.png" {
			t.Error("image should not be covered when signing only the document")
		}
	}
}

func TestVerifyDetectsTamperedPart(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	cert := testCert(t, key)
	pkg := buildMinimalPackage(t)

	src := openBytes(t, pkg)
	var signed bytes.Buffer
	if err := SignPackage(src, &signed, key, cert, nil); err != nil {
		t.Fatalf("SignPackage: %v", err)
	}

	// Rewrite the signed package with document.xml altered but the signature
	// left intact.
	tampered := rewriteZipEntry(t, signed.Bytes(), "word/document.xml", []byte("<document>TAMPERED</document>"))

	r := openBytes(t, tampered)
	infos, err := r.VerifySignatures()
	if err != nil {
		t.Fatalf("VerifySignatures: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("got %d signatures, want 1", len(infos))
	}
	info := infos[0]
	// The SignedInfo signature itself still verifies (the certificate is
	// untouched), but the tampered part's digest must no longer match.
	if !info.SignedInfoValid {
		t.Error("SignedInfo should still verify after a content-only tamper")
	}
	if info.Valid {
		t.Fatal("tampered package reported as valid")
	}
	var docRef *ReferenceResult
	for i := range info.References {
		if partNameFromURI(info.References[i].URI) == "/word/document.xml" {
			docRef = &info.References[i]
		}
	}
	if docRef == nil {
		t.Fatal("no reference for document.xml")
	}
	if docRef.Valid {
		t.Error("document.xml reference should be invalid after tampering")
	}
}

func TestSignIncludesOfficeObject(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	cert := testCert(t, key)
	pkg := buildMinimalPackage(t)

	src := openBytes(t, pkg)
	var signed bytes.Buffer
	if err := SignPackage(src, &signed, key, cert, nil); err != nil {
		t.Fatalf("SignPackage: %v", err)
	}
	r := openBytes(t, signed.Bytes())

	// The signature must be valid, which requires the Office object's digest
	// (referenced from SignedInfo) to match: this covers the object end to end.
	requireOneValidSignature(t, r)

	sigXML, err := r.GetFile(signaturePartName).ReadAll()
	if err != nil {
		t.Fatalf("reading signature part: %v", err)
	}
	s := string(sigXML)
	for _, want := range []string{
		`Id="idOfficeObject"`,
		`<SignatureInfoV1 xmlns="` + nsOfficeDigSig + `">`,
		`URI="#idOfficeObject"`,
		`<SignatureType>1</SignatureType>`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("signature XML missing %q", want)
		}
	}

	// The Office object reference must be among the verified references.
	info := requireOneValidSignature(t, r)
	var sawOffice bool
	for _, ref := range info.References {
		if ref.URI == "#idOfficeObject" {
			sawOffice = true
			if !ref.Valid {
				t.Errorf("Office object reference invalid: %s", ref.Detail)
			}
		}
	}
	if !sawOffice {
		t.Error("Office object reference not verified")
	}
}

func TestVerifyUnsignedPackage(t *testing.T) {
	pkg := buildMinimalPackage(t)
	r := openBytes(t, pkg)
	infos, err := r.VerifySignatures()
	if err != nil {
		t.Fatalf("VerifySignatures: %v", err)
	}
	if len(infos) != 0 {
		t.Fatalf("unsigned package reported %d signatures", len(infos))
	}
}

// rewriteZipEntry returns a copy of the zip in data with the named entry's
// contents replaced. Other entries are copied verbatim.
func rewriteZipEntry(t *testing.T, data []byte, name string, content []byte) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, f := range zr.File {
		hdr := f.FileHeader
		w, err := zw.CreateHeader(&hdr)
		if err != nil {
			t.Fatalf("CreateHeader: %v", err)
		}
		var body []byte
		if f.Name == name {
			body = content
		} else {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open entry %s: %v", f.Name, err)
			}
			body, err = io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				t.Fatalf("read entry %s: %v", f.Name, err)
			}
		}
		if _, err := w.Write(body); err != nil {
			t.Fatalf("write entry %s: %v", f.Name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return out.Bytes()
}
