package opc

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"

	spinecrypto "github.com/mgilbir/spine/common/crypto"
)

// signedPackage signs pkg over parts and returns the signed package bytes.
func signedPackage(t *testing.T, pkg []byte, parts []string) ([]byte, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	cert := testCert(t, key)
	src := openBytes(t, pkg)
	var signed bytes.Buffer
	if err := SignPackage(src, &signed, key, cert, parts); err != nil {
		t.Fatalf("SignPackage: %v", err)
	}
	return signed.Bytes(), key
}

// injectIntoSignature rewrites the signature part of a signed package,
// inserting xml immediately before mark (the first occurrence). It leaves
// SignedInfo, SignatureValue and KeyInfo untouched — exactly the freedom an
// attacker who does not hold the private key has.
func injectIntoSignature(t *testing.T, signed []byte, mark, xml string) []byte {
	t.Helper()
	r := openBytes(t, signed)
	f := r.GetFile(signaturePartName)
	if f == nil {
		t.Fatalf("signature part %s missing", signaturePartName)
	}
	sigXML, err := f.ReadAll()
	if err != nil {
		t.Fatalf("reading signature part: %v", err)
	}
	s := string(sigXML)
	i := strings.Index(s, mark)
	if i < 0 {
		t.Fatalf("signature XML has no %q to inject before:\n%s", mark, s)
	}
	forged := s[:i] + xml + s[i:]
	return rewriteZipEntry(t, signed, strings.TrimPrefix(signaturePartName, "/"), []byte(forged))
}

// manifestObject builds an <Object> whose Manifest claims a correct SHA-256
// digest for partName — the payload of the C362 forgery.
func manifestObject(t *testing.T, pkg []byte, id, partName string) string {
	t.Helper()
	r := openBytes(t, pkg)
	f := r.GetFile(partName)
	if f == nil {
		t.Fatalf("part %s missing from package", partName)
	}
	data, err := f.ReadAll()
	if err != nil {
		t.Fatalf("reading %s: %v", partName, err)
	}
	sum := sha256.Sum256(data)
	idAttr := ""
	if id != "" {
		idAttr = ` Id="` + id + `"`
	}
	return `<Object xmlns="` + nsXMLDSig + `"` + idAttr + `><Manifest><Reference URI="` +
		partName + `"><DigestMethod Algorithm="` + spinecrypto.AlgSHA256 + `"/><DigestValue>` +
		base64.StdEncoding.EncodeToString(sum[:]) + `</DigestValue></Reference></Manifest></Object>`
}

