package opc

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/base64"
	stdxml "encoding/xml"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"time"

	spinecrypto "github.com/mgilbir/spine/common/crypto"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// This file implements OPC package digital signatures (ECMA-376 Part 2 §13):
// an XML Digital Signature (W3C XML-DSig) stored under _xmlsignatures/ that
// covers package parts (each referenced by a content-type-qualified URI and a
// digest), the package relationships (via the OPC Relationship Transform), and
// a signing-time property. Verification recomputes every digest and checks the
// SignatureValue over the canonicalized SignedInfo against the signer's
// embedded X.509 certificate; signing builds the same structure and writes the
// signature parts into a copy of the package.
//
// The trust chain is SignedInfo → Object → Manifest → part, and verification
// walks it as a chain: SignatureValue covers SignedInfo, a SignedInfo reference
// covers an <Object>, and only the manifest of a covered Object says anything
// about the package. An <Object> that no verified SignedInfo reference reaches
// is unsigned — anyone can append one to a signed package — so its manifest is
// reported as a Problem rather than as coverage.
//
// Canonicalization uses inclusive Canonical XML 1.0 (see common/xml/c14n.go).
// The signer emits SHA-256 digests and RSA-SHA256 / ECDSA-SHA256 signatures;
// the verifier additionally accepts the SHA-1 based algorithms that older
// Office signatures use.

const (
	// nsXMLDSig is the W3C XML Digital Signature namespace.
	nsXMLDSig = "http://www.w3.org/2000/09/xmldsig#"

	// nsMDSSI is the Microsoft/OPC digital-signature namespace used for the
	// Relationship Transform child elements and the signing-time property.
	nsMDSSI = "http://schemas.openxmlformats.org/package/2006/digital-signature"

	// algRelationshipTransform is the OPC Relationship Transform algorithm URI.
	algRelationshipTransform = "http://schemas.openxmlformats.org/package/2006/RelationshipTransform"

	// typeObject qualifies a same-document reference to an <Object> element.
	typeObject = nsXMLDSig + "Object"

	// RelTypeDigitalSignatureOrigin links the package to the signature origin
	// part (_xmlsignatures/origin.sigs).
	RelTypeDigitalSignatureOrigin = "http://schemas.openxmlformats.org/package/2006/relationships/digital-signature/origin"

	// ContentTypeDigitalSignatureOrigin is the content type of the signature
	// origin part.
	ContentTypeDigitalSignatureOrigin = "application/vnd.openxmlformats-package.digital-signature-origin"

	// ContentTypeDigitalSignature is the content type of an XML signature part.
	ContentTypeDigitalSignature = "application/vnd.openxmlformats-package.digital-signature-xmlsignature+xml"

	// originPartName and signaturePartName give the canonical locations of the
	// signature origin part and the first signature part.
	originPartName    = "/_xmlsignatures/origin.sigs"
	signaturePartName = "/_xmlsignatures/sig1.xml"

	// idPackageSignature and idPackageObject are the fixed XML ids Office uses
	// for the signature element and the package object; reusing them keeps the
	// output familiar to other tools.
	idPackageSignature = "idPackageSignature"
	idPackageObject    = "idPackageObject"
	idSignatureTime    = "idSignatureTime"

	// nsOfficeDigSig is the Microsoft Office digital-signature namespace carrying
	// the SignatureInfoV1 details Office's signature UI reads.
	nsOfficeDigSig = "http://schemas.microsoft.com/office/2006/digsig"

	// idOfficeObject and idOfficeV1Details are the fixed ids Office uses for its
	// application-specific signature Object and its SignatureInfoV1 property.
	idOfficeObject    = "idOfficeObject"
	idOfficeV1Details = "idOfficeV1Details"
)

// SignatureInfo reports the outcome of verifying one signature part.
type SignatureInfo struct {
	// PartName is the signature part, e.g. "/_xmlsignatures/sig1.xml".
	PartName string

	// Valid is true only when the SignatureValue verifies against the signer
	// certificate and every reference digest (objects, parts, relationships)
	// matches. A false value with a nil Err means the signature is well-formed
	// but does not match the current package contents (tampering or an
	// unsupported construct); Problems explains why.
	Valid bool

	// SignedInfoValid reports whether the SignatureValue verified against the
	// certificate's public key over the canonicalized SignedInfo. This is the
	// cryptographic authenticity check, independent of the content digests.
	SignedInfoValid bool

	// SignerCertificate is the signer's X.509 certificate (the first
	// certificate in KeyInfo/X509Data), or nil when it could not be parsed.
	SignerCertificate *x509.Certificate

	// SignerSubject and SignerIssuer are the certificate subject and issuer
	// distinguished names, for display.
	SignerSubject string
	SignerIssuer  string

	// NotBefore and NotAfter are the certificate validity window.
	NotBefore time.Time
	NotAfter  time.Time

	// SigningTime is the time recorded in the SignatureTime property of a
	// signed <Object>, or the zero time when absent. A SignatureTime carried by
	// an <Object> the signature does not cover is ignored.
	SigningTime time.Time

	// SignatureMethod is the algorithm URI of the SignatureValue, as declared by
	// SignedInfo (for example the URI for RSA-SHA256).
	SignatureMethod string

	// DigestMethods lists the distinct digest algorithm URIs of the references
	// that were checked, in first-seen order. A signature normally uses one
	// throughout, but nothing requires it to.
	DigestMethods []string

	// WeakAlgorithms lists the algorithm URIs among SignatureMethod and
	// DigestMethods that are no longer collision resistant (the SHA-1 and MD5
	// based methods). The verifier accepts those algorithms for compatibility
	// with older Office signatures, so Valid can be true for a SHA-1 signature:
	// a caller that wants to refuse one must test this field (or the equivalent
	// UsesWeakAlgorithms).
	WeakAlgorithms []string

	// CoveredParts lists the package part names this signature cryptographically
	// protects, in the order they appear. A part is listed only when it is
	// covered by the manifest of an <Object> whose own digest was signed — that
	// is, an Object reached by a SignedInfo reference that verified — and only
	// when SignedInfoValid is true. Manifest entries in any other <Object> are
	// not signed by the certificate holder and never appear here.
	CoveredParts []string

	// References details every reference that was checked. It is diagnostic
	// output, not a coverage statement: a reference is listed once it has been
	// evaluated, whether or not the signature as a whole is trustworthy. Use
	// CoveredParts to learn what the signature actually protects.
	References []ReferenceResult

	// Problems lists human-readable reasons the signature is not Valid.
	Problems []string
}

