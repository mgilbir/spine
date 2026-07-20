package opc

import (
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
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

	// SigningTime is the time recorded in the signature's SignatureTime
	// property, or the zero time when absent.
	SigningTime time.Time

	// SignatureMethod and DigestMethod are the algorithm URIs used.
	SignatureMethod string

	// CoveredParts lists the package part names the signature's manifest
	// covers, in the order they appear.
	CoveredParts []string

	// References details every reference that was checked.
	References []ReferenceResult

	// Problems lists human-readable reasons the signature is not Valid.
	Problems []string
}

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

	// SignedInfo references (each addresses an <Object> in this signature).
	for _, ref := range sig.SignedInfo.References {
		res := r.verifyObjectReference(ref, tree)
		info.References = append(info.References, res)
	}

	// Manifest references inside each Object (parts and relationships).
	for _, obj := range sig.Objects {
		for _, ref := range obj.Manifest.References {
			res, part := r.verifyManifestReference(ref)
			info.References = append(info.References, res)
			if part != "" {
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
	info.Valid = info.SignedInfoValid && allRefsValid && len(info.References) > 0
	return info, nil
}

// verifyObjectReference checks a SignedInfo reference to a same-document
// <Object> element: it canonicalizes the object and compares the digest.
func (r *Reader) verifyObjectReference(ref xmlReference, tree *xmlb.C14NNode) ReferenceResult {
	res := ReferenceResult{URI: ref.URI, Kind: "object"}
	id := strings.TrimPrefix(ref.URI, "#")
	if id == "" || !strings.HasPrefix(ref.URI, "#") {
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
	partName := partNameFromURI(ref.URI)
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
func partReferenceURI(partName, contentType string) string {
	if contentType == "" {
		return partName
	}
	return partName + "?ContentType=" + contentType
}

// partNameFromURI extracts and normalizes the package part name from a manifest
// reference URI, discarding the ?ContentType= query.
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