func verifyOne(t *testing.T, pkg []byte) SignatureInfo {
	t.Helper()
	infos, err := openBytes(t, pkg).VerifySignatures()
	if err != nil {
		t.Fatalf("VerifySignatures: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("got %d signatures, want 1", len(infos))
	}
	return infos[0]
}

func covers(info SignatureInfo, part string) bool {
	for _, p := range info.CoveredParts {
		if p == part {
			return true
		}
	}
	return false
}

// TestVerifyRejectsCoverageFromUnsignedObject is the C362 regression: a package
// is signed over /word/document.xml only, then an <Object> whose Manifest lists
// /word/media/image1.png with a correct digest is appended. SignedInfo — the
// only thing the private key covers — is untouched, so the injected Object is
// not covered by the signature and the part it claims must not be reported as
// signed.
func TestVerifyRejectsCoverageFromUnsignedObject(t *testing.T) {
	pkg := buildMinimalPackage(t)
	signed, _ := signedPackage(t, pkg, []string{"/word/document.xml"})

	forged := injectIntoSignature(t, signed, "</Signature>",
		manifestObject(t, pkg, "idEvilObject", "/word/media/image1.png"))

	info := verifyOne(t, forged)
	if !info.SignedInfoValid {
		t.Fatalf("SignedInfo should still verify after appending an Object: %v", info.Problems)
	}
	if covers(info, "/word/media/image1.png") {
		t.Errorf("forged Object's part reported as covered: %v", info.CoveredParts)
	}
	if !covers(info, "/word/document.xml") {
		t.Errorf("genuinely signed part missing from coverage: %v", info.CoveredParts)
	}
	if info.Valid {
		t.Error("package carrying an unsigned coverage claim reported as Valid")
	}
	if !hasProblemContaining(info, "idEvilObject") {
		t.Errorf("forged Object not reported in Problems: %v", info.Problems)
	}
	for _, ref := range info.References {
		if strings.Contains(ref.URI, "image1.png") {
			t.Errorf("reference from the unsigned Object was checked and reported: %+v", ref)
		}
	}
}

// TestVerifyRejectsCoverageFromUnsignedObjectWithoutID covers the same forgery
// with the Id omitted, which no SignedInfo reference could ever address.
func TestVerifyRejectsCoverageFromUnsignedObjectWithoutID(t *testing.T) {
	pkg := buildMinimalPackage(t)
	signed, _ := signedPackage(t, pkg, []string{"/word/document.xml"})

	forged := injectIntoSignature(t, signed, "</Signature>",
		manifestObject(t, pkg, "", "/word/media/image1.png"))

	info := verifyOne(t, forged)
	if covers(info, "/word/media/image1.png") {
		t.Errorf("forged Object's part reported as covered: %v", info.CoveredParts)
	}
	if info.Valid {
		t.Error("package carrying an unsigned coverage claim reported as Valid")
	}
	if !hasProblemContaining(info, "no Id") {
		t.Errorf("anonymous forged Object not reported in Problems: %v", info.Problems)
	}
}

// TestVerifyDuplicateObjectIDCannotShadow appends a second <Object> reusing the
// signed Object's Id. The digest the signature covers was computed over the
// first Object in document order, so the appended twin must not inherit its
// trust.
func TestVerifyDuplicateObjectIDCannotShadow(t *testing.T) {
	pkg := buildMinimalPackage(t)
	signed, _ := signedPackage(t, pkg, []string{"/word/document.xml"})

	forged := injectIntoSignature(t, signed, "</Signature>",
		manifestObject(t, pkg, idPackageObject, "/word/media/image1.png"))

	info := verifyOne(t, forged)
	if !info.SignedInfoValid {
		t.Fatalf("SignedInfo should still verify: %v", info.Problems)
	}
	if covers(info, "/word/media/image1.png") {
		t.Errorf("duplicate-Id Object's part reported as covered: %v", info.CoveredParts)
	}
	if !covers(info, "/word/document.xml") {
		t.Errorf("genuinely signed part missing from coverage: %v", info.CoveredParts)
	}
	if info.Valid {
		t.Error("package carrying a duplicate-Id coverage claim reported as Valid")
	}
}

// TestVerifyPrependedDecoyObjectFailsClosed puts the twin *before* the genuine
// Object instead. Digest resolution takes the first match in document order, so
// the reference now digests the decoy and fails — nothing is reported as
// covered.
func TestVerifyPrependedDecoyObjectFailsClosed(t *testing.T) {
	pkg := buildMinimalPackage(t)
	signed, _ := signedPackage(t, pkg, []string{"/word/document.xml"})

	forged := injectIntoSignature(t, signed, `<Object xmlns="`+nsXMLDSig+`" Id="`+idPackageObject+`">`,
		manifestObject(t, pkg, idPackageObject, "/word/media/image1.png"))

	info := verifyOne(t, forged)
	if !info.SignedInfoValid {
		t.Fatalf("SignedInfo should still verify: %v", info.Problems)
	}
	if len(info.CoveredParts) != 0 {
		t.Errorf("decoy Object produced coverage: %v", info.CoveredParts)
	}
	if info.Valid {
		t.Error("package with a decoy Object reported as Valid")
	}
}

// TestVerifyIgnoresObjectWhoseReferenceFails is the second negative case: the
// signed Object is edited, so the SignedInfo reference addressing it no longer
// verifies. An Object reached by a reference that failed contributes no
// coverage.
func TestVerifyIgnoresObjectWhoseReferenceFails(t *testing.T) {
	pkg := buildMinimalPackage(t)
	signed, _ := signedPackage(t, pkg, []string{"/word/document.xml"})

	// Insert a harmless extra element into the signed Object: its canonical
	// form changes, so its digest no longer matches the signed one.
	forged := injectIntoSignature(t, signed, "</Manifest>", "<Decoy></Decoy>")

	info := verifyOne(t, forged)
	if !info.SignedInfoValid {
		t.Fatalf("SignedInfo should still verify: %v", info.Problems)
	}
	if len(info.CoveredParts) != 0 {
		t.Errorf("Object with a failed reference produced coverage: %v", info.CoveredParts)
	}
	if info.Valid {
		t.Error("package with an altered signed Object reported as Valid")
	}
}

// TestVerifyCoverageRequiresSignedInfoToVerify checks the outer gate: when
// SignatureValue itself does not verify, nothing is protected, so no part is
// reported as covered even though every digest still matches.
func TestVerifyCoverageRequiresSignedInfoToVerify(t *testing.T) {
	pkg := buildMinimalPackage(t)
	signed, _ := signedPackage(t, pkg, nil)

	r := openBytes(t, signed)
	sigXML, err := r.GetFile(signaturePartName).ReadAll()
	if err != nil {
		t.Fatalf("reading signature part: %v", err)
	}
	// Swap the certificate for one belonging to a different key: KeyInfo is not
	// covered by the signature, so this is an edit an attacker can make.
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	otherCert := testCert(t, otherKey)
	s := string(sigXML)
	start := strings.Index(s, "<X509Certificate>") + len("<X509Certificate>")
	end := strings.Index(s, "</X509Certificate>")
	if start < len("<X509Certificate>") || end < start {
		t.Fatalf("signature XML has no X509Certificate:\n%s", s)
	}
	s = s[:start] + base64.StdEncoding.EncodeToString(otherCert.Raw) + s[end:]
	swapped := rewriteZipEntry(t, signed, strings.TrimPrefix(signaturePartName, "/"), []byte(s))

	info := verifyOne(t, swapped)
	if info.SignedInfoValid || info.Valid {
		t.Fatalf("signature verified against a foreign certificate: %+v", info.Problems)
	}
	if len(info.CoveredParts) != 0 {
		t.Errorf("coverage reported for a signature that does not verify: %v", info.CoveredParts)
	}
	// The per-reference detail stays available for diagnosis.
	if len(info.References) == 0 {
		t.Error("no references reported; the diagnostic detail was lost")
	}
}

// TestVerifyAcceptsUnreferencedPropertiesObject guards the reachability rule
// against real producer layouts: Office signatures carry an <Object> holding the
// XAdES QualifyingProperties whose *child* SignedProperties is what SignedInfo
// references — the Object itself has no Id and is not referenced. Such an Object
// claims no package coverage and must not make a genuine signature invalid.
func TestVerifyAcceptsUnreferencedPropertiesObject(t *testing.T) {
	pkg := buildMinimalPackage(t)
	signed, _ := signedPackage(t, pkg, nil)

	before := verifyOne(t, signed)
	if !before.Valid {
		t.Fatalf("baseline signature not valid: %v", before.Problems)
	}

	xades := `<Object xmlns="` + nsXMLDSig + `"><xd:QualifyingProperties ` +
		`xmlns:xd="http://uri.etsi.org/01903/v1.3.2#" Target="#` + idPackageSignature + `">` +
		`<xd:SignedProperties Id="idSignedProperties"><xd:SignedSignatureProperties>` +
		`<xd:SigningTime>2026-07-27T00:00:00Z</xd:SigningTime>` +
		`</xd:SignedSignatureProperties></xd:SignedProperties></xd:QualifyingProperties></Object>`
	withXAdES := injectIntoSignature(t, signed, "</Signature>", xades)

	info := verifyOne(t, withXAdES)
	if !info.Valid {
		t.Errorf("signature with an unreferenced properties Object reported invalid: %v", info.Problems)
	}
	if len(info.Problems) != 0 {
		t.Errorf("unexpected problems: %v", info.Problems)
	}
	if len(info.CoveredParts) != len(before.CoveredParts) {
		t.Errorf("coverage changed: got %v, want %v", info.CoveredParts, before.CoveredParts)
	}
}

// TestVerifyIgnoresSigningTimeOfUnsignedObject checks that the reported signing
// time also comes from covered Objects only.
func TestVerifyIgnoresSigningTimeOfUnsignedObject(t *testing.T) {
	pkg := buildMinimalPackage(t)
	signed, _ := signedPackage(t, pkg, nil)

	genuine := verifyOne(t, signed)
	if genuine.SigningTime.IsZero() {
		t.Fatal("baseline signature has no signing time")
	}

	forged := injectIntoSignature(t, signed, "</Signature>",
		`<Object xmlns="`+nsXMLDSig+`" Id="idFakeTime"><SignatureProperties>`+
			`<SignatureProperty Id="idSignatureTime" Target="#`+idPackageSignature+`">`+
			`<mdssi:SignatureTime xmlns:mdssi="`+nsMDSSI+`">`+
			`<mdssi:Format>YYYY-MM-DDThh:mm:ssTZD</mdssi:Format>`+
			`<mdssi:Value>1999-01-01T00:00:00Z</mdssi:Value>`+
			`</mdssi:SignatureTime></SignatureProperty></SignatureProperties></Object>`)

	info := verifyOne(t, forged)
	if !info.SigningTime.Equal(genuine.SigningTime) {
		t.Errorf("signing time taken from an unsigned Object: got %s, want %s",
			info.SigningTime, genuine.SigningTime)
	}
}

func hasProblemContaining(info SignatureInfo, substr string) bool {
	for _, p := range info.Problems {
		if strings.Contains(p, substr) {
			return true
		}
	}
	return false
}

// ---- C391: reference URI escaping -----------------------------------------

func TestPartReferenceURIEscaping(t *testing.T) {
	tests := []struct {
		name     string
		part     string
		ct       string
		wantURI  string
		wantPart string
	}{{
		name:     "conformant name and type are emitted verbatim",
		part:     "/word/document.xml",
		ct:       "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml",
		wantURI:  "/word/document.xml?ContentType=application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml",
		wantPart: "/word/document.xml",
	}, {
		name:     "percent escapes in a conformant part name are preserved",
		part:     "/word/media/a%41.png",
		ct:       "image/png",
		wantURI:  "/word/media/a%41.png?ContentType=image/png",
		wantPart: "/word/media/a%41.png",
	}, {
		name:     "sub-delims a part name may carry stay literal",
		part:     "/word/a&b,c(d).xml",
		ct:       "application/xml",
		wantURI:  "/word/a&b,c(d).xml?ContentType=application/xml",
		wantPart: "/word/a&b,c(d).xml",
	}, {
		name:     "a space in a wild part name is escaped",
		part:     "/word/media/my image.png",
		ct:       "image/png",
		wantURI:  "/word/media/my%20image.png?ContentType=image/png",
		wantPart: "/word/media/my image.png",
	}, {
		name:     "a bare percent in a wild part name is escaped",
		part:     "/word/50%.xml",
		ct:       "",
		wantURI:  "/word/50%25.xml",
		wantPart: "/word/50%.xml",
	}, {
		name:     "an ampersand in a content type cannot split the query",
		part:     "/word/document.xml",
		ct:       "application/x-a&b",
		wantURI:  "/word/document.xml?ContentType=application/x-a%26b",
		wantPart: "/word/document.xml",
	}, {
		name:     "a space in a content type is escaped",
		part:     "/word/document.xml",
		ct:       "text/plain; charset=utf-8",
		wantURI:  "/word/document.xml?ContentType=text/plain;%20charset=utf-8",
		wantPart: "/word/document.xml",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := partReferenceURI(tc.part, tc.ct)
			if got != tc.wantURI {
				t.Errorf("partReferenceURI(%q, %q) = %q, want %q", tc.part, tc.ct, got, tc.wantURI)
			}
			// The URI must be usable as an XML attribute value and as a URI:
			// nothing outside the URI character set may survive.
			for i := 0; i < len(got); i++ {
				if c := got[i]; c <= ' ' || c == '"' || c == '<' || c == '>' || c >= 0x7f {
					t.Fatalf("reference URI %q contains illegal byte %q", got, rune(c))
				}
			}
			// Round-trip: the part the URI addresses must be recoverable. The
			// package-less decode is the escaped-producer reading; a package
			// that literally contains the name resolves to it directly.
			if decoded := partNameFromURI(got); decoded != tc.wantPart && literalPartNameFromURI(got) != tc.wantPart {
				t.Errorf("partNameFromURI(%q) = %q and literal = %q, want %q",
					got, decoded, literalPartNameFromURI(got), tc.wantPart)
			}
		})
	}
}