// UsesWeakAlgorithms reports whether the signature relies on a digest or
// signature algorithm that is no longer collision resistant (SHA-1 or MD5
// based). Such signatures still verify — the verifier accepts them so that
// older Office signatures remain inspectable — so a caller with a modern
// policy should reject them explicitly:
//
//	if !info.Valid || info.UsesWeakAlgorithms() { … }
func (i SignatureInfo) UsesWeakAlgorithms() bool { return len(i.WeakAlgorithms) > 0 }

// ReferenceResult reports the verification of a single XML-DSig reference.
type ReferenceResult struct {
	// URI is the reference URI as written in the signature.
	URI string

	// Kind is "object" (same-document Object reference), "part" (a package
	// part digest) or "relationships" (a .rels part via the Relationship
	// Transform).
	Kind string

	// Valid reports whether the recomputed digest matched.
	Valid bool

	// Detail explains a failure, or is empty on success.
	Detail string
}

// ---- Parsing structures (values only; canonical bytes come from c14n) ------

type xmlSignature struct {
	ID             string        `xml:"Id,attr"`
	SignedInfo     xmlSignedInfo `xml:"SignedInfo"`
	SignatureValue string        `xml:"SignatureValue"`
	KeyInfo        xmlKeyInfo    `xml:"KeyInfo"`
	Objects        []xmlObject   `xml:"Object"`
}

type xmlSignedInfo struct {
	CanonicalizationMethod xmlMethod      `xml:"CanonicalizationMethod"`
	SignatureMethod        xmlMethod      `xml:"SignatureMethod"`
	References             []xmlReference `xml:"Reference"`
}

type xmlMethod struct {
	Algorithm string `xml:"Algorithm,attr"`
}

type xmlReference struct {
	URI          string         `xml:"URI,attr"`
	Type         string         `xml:"Type,attr"`
	Transforms   []xmlTransform `xml:"Transforms>Transform"`
	DigestMethod xmlMethod      `xml:"DigestMethod"`
	DigestValue  string         `xml:"DigestValue"`
}

type xmlTransform struct {
	Algorithm              string                 `xml:"Algorithm,attr"`
	RelationshipReferences []xmlRelationshipRef   `xml:"RelationshipReference"`
	RelationshipGroupRefs  []xmlRelationshipGroup `xml:"RelationshipsGroupReference"`
}

type xmlRelationshipRef struct {
	SourceID string `xml:"SourceId,attr"`
}

type xmlRelationshipGroup struct {
	SourceType string `xml:"SourceType,attr"`
}

type xmlKeyInfo struct {
	X509Certificates []string `xml:"X509Data>X509Certificate"`
}

type xmlObject struct {
	ID                  string            `xml:"Id,attr"`
	Manifest            xmlManifest       `xml:"Manifest"`
	SignatureProperties xmlSignatureProps `xml:"SignatureProperties"`
}

type xmlManifest struct {
	References []xmlReference `xml:"Reference"`
}

type xmlSignatureProps struct {
	Properties []xmlSignatureProp `xml:"SignatureProperty"`
}

type xmlSignatureProp struct {
	ID            string           `xml:"Id,attr"`
	SignatureTime xmlSignatureTime `xml:"SignatureTime"`
}

type xmlSignatureTime struct {
	Format string `xml:"Format"`
	Value  string `xml:"Value"`
}

// ---- Verification ----------------------------------------------------------

// VerifySignatures locates and verifies every OPC digital signature in the
// package. It returns one SignatureInfo per signature part found; an unsigned
// package yields an empty slice and a nil error. An error is returned only for
// a structural failure that prevents inspection (e.g. a signature part that
// cannot be read); an individual signature that fails to verify is reported via
// its SignatureInfo (Valid=false, Problems populated), not as an error.
func (r *Reader) VerifySignatures() ([]SignatureInfo, error) {
	sigParts := r.signaturePartNames()
	if len(sigParts) == 0 {
		return nil, nil
	}
	infos := make([]SignatureInfo, 0, len(sigParts))
	for _, name := range sigParts {
		info, err := r.verifySignaturePart(name)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

// signaturePartNames discovers signature parts, preferring the origin
// relationship chain and falling back to scanning the _xmlsignatures folder.
func (r *Reader) signaturePartNames() []string {
	seen := make(map[string]bool)
	var names []string
	add := func(n string) {
		if n == "" || seen[n] {
			return
		}
		if r.GetFile(n) == nil {
			return
		}
		seen[n] = true
		names = append(names, n)
	}

	for _, rel := range r.GetRelationshipsByType(RelTypeDigitalSignatureOrigin) {
		origin := ResolvePartName("/", rel.Target)
		originRels, err := r.GetPartRelationships(origin)
		if err != nil {
			continue
		}
		for _, sr := range originRels {
			if sr.Type == RelTypeDigitalSignature {
				add(ResolvePartName(origin, sr.Target))
			}
		}
	}

	// Fallback: any XML part directly under /_xmlsignatures/ that is not the
	// origin part. Covers packages whose origin relationships are missing.
	for _, f := range r.Files {
		lower := strings.ToLower(f.Name)
		if strings.HasPrefix(lower, "/_xmlsignatures/") &&
			strings.HasSuffix(lower, ".xml") &&
			!strings.Contains(lower, "/_rels/") {
			add(f.Name)
		}
	}
	sort.Strings(names)
	return names
}

func (r *Reader) verifySignaturePart(partName string) (SignatureInfo, error) {
	info := SignatureInfo{PartName: partName}

	f := r.GetFile(partName)
	if f == nil {
		return info, fmt.Errorf("opc: signature part %q not found", partName)
	}
	data, err := f.ReadAll()
	if err != nil {
		return info, fmt.Errorf("opc: reading signature part %q: %w", partName, err)
	}

	var sig xmlSignature
	if err := xmlb.Unmarshal(data, &sig); err != nil {
		info.Problems = append(info.Problems, "signature XML does not parse: "+err.Error())
		return info, nil
	}
	tree, err := xmlb.ParseC14N(data)
	if err != nil {
		info.Problems = append(info.Problems, "signature XML cannot be canonicalized: "+err.Error())
		return info, nil
	}

	info.SignatureMethod = sig.SignedInfo.SignatureMethod.Algorithm
	if isWeakAlgorithm(info.SignatureMethod) {
		info.WeakAlgorithms = append(info.WeakAlgorithms, info.SignatureMethod)
	}

	// Certificate.
	cert := parseFirstCertificate(sig.KeyInfo.X509Certificates)
	if cert == nil {
		info.Problems = append(info.Problems, "no usable X.509 certificate in KeyInfo")
		return info, nil
	}
	info.SignerCertificate = cert
	info.SignerSubject = cert.Subject.String()
	info.SignerIssuer = cert.Issuer.String()
	info.NotBefore = cert.NotBefore
	info.NotAfter = cert.NotAfter

	// Cryptographic check of SignatureValue over canonicalized SignedInfo.
	siNode := tree.FindChild("SignedInfo")
	sigValue, decErr := decodeBase64(sig.SignatureValue)
	switch {
	case siNode == nil:
		info.Problems = append(info.Problems, "SignedInfo element not found")
	case decErr != nil:
		info.Problems = append(info.Problems, "SignatureValue is not valid base64: "+decErr.Error())
	default:
		canonSI := siNode.Canonical()
		if err := spinecrypto.Verify(cert.PublicKey, info.SignatureMethod, canonSI, sigValue); err != nil {
			info.Problems = append(info.Problems, "SignatureValue does not verify against the certificate: "+err.Error())
		} else {
			info.SignedInfoValid = true
		}
	}

	// SignedInfo references. Each addresses an <Object> of this signature, and a
	// reference that verifies is the *only* thing that binds an Object into the
	// signature: everything outside SignedInfo is attacker-controlled once the
	// package has left the signer's hands. signedObjects therefore records the
	// Objects whose canonical bytes were actually covered by SignatureValue, and
	// the covered part set below is derived from those Objects alone.
	owners := objectIDOwners(data)
	signedObjects := make(map[int]bool, len(sig.Objects))
	referencedIDs := make(map[string]bool, len(sig.SignedInfo.References))
	for _, ref := range sig.SignedInfo.References {
		res := r.verifyObjectReference(ref, tree)
		info.References = append(info.References, res)
		info.addDigestMethod(ref.DigestMethod.Algorithm)
		id := sameDocumentID(ref.URI)
		if id == "" {
			continue
		}
		referencedIDs[id] = true
		// owners maps an Id to the top-level <Object> that carries it, and to
		// -1 when the first element bearing that Id in document order is
		// something else. That first-in-document-order rule is exactly the one
		// C14NNode.FindByID applied when computing the digest above, so a
		// duplicate Id later in the document cannot claim a digest that was
		// computed over an earlier element.
		if idx, ok := owners[id]; ok && idx >= 0 && res.Valid {
			signedObjects[idx] = true
		}
	}

	// Manifest references inside the Objects the signature covers (parts and
	// relationships). An Object the signature does not cover contributes
	// nothing: its manifest is unsigned, so the parts it lists are not signed
	// by the certificate holder.
	forgedCoverage := false
	for i, obj := range sig.Objects {
		if !signedObjects[i] {
			if problem := unsignedObjectProblem(obj, referencedIDs); problem != "" {
				info.Problems = append(info.Problems, problem)
				forgedCoverage = true
			}
			continue
		}
		for _, ref := range obj.Manifest.References {
			res, part := r.verifyManifestReference(ref)
			info.References = append(info.References, res)
			info.addDigestMethod(ref.DigestMethod.Algorithm)
			// Coverage is a statement about what the certificate holder signed,
			// so it is claimed only when SignatureValue itself verified.
			if part != "" && info.SignedInfoValid {
				info.CoveredParts = append(info.CoveredParts, part)
			}
		}
		if t := signatureTimeOf(obj); !t.IsZero() {
			info.SigningTime = t
		}
	}

	allRefsValid := true
	for _, ref := range info.References {
		if !ref.Valid {
			allRefsValid = false
			if ref.Detail != "" {
				info.Problems = append(info.Problems, ref.Detail)
			}
		}
	}
	info.Valid = info.SignedInfoValid && allRefsValid && !forgedCoverage && len(info.References) > 0
	return info, nil
}

// addDigestMethod records a digest algorithm URI seen on a checked reference,
// keeping the list distinct and in first-seen order.
func (i *SignatureInfo) addDigestMethod(uri string) {
	if uri == "" {
		return
	}
	for _, seen := range i.DigestMethods {
		if seen == uri {
			return
		}
	}
	i.DigestMethods = append(i.DigestMethods, uri)
	if isWeakAlgorithm(uri) {
		i.WeakAlgorithms = append(i.WeakAlgorithms, uri)
	}
}

// isWeakAlgorithm reports whether an XML-DSig algorithm URI names a digest or
// signature method built on a hash that is no longer collision resistant. The
// verifier accepts these (older Office signatures use them); the point of
// naming them is to let a caller refuse them.
func isWeakAlgorithm(uri string) bool {
	lower := strings.ToLower(uri)
	return strings.HasSuffix(lower, "sha1") || strings.HasSuffix(lower, "md5")
}

// sameDocumentID returns the id a same-document reference URI ("#id")
// addresses, or "" when the URI is not of that form.
func sameDocumentID(uri string) string {
	if !strings.HasPrefix(uri, "#") {
		return ""
	}
	return uri[1:]
}

// objectIDOwners scans a signature document and maps every Id value to the
// position, among the top-level <Object> elements, of the Object that carries
// it — or to -1 when the first element bearing that Id in document order is not
// a top-level Object (the root Signature, an element nested inside an Object, a
// foreign element smuggled into KeyInfo, …).
//
// Only the first element with a given Id is recorded, which mirrors
// C14NNode.FindByID: the digest of a same-document reference is always computed
// over the first match in document order, so a later duplicate can never
// inherit the trust of an earlier element's verified digest.
//
// The positions line up with the Objects slice unmarshalled from the same
// document: both count the direct <Object> children of the root, in order.
// A truncated or malformed tail simply yields fewer entries, which is
// fail-closed — an Object whose Id is not resolved here is never treated as
// signed.
func objectIDOwners(data []byte) map[string]int {
	owners := make(map[string]int)
	dec := xmlb.NewDecoder(bytes.NewReader(data))
	depth, objects := 0, 0
	for {
		tok, err := dec.Token()
		if err != nil {
			return owners
		}
		switch t := tok.(type) {
		case stdxml.StartElement:
			idx := -1
			if depth == 1 && t.Name.Local == "Object" {
				idx = objects
				objects++
			}
			for _, a := range t.Attr {
				// Namespace declarations are not attributes for id lookup, and
				// the id may carry any prefix (xd:Id, wsu:Id, …), matching the
				// local-name test FindByID uses.
				if a.Name.Space == "xmlns" || a.Name.Local != "Id" {
					continue
				}
				if _, seen := owners[a.Value]; !seen {
					owners[a.Value] = idx
				}
			}
			depth++
		case stdxml.EndElement:
			depth--
		}
	}
}

// unsignedObjectProblem describes an <Object> that the signature does not cover
// but which claims package coverage anyway, or "" when the Object claims none.
//
// An uncovered Object with no manifest is benign and common: Office packages
// carry an unreferenced <Object> holding the XAdES QualifyingProperties (its
// SignedProperties child, not the Object, is what SignedInfo references), and
// such an Object says nothing about the package. An uncovered Object that does
// carry a manifest is the C362 forgery: it asserts that parts are signed when
// no signature covers the assertion.
func unsignedObjectProblem(obj xmlObject, referencedIDs map[string]bool) string {
	n := len(obj.Manifest.References)
	if n == 0 {
		return ""
	}
	what := "an <Object> with no Id"
	if obj.ID != "" {
		what = "<Object> " + obj.ID
	}
	claim := " claims " + itoa(n) + " manifest reference(s) but "
	if obj.ID != "" && referencedIDs[obj.ID] {
		return what + claim + "its SignedInfo reference does not cover it; its coverage claims are ignored"
	}
	return what + claim + "no SignedInfo reference covers it; its coverage claims are ignored"
}

// verifyObjectReference checks a SignedInfo reference to a same-document
// <Object> element: it canonicalizes the object and compares the digest.
func (r *Reader) verifyObjectReference(ref xmlReference, tree *xmlb.C14NNode) ReferenceResult {
	res := ReferenceResult{URI: ref.URI, Kind: "object"}
	id := sameDocumentID(ref.URI)
	if id == "" {
		res.Detail = "object reference " + ref.URI + " is not a same-document reference"
		return res
	}
	if !onlyC14NTransforms(ref.Transforms) {
		res.Detail = "object reference " + ref.URI + " uses an unsupported transform"
		return res
	}
	node := tree.FindByID(id)
	if node == nil {
		res.Detail = "object " + ref.URI + " not found in signature"
		return res
	}
	ok, detail := compareDigest(ref, node.Canonical())
	res.Valid = ok
	if !ok {
		res.Detail = "object " + ref.URI + ": " + detail
	}
	return res
}

// verifyManifestReference checks a manifest reference against the package: a
// plain part digest, or a .rels part run through the Relationship Transform.
// It returns the covered part name (empty when the reference is unusable).
func (r *Reader) verifyManifestReference(ref xmlReference) (ReferenceResult, string) {
	res := ReferenceResult{URI: ref.URI}
	partName := r.resolveManifestPartName(ref.URI)
	if partName == "" {
		res.Detail = "manifest reference " + ref.URI + " has no part name"
		return res, ""
	}

	relTransform, tail := splitRelationshipTransform(ref.Transforms)
	if relTransform != nil {
		res.Kind = "relationships"
		if !onlyC14NTransforms(tail) {
			res.Detail = "relationship reference " + ref.URI + " uses an unsupported post-transform"
			return res, partName
		}
		digest, err := r.relationshipReferenceDigest(partName, relTransform, ref.DigestMethod.Algorithm)
		if err != nil {
			res.Detail = "relationship reference " + ref.URI + ": " + err.Error()
			return res, partName
		}
		ok, detail := compareDigestValue(ref.DigestValue, digest)
		res.Valid = ok
		if !ok {
			res.Detail = "relationship reference " + ref.URI + ": " + detail
		}
		return res, partName
	}

	res.Kind = "part"
	if !onlyC14NTransforms(ref.Transforms) {
		res.Detail = "part reference " + ref.URI + " uses an unsupported transform"
		return res, partName
	}
	f := r.GetFile(partName)
	if f == nil {
		res.Detail = "signed part " + partName + " is missing from the package"
		return res, partName
	}
	partData, err := f.ReadAll()
	if err != nil {
		res.Detail = "reading signed part " + partName + ": " + err.Error()
		return res, partName
	}
	ok, detail := compareDigest(ref, partData)
	res.Valid = ok
	if !ok {
		res.Detail = "part " + partName + ": " + detail
	}
	return res, partName
}

// relationshipReferenceDigest applies the OPC Relationship Transform to a .rels
// part and returns the digest of the canonicalized result.
func (r *Reader) relationshipReferenceDigest(partName string, transform *xmlTransform, digestURI string) ([]byte, error) {
	f := r.GetFile(partName)
	if f == nil {
		return nil, fmt.Errorf("relationships part %s is missing", partName)
	}
	data, err := f.ReadAll()
	if err != nil {
		return nil, err
	}
	rels, err := UnmarshalRelationships(data)
	if err != nil {
		return nil, err
	}
	selected := selectRelationships(rels, transform)
	return relationshipsDigest(digestURI, selected)
}

// ---- Signing ---------------------------------------------------------------

// SignPackage writes a copy of the package read from src into dst, adding a
// single OPC digital signature over the requested parts. signer supplies the
// private key (RSA or ECDSA) and cert the matching X.509 certificate embedded
// in the signature. parts lists the package part names to sign (e.g.
// "/word/document.xml"); when empty, every content part in the package is
// signed. The package relationships (and each signed part's own relationships,
// when present) are always signed via the Relationship Transform.
//
// The signature always uses SHA-256 digests and an RSA-SHA256 or ECDSA-SHA256
// signature. Any pre-existing signature parts in src are dropped: SignPackage
// writes a fresh /_xmlsignatures/sig1.xml.
//
// The produced signature is a standards-compliant XML-DSig that this package's
// VerifySignatures accepts. It also includes Microsoft Office's
// application-specific signature Object (SignatureInfoV1), so Office's signature
// UI recognizes the signature; the environment fields in that Object carry
// neutral placeholder values. Interoperability with Office's UI is best-effort
// and has not been validated against Office itself.
func SignPackage(src *Reader, dst io.Writer, signer crypto.Signer, cert *x509.Certificate, parts []string) error {
	if src == nil {
		return fmt.Errorf("opc: SignPackage: nil source reader")
	}
	if signer == nil || cert == nil {
		return fmt.Errorf("opc: SignPackage: signer and certificate are required")
	}
	sigURI, digestURI, err := spinecrypto.SignatureMethodForKey(signer.Public())
	if err != nil {
		return fmt.Errorf("opc: SignPackage: %w", err)
	}

	contentParts, relsParts := src.classifySignTargets(parts)

	// Build the signature XML.
	sigXML, err := src.buildSignature(signer, cert, sigURI, digestURI, contentParts, relsParts)
	if err != nil {
		return fmt.Errorf("opc: SignPackage: %w", err)
	}

	return src.writeSignedPackage(dst, sigXML)
}

// classifySignTargets resolves the parts to sign into content parts (plain
// digest) and relationships parts (Relationship Transform). An empty parts list
// signs every part in the package.
func (r *Reader) classifySignTargets(parts []string) (contentParts, relsParts []string) {
	isRels := func(name string) bool { return strings.HasSuffix(strings.ToLower(name), ".rels") }
	isSigInfra := func(name string) bool {
		return strings.HasPrefix(strings.ToLower(name), "/_xmlsignatures/")
	}

	relsSet := make(map[string]bool)
	addRels := func(name string) {
		if name != "" && !relsSet[name] && r.GetFile(name) != nil {
			relsSet[name] = true
			relsParts = append(relsParts, name)
		}
	}

	if len(parts) == 0 {
		for _, f := range r.Files {
			if isSigInfra(f.Name) {
				continue
			}
			if isRels(f.Name) {
				addRels(f.Name)
				continue
			}
			contentParts = append(contentParts, f.Name)
		}
		return contentParts, relsParts
	}

	// Always cover the package relationships.
	addRels(GetRelationshipsPartName(""))
	for _, p := range parts {
		name := NormalizePartName(p)
		if isSigInfra(name) {
			continue
		}
		if isRels(name) {
			addRels(name)
			continue
		}
		contentParts = append(contentParts, name)
		addRels(GetRelationshipsPartName(name))
	}
	return contentParts, relsParts
}

// buildSignature assembles the XML-DSig Signature document.
func (r *Reader) buildSignature(signer crypto.Signer, cert *x509.Certificate, sigURI, digestURI string, contentParts, relsParts []string) ([]byte, error) {
	// Object: manifest of part/relationship references plus the signing time.
	var manifest strings.Builder
	for _, name := range contentParts {
		digest, err := r.partDigest(digestURI, name)
		if err != nil {
			return nil, err
		}
		ct := r.ContentTypes.GetContentType(name)
		manifest.WriteString(`<Reference URI="`)
		manifest.WriteString(xmlb.EscapeAttrValue(partReferenceURI(name, ct)))
		manifest.WriteString(`">`)
		manifest.WriteString(digestMethodAndValue(digestURI, digest))
		manifest.WriteString(`</Reference>`)
	}
	for _, name := range relsParts {
		rels, err := r.relationshipsForPart(name)
		if err != nil {
			return nil, err
		}
		selected := signableRelationships(rels)
		digest, err := relationshipsDigest(digestURI, selected)
		if err != nil {
			return nil, err
		}
		ct := r.ContentTypes.GetContentType(name)
		if ct == "" {
			ct = ContentTypeRelationships
		}
		manifest.WriteString(`<Reference URI="`)
		manifest.WriteString(xmlb.EscapeAttrValue(partReferenceURI(name, ct)))
		manifest.WriteString(`">`)
		manifest.WriteString(`<Transforms><Transform Algorithm="`)
		manifest.WriteString(xmlb.EscapeAttrValue(algRelationshipTransform))
		manifest.WriteString(`">`)
		for _, rel := range selected {
			manifest.WriteString(`<mdssi:RelationshipReference xmlns:mdssi="`)
			manifest.WriteString(xmlb.EscapeAttrValue(nsMDSSI))
			manifest.WriteString(`" SourceId="`)
			manifest.WriteString(xmlb.EscapeAttrValue(rel.ID))
			manifest.WriteString(`"/>`)
		}
		manifest.WriteString(`</Transform><Transform Algorithm="`)
		manifest.WriteString(xmlb.EscapeAttrValue(spinecrypto.AlgC14N))
		manifest.WriteString(`"/></Transforms>`)
		manifest.WriteString(digestMethodAndValue(digestURI, digest))
		manifest.WriteString(`</Reference>`)
	}

	signingTime := time.Now().UTC().Format(time.RFC3339)

	// The Object declares the dsig default namespace so it canonicalizes
	// identically whether taken standalone (here) or as a child of the
	// Signature element (during verification). mdssi is declared inline.
	var object strings.Builder
	object.WriteString(`<Object xmlns="`)
	object.WriteString(nsXMLDSig)
	object.WriteString(`" Id="`)
	object.WriteString(idPackageObject)
	object.WriteString(`"><Manifest>`)
	object.WriteString(manifest.String())
	object.WriteString(`</Manifest><SignatureProperties><SignatureProperty Id="`)
	object.WriteString(idSignatureTime)
	object.WriteString(`" Target="#`)
	object.WriteString(idPackageSignature)
	object.WriteString(`"><mdssi:SignatureTime xmlns:mdssi="`)
	object.WriteString(nsMDSSI)
	object.WriteString(`"><mdssi:Format>YYYY-MM-DDThh:mm:ssTZD</mdssi:Format><mdssi:Value>`)
	object.WriteString(xmlEscape(signingTime))
	object.WriteString(`</mdssi:Value></mdssi:SignatureTime></SignatureProperty></SignatureProperties></Object>`)

	objectXML := object.String()
	objectDigest, err := spinecrypto.Digest(digestURI, canonicalize(objectXML))
	if err != nil {
		return nil, err
	}

	// Office-specific Object: carries the SignatureInfoV1 details Microsoft
	// Office's signature UI reads so it recognizes the signature as its own. It
	// is covered by the signature via its own SignedInfo reference below.
	officeObjectXML := officeObjectXML()
	officeObjectDigest, err := spinecrypto.Digest(digestURI, canonicalize(officeObjectXML))
	if err != nil {
		return nil, err
	}

	// SignedInfo references both objects by id, each with a C14N transform.
	var signedInfo strings.Builder
	signedInfo.WriteString(`<SignedInfo xmlns="`)
	signedInfo.WriteString(nsXMLDSig)
	signedInfo.WriteString(`"><CanonicalizationMethod Algorithm="`)
	signedInfo.WriteString(xmlb.EscapeAttrValue(spinecrypto.AlgC14N))
	signedInfo.WriteString(`"/><SignatureMethod Algorithm="`)
	signedInfo.WriteString(xmlb.EscapeAttrValue(sigURI))
	signedInfo.WriteString(`"/><Reference URI="#`)
	signedInfo.WriteString(idPackageObject)
	signedInfo.WriteString(`" Type="`)
	signedInfo.WriteString(xmlb.EscapeAttrValue(typeObject))
	signedInfo.WriteString(`"><Transforms><Transform Algorithm="`)
	signedInfo.WriteString(xmlb.EscapeAttrValue(spinecrypto.AlgC14N))
	signedInfo.WriteString(`"/></Transforms>`)
	signedInfo.WriteString(digestMethodAndValue(digestURI, objectDigest))
	signedInfo.WriteString(`</Reference><Reference URI="#`)
	signedInfo.WriteString(idOfficeObject)
	signedInfo.WriteString(`" Type="`)
	signedInfo.WriteString(xmlb.EscapeAttrValue(typeObject))
	signedInfo.WriteString(`"><Transforms><Transform Algorithm="`)
	signedInfo.WriteString(xmlb.EscapeAttrValue(spinecrypto.AlgC14N))
	signedInfo.WriteString(`"/></Transforms>`)
	signedInfo.WriteString(digestMethodAndValue(digestURI, officeObjectDigest))
	signedInfo.WriteString(`</Reference></SignedInfo>`)

	signedInfoXML := signedInfo.String()
	sigValue, err := spinecrypto.Sign(signer, sigURI, canonicalize(signedInfoXML))
	if err != nil {
		return nil, err
	}

	// Assemble the full signature document. The root declares the dsig
	// namespace; SignedInfo and Object re-declare it (redundant but harmless)
	// so their standalone canonical forms above match their embedded ones.
	var doc strings.Builder
	doc.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	doc.WriteString("\r\n")
	doc.WriteString(`<Signature xmlns="`)
	doc.WriteString(nsXMLDSig)
	doc.WriteString(`" Id="`)
	doc.WriteString(idPackageSignature)
	doc.WriteString(`">`)
	doc.WriteString(signedInfoXML)
	doc.WriteString(`<SignatureValue>`)
	doc.WriteString(base64.StdEncoding.EncodeToString(sigValue))
	doc.WriteString(`</SignatureValue><KeyInfo><X509Data><X509Certificate>`)
	doc.WriteString(base64.StdEncoding.EncodeToString(cert.Raw))
	doc.WriteString(`</X509Certificate></X509Data></KeyInfo>`)
	doc.WriteString(objectXML)
	doc.WriteString(officeObjectXML)
	doc.WriteString(`</Signature>`)
	return []byte(doc.String()), nil
}

// officeObjectXML builds Microsoft Office's application-specific signature
// Object (the SignatureInfoV1 details in the office digsig namespace). Office's
// signature UI expects this Object alongside the standard package Object to
// display and trust the signature; standards-based verifiers ignore it (it is
// covered by the signature like any other Object). The environment fields carry
// neutral placeholder values — the signature semantics live in SignatureType=1
// (a package signature) and the null SignatureProviderId.
//
// Like the package Object, it declares the dsig default namespace so its
// standalone canonical form (used to compute the digest at signing time) matches
// its embedded form (recomputed at verification time).
func officeObjectXML() string {
	var b strings.Builder
	b.WriteString(`<Object xmlns="`)
	b.WriteString(nsXMLDSig)
	b.WriteString(`" Id="`)
	b.WriteString(idOfficeObject)
	b.WriteString(`"><SignatureProperties><SignatureProperty Id="`)
	b.WriteString(idOfficeV1Details)
	b.WriteString(`" Target="#`)
	b.WriteString(idPackageSignature)
	b.WriteString(`"><SignatureInfoV1 xmlns="`)
	b.WriteString(nsOfficeDigSig)
	b.WriteString(`"><SetupID></SetupID><SignatureText></SignatureText>` +
		`<SignatureImage></SignatureImage><SignatureComments></SignatureComments>` +
		`<WindowsVersion>10.0</WindowsVersion><OfficeVersion>16.0</OfficeVersion>` +
		`<ApplicationVersion>16.0</ApplicationVersion><Monitors>1</Monitors>` +
		`<HorizontalResolution>1920</HorizontalResolution><VerticalResolution>1080</VerticalResolution>` +
		`<ColorDepth>32</ColorDepth>` +
		`<SignatureProviderId>{00000000-0000-0000-0000-000000000000}</SignatureProviderId>` +
		`<SignatureProviderUrl></SignatureProviderUrl><SignatureProviderDetails>9</SignatureProviderDetails>` +
		`<SignatureType>1</SignatureType>`)
	b.WriteString(`</SignatureInfoV1></SignatureProperty></SignatureProperties></Object>`)
	return b.String()
}

// writeSignedPackage copies every part of the source package into dst, then
// adds the signature origin part, the signature part, and the origin
// relationship, regenerating the package relationships and content types.
func (r *Reader) writeSignedPackage(dst io.Writer, sigXML []byte) error {
	w := NewWriter(dst)

	// Preserve the source content types verbatim; new signature overrides are
	// merged in at Close.
	rawCT, err := r.GetRawZipFile("[Content_Types].xml")
	if err != nil {
		return err
	}
	if err := w.WriteRawFile("[Content_Types].xml", rawCT); err != nil {
		return err
	}

	// Copy all parts except the package relationships (regenerated below) and
	// any existing signature infrastructure (replaced).
	for _, f := range r.Files {
		lower := strings.ToLower(f.Name)
		if lower == "/_rels/.rels" || strings.HasPrefix(lower, "/_xmlsignatures/") {
			continue
		}
		data, err := f.ReadAll()
		if err != nil {
			return err
		}
		if err := w.WritePreservedPart(f.Name, "", data); err != nil {
			return err
		}
	}

	// Reproduce the source archive's directory entries for fidelity.
	if err := w.WriteDirectoryEntries(r.DirectoryEntries); err != nil {
		return err
	}

	// Signature origin (empty part) and its relationship to the signature.
	if err := w.WritePart(originPartName, ContentTypeDigitalSignatureOrigin, nil); err != nil {
		return err
	}
	originRels := []*Relationship{{
		ID:         "rId1",
		Type:       RelTypeDigitalSignature,
		Target:     "sig1.xml",
		TargetMode: TargetModeInternal,
	}}
	if err := w.WritePartRelationships(originPartName, originRels); err != nil {
		return err
	}

	// The signature part itself.
	if err := w.WritePart(signaturePartName, ContentTypeDigitalSignature, sigXML); err != nil {
		return err
	}

	// Regenerate the package relationships: the source set (minus any stale
	// signature-origin relationship) plus the new origin relationship.
	rels := make([]*Relationship, 0, len(r.Relationships)+1)
	for _, rel := range r.Relationships {
		if rel.Type == RelTypeDigitalSignatureOrigin {
			continue
		}
		rels = append(rels, rel)
	}
	rels = append(rels, &Relationship{
		ID:         nextRelationshipID(rels),
		Type:       RelTypeDigitalSignatureOrigin,
		Target:     "_xmlsignatures/origin.sigs",
		TargetMode: TargetModeInternal,
	})
	w.Relationships = rels

	return w.Close()
}

// ---- Shared helpers --------------------------------------------------------

// canonicalize returns the inclusive C14N of s, discarding the (impossible for
// internally generated XML) parse error.
func canonicalize(s string) []byte {
	out, err := xmlb.Canonicalize([]byte(s))
	if err != nil {
		// The input is XML this package just generated; a parse failure is a
		// programming error, not a runtime condition.
		panic("opc: internal signature XML failed to canonicalize: " + err.Error())
	}
	return out
}

// relationshipsDigest applies the Relationship Transform serialization to the
// selected relationships and returns the digest of the canonical result.
func relationshipsDigest(digestURI string, selected []*Relationship) ([]byte, error) {
	canon, err := xmlb.Canonicalize(relationshipTransformXML(selected))
	if err != nil {
		return nil, err
	}
	return spinecrypto.Digest(digestURI, canon)
}

// relationshipTransformXML serializes the selected relationships as the
// Relationship Transform prescribes: a <Relationships> element whose children
// are the selected <Relationship> elements sorted by Id, each carrying an
// explicit TargetMode. Canonicalization sorts the attributes.
func relationshipTransformXML(selected []*Relationship) []byte {
	sorted := make([]*Relationship, len(selected))
	copy(sorted, selected)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	var b strings.Builder
	b.WriteString(`<Relationships xmlns="`)
	b.WriteString(RelationshipsNamespace)
	b.WriteString(`">`)
	for _, rel := range sorted {
		tm := rel.TargetMode
		if tm == "" {
			tm = TargetModeInternal
		}
		b.WriteString(`<Relationship Id="`)
		b.WriteString(xmlb.EscapeAttrValue(rel.ID))
		b.WriteString(`" Type="`)
		b.WriteString(xmlb.EscapeAttrValue(rel.Type))
		b.WriteString(`" Target="`)
		b.WriteString(xmlb.EscapeAttrValue(rel.Target))
		b.WriteString(`" TargetMode="`)
		b.WriteString(xmlb.EscapeAttrValue(string(tm)))
		b.WriteString(`"/>`)
	}
	b.WriteString(`</Relationships>`)
	return []byte(b.String())
}

// signableRelationships returns the relationships eligible to be signed: all of
// them except any digital-signature-origin relationship (which is added by the
// signing process itself and must not invalidate the signature).
func signableRelationships(rels []*Relationship) []*Relationship {
	out := make([]*Relationship, 0, len(rels))
	for _, rel := range rels {
		if rel.Type == RelTypeDigitalSignatureOrigin {
			continue
		}
		out = append(out, rel)
	}
	return out
}

// selectRelationships resolves a Relationship Transform's SourceId and
// SourceType selectors against a .rels part's relationships.
func selectRelationships(rels []*Relationship, transform *xmlTransform) []*Relationship {
	if transform == nil {
		return signableRelationships(rels)
	}
	ids := make(map[string]bool)
	for _, ref := range transform.RelationshipReferences {
		ids[ref.SourceID] = true
	}
	types := make(map[string]bool)
	for _, g := range transform.RelationshipGroupRefs {
		types[g.SourceType] = true
	}
	if len(ids) == 0 && len(types) == 0 {
		return signableRelationships(rels)
	}
	var out []*Relationship
	for _, rel := range rels {
		if ids[rel.ID] || types[rel.Type] {
			out = append(out, rel)
		}
	}
	return out
}

func (r *Reader) relationshipsForPart(relsPartName string) ([]*Relationship, error) {
	f := r.GetFile(relsPartName)
	if f == nil {
		return nil, fmt.Errorf("relationships part %s not found", relsPartName)
	}
	data, err := f.ReadAll()
	if err != nil {
		return nil, err
	}
	return UnmarshalRelationships(data)
}

func (r *Reader) partDigest(digestURI, name string) ([]byte, error) {
	f := r.GetFile(name)
	if f == nil {
		return nil, fmt.Errorf("part %s not found", name)
	}
	data, err := f.ReadAll()
	if err != nil {
		return nil, err
	}
	return spinecrypto.Digest(digestURI, data)
}

// partReferenceURI builds the content-type-qualified reference URI OPC uses to
// address a package part inside a signature manifest.
//
// A part name that conforms to the OPC grammar (ECMA-376 Part 2 §9.1.1) is
// already a URI path — the grammar admits only pchar and percent-encoded octets
// — so it is emitted verbatim, which is both what Office writes and what a
// third-party verifier resolves back to the same part. Names that only wild
// packages carry (a space, a bare '%', non-ASCII bytes: all illegal in a part
// name, all preserved verbatim by this library) are percent-encoded so the
// reference stays a well-formed URI; verification undoes that (see
// resolveManifestPartName). The content type is escaped for the query
// component, so a type carrying '&' or a space cannot break the query apart for
// a verifier that parses it.
func partReferenceURI(partName, contentType string) string {
	uri := escapeURIPath(partName)
	if contentType == "" {
		return uri
	}
	return uri + "?ContentType=" + escapeURIQueryValue(contentType)
}

// escapeURIPath percent-encodes a part name for use as a URI path, leaving
// every character the OPC part-name grammar allows untouched (so a conformant
// part name is returned unchanged, percent-escapes included).
func escapeURIPath(partName string) string {
	if !needsURIPathEscape(partName) {
		return partName
	}
	return (&url.URL{Path: partName}).EscapedPath()
}

// needsURIPathEscape reports whether partName contains anything the part-name
// grammar forbids and that therefore has to be percent-encoded to appear in a
// URI. A conformant name (pchar plus well-formed percent-escapes) needs none.
func needsURIPathEscape(partName string) bool {
	for i := 0; i < len(partName); i++ {
		c := partName[i]
		switch {
		case c == '/' || isPartNameChar(c):
		case c == '%':
			if i+2 >= len(partName) || !isHexDigit(partName[i+1]) || !isHexDigit(partName[i+2]) {
				return true
			}
			i += 2
		default:
			return true
		}
	}
	return false
}

// escapeURIQueryValue percent-encodes s for use as a value in a reference URI's
// query. Characters of RFC 3986's query production are kept literal — so the
// usual "?ContentType=application/…+xml" is unchanged — except '&', which would
// otherwise let a content type introduce a second query parameter.
func escapeURIQueryValue(s string) string {
	escape := false
	for i := 0; i < len(s); i++ {
		if !isURIQueryChar(s[i]) {
			escape = true
			break
		}
	}
	if !escape {
		return s
	}
	const hexDigits = "0123456789ABCDEF"
	var b strings.Builder
	b.Grow(len(s) + 8)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isURIQueryChar(c) {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hexDigits[c>>4])
		b.WriteByte(hexDigits[c&0x0f])
	}
	return b.String()
}

// isURIQueryChar reports whether c may appear literally in the query component
// of a reference URI: RFC 3986's query production (pchar / "/" / "?") minus
// '&' and '?', which are reserved here for splitting the query itself.
func isURIQueryChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	case c == '-' || c == '.' || c == '_' || c == '~':
		return true
	case c == '!' || c == '$' || c == '\'' || c == '(' || c == ')' ||
		c == '*' || c == '+' || c == ',' || c == ';' || c == '=':
		return true
	case c == ':' || c == '@' || c == '/':
		return true
	}
	return false
}