// TestSignVerifyPercentEncodedPartName is the C391 regression: a part name
// containing a percent-escape is legal under the OPC grammar, and signing it
// must produce a manifest reference that verification resolves back to the same
// part. Before the fix the reference was written raw and read back
// percent-decoded, so spine reported its own signature as covering a part that
// does not exist.
func TestSignVerifyPercentEncodedPartName(t *testing.T) {
	const partName = "/word/media/a%41.png"
	if err := ValidatePartName(partName); err != nil {
		t.Fatalf("%s should be a legal part name: %v", partName, err)
	}

	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.WritePart("/word/document.xml", "application/xml", []byte("<document/>")); err != nil {
		t.Fatalf("WritePart document: %v", err)
	}
	if err := w.WritePart(partName, "image/png", []byte("\x89PNG\r\n\x1a\npixels")); err != nil {
		t.Fatalf("WritePart image: %v", err)
	}
	if _, err := w.AddRelationship(RelTypeOfficeDocument, "word/document.xml", TargetModeInternal); err != nil {
		t.Fatalf("AddRelationship: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	signed, _ := signedPackage(t, buf.Bytes(), nil)
	info := verifyOne(t, signed)
	if !info.Valid {
		t.Fatalf("signature over a percent-escaped part name not valid: %v", info.Problems)
	}
	if !covers(info, partName) {
		t.Errorf("%s not covered; covered=%v", partName, info.CoveredParts)
	}
}

// TestSignVerifyPartNameNeedingEscape signs a package whose part name the OPC
// grammar forbids but which wild packages carry verbatim (a literal space). The
// manifest reference must be escaped so it stays a well-formed URI, and
// verification must resolve it back to the part.
func TestSignVerifyPartNameNeedingEscape(t *testing.T) {
	const partName = "/word/media/my image.png"

	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.WritePart("/word/document.xml", "application/xml", []byte("<document/>")); err != nil {
		t.Fatalf("WritePart document: %v", err)
	}
	if err := w.WritePreservedPart(partName, "image/png", []byte("\x89PNG\r\n\x1a\npixels")); err != nil {
		t.Fatalf("WritePreservedPart image: %v", err)
	}
	if _, err := w.AddRelationship(RelTypeOfficeDocument, "word/document.xml", TargetModeInternal); err != nil {
		t.Fatalf("AddRelationship: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	signed, _ := signedPackage(t, buf.Bytes(), nil)
	r := openBytes(t, signed)
	sigXML, err := r.GetFile(signaturePartName).ReadAll()
	if err != nil {
		t.Fatalf("reading signature part: %v", err)
	}
	if bytes.Contains(sigXML, []byte("my image.png")) {
		t.Error("manifest reference carries a raw space")
	}
	if !bytes.Contains(sigXML, []byte("my%20image.png")) {
		t.Errorf("manifest reference is not percent-encoded:\n%s", sigXML)
	}

	info := verifyOne(t, signed)
	if !info.Valid {
		t.Fatalf("signature over an escaped part name not valid: %v", info.Problems)
	}
	if !covers(info, partName) {
		t.Errorf("%s not covered; covered=%v", partName, info.CoveredParts)
	}
}

// ---- C460: algorithm reporting ---------------------------------------------

func TestSignatureInfoReportsDigestMethods(t *testing.T) {
	pkg := buildMinimalPackage(t)
	signed, _ := signedPackage(t, pkg, nil)

	info := verifyOne(t, signed)
	if !info.Valid {
		t.Fatalf("signature not valid: %v", info.Problems)
	}
	if info.SignatureMethod != spinecrypto.AlgRSASHA256 {
		t.Errorf("SignatureMethod = %q, want %q", info.SignatureMethod, spinecrypto.AlgRSASHA256)
	}
	if len(info.DigestMethods) != 1 || info.DigestMethods[0] != spinecrypto.AlgSHA256 {
		t.Errorf("DigestMethods = %v, want [%s]", info.DigestMethods, spinecrypto.AlgSHA256)
	}
	if info.UsesWeakAlgorithms() || len(info.WeakAlgorithms) != 0 {
		t.Errorf("SHA-256 signature reported as weak: %v", info.WeakAlgorithms)
	}
}

// TestSignatureInfoFlagsSHA1 builds a SHA-1 signature — which the verifier
// accepts by design, for older Office signatures — and checks a caller can tell
// it apart from a SHA-256 one and refuse it.
func TestSignatureInfoFlagsSHA1(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	cert := testCert(t, key)
	src := openBytes(t, buildMinimalPackage(t))

	contentParts, relsParts := src.classifySignTargets(nil)
	sigXML, err := src.buildSignature(key, cert, spinecrypto.AlgRSASHA1, spinecrypto.AlgSHA1, contentParts, relsParts)
	if err != nil {
		t.Fatalf("buildSignature: %v", err)
	}
	var signed bytes.Buffer
	if err := src.writeSignedPackage(&signed, sigXML); err != nil {
		t.Fatalf("writeSignedPackage: %v", err)
	}

	info := verifyOne(t, signed.Bytes())
	if !info.Valid {
		t.Fatalf("SHA-1 signature should still verify: %v", info.Problems)
	}
	if !info.UsesWeakAlgorithms() {
		t.Fatal("SHA-1 signature not reported as weak")
	}
	weak := strings.Join(info.WeakAlgorithms, " ")
	for _, want := range []string{spinecrypto.AlgRSASHA1, spinecrypto.AlgSHA1} {
		if !strings.Contains(weak, want) {
			t.Errorf("WeakAlgorithms = %v, missing %s", info.WeakAlgorithms, want)
		}
	}
	if len(info.DigestMethods) != 1 || info.DigestMethods[0] != spinecrypto.AlgSHA1 {
		t.Errorf("DigestMethods = %v, want [%s]", info.DigestMethods, spinecrypto.AlgSHA1)
	}
}