// resolveManifestPartName maps a manifest reference URI to the part of this
// package it addresses. A conformant reference URI *is* the part name, so the
// literal form is tried first; only when the package has no such part is the
// percent-decoded form tried, which is what a producer that had to escape a
// grammar-illegal part name wrote. When neither exists the literal form is
// returned, so the failure is reported under the name the signature actually
// carries.
func (r *Reader) resolveManifestPartName(uri string) string {
	literal := literalPartNameFromURI(uri)
	if literal == "" {
		return ""
	}
	if r.GetFile(literal) != nil {
		return literal
	}
	if decoded := partNameFromURI(uri); decoded != "" && decoded != literal && r.GetFile(decoded) != nil {
		return decoded
	}
	return literal
}

// literalPartNameFromURI strips the ?ContentType= query and any fragment from a
// manifest reference URI and normalizes what is left, without percent-decoding:
// the result is the part name a conformant producer wrote.
func literalPartNameFromURI(uri string) string {
	if i := strings.IndexByte(uri, '?'); i >= 0 {
		uri = uri[:i]
	}
	if i := strings.IndexByte(uri, '#'); i >= 0 {
		uri = uri[:i]
	}
	if uri == "" {
		return ""
	}
	if !strings.HasPrefix(uri, "/") {
		uri = "/" + uri
	}
	return NormalizePartName(uri)
}

// partNameFromURI extracts and normalizes the package part name from a manifest
// reference URI, discarding the ?ContentType= query and percent-decoding what
// remains.
func partNameFromURI(uri string) string {
	if i := strings.IndexByte(uri, '?'); i >= 0 {
		uri = uri[:i]
	}
	if uri == "" {
		return ""
	}
	return ResolvePartName("/", uri)
}

func digestMethodAndValue(digestURI string, digest []byte) string {
	return `<DigestMethod Algorithm="` + xmlb.EscapeAttrValue(digestURI) + `"/><DigestValue>` +
		base64.StdEncoding.EncodeToString(digest) + `</DigestValue>`
}

// onlyC14NTransforms reports whether every transform is inclusive C14N (or the
// list is empty). Object and part references may carry a C14N transform or none.
func onlyC14NTransforms(transforms []xmlTransform) bool {
	for _, t := range transforms {
		if t.Algorithm != spinecrypto.AlgC14N {
			return false
		}
	}
	return true
}

// splitRelationshipTransform returns the Relationship Transform (if present) and
// the remaining transforms that follow it.
func splitRelationshipTransform(transforms []xmlTransform) (*xmlTransform, []xmlTransform) {
	for i := range transforms {
		if transforms[i].Algorithm == algRelationshipTransform {
			t := transforms[i]
			return &t, transforms[i+1:]
		}
	}
	return nil, transforms
}

func compareDigest(ref xmlReference, data []byte) (bool, string) {
	digest, err := spinecrypto.Digest(ref.DigestMethod.Algorithm, data)
	if err != nil {
		return false, err.Error()
	}
	return compareDigestValue(ref.DigestValue, digest)
}

func compareDigestValue(want string, got []byte) (bool, string) {
	wantBytes, err := decodeBase64(want)
	if err != nil {
		return false, "stored digest is not valid base64: " + err.Error()
	}
	if len(wantBytes) != len(got) {
		return false, "digest mismatch"
	}
	var diff byte
	for i := range got {
		diff |= got[i] ^ wantBytes[i]
	}
	if diff != 0 {
		return false, "digest mismatch"
	}
	return true, ""
}

func parseFirstCertificate(b64Certs []string) *x509.Certificate {
	for _, raw := range b64Certs {
		der, err := decodeBase64(raw)
		if err != nil {
			continue
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			continue
		}
		return cert
	}
	return nil
}

func signatureTimeOf(obj xmlObject) time.Time {
	for _, p := range obj.SignatureProperties.Properties {
		if v := strings.TrimSpace(p.SignatureTime.Value); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

// decodeBase64 decodes standard base64 after stripping ASCII whitespace, which
// signature producers freely insert into long base64 values.
func decodeBase64(s string) ([]byte, error) {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '\r', '\n':
			continue
		default:
			b.WriteByte(s[i])
		}
	}
	return base64.StdEncoding.DecodeString(b.String())
}

// nextRelationshipID returns an rId not already used by rels.
func nextRelationshipID(rels []*Relationship) string {
	used := make(map[string]bool, len(rels))
	for _, rel := range rels {
		used[rel.ID] = true
	}
	for i := 1; ; i++ {
		id := fmt.Sprintf("rId%d", i)
		if !used[id] {
			return id
		}
	}
}
