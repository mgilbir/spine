package opc

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// pyVerifierScript independently verifies an OPC XML-DSig using lxml (for
// inclusive Canonical XML 1.0) and cryptography (for RSA-SHA256), neither of
// which shares code with this package. It checks the SignatureValue over the
// canonicalized SignedInfo and recomputes every same-document Object digest —
// including Microsoft's Office-specific idOfficeObject — so a passing run means
// an outside toolchain agrees the signature and its object digests are correct.
const pyVerifierScript = `
import sys, base64, hashlib
from lxml import etree
from cryptography import x509
from cryptography.hazmat.primitives.asymmetric import padding
from cryptography.hazmat.primitives import hashes

DS = 'http://www.w3.org/2000/09/xmldsig#'
def q(*t): return '/'.join('{%s}%s' % (DS, x) for x in t)

# Inclusive C14N of a subtree. lxml's tostring(method='c14n') on an element that
# is still attached to a larger tree spuriously emits xmlns="" on descendants
# that inherit a default namespace; reparsing the element standalone (each of
# ours redeclares its own default namespace, so it is self-contained) yields the
# spec-correct canonical form. This matches what a full XML-DSig verifier
# (e.g. xmlsec) computes over the in-document subtree.
def c14n(el):
    return etree.tostring(etree.fromstring(etree.tostring(el)),
                          method='c14n', exclusive=False, with_comments=False)

root = etree.parse(sys.argv[1]).getroot()
si = root.find(q('SignedInfo'))
signed = c14n(si)
sigval = base64.b64decode(root.findtext(q('SignatureValue')))
certder = base64.b64decode(root.findtext(q('KeyInfo', 'X509Data', 'X509Certificate')))
pub = x509.load_der_x509_certificate(certder).public_key()
pub.verify(sigval, signed, padding.PKCS1v15(), hashes.SHA256())
print('SIGNEDINFO_OK')

objs = {o.get('Id'): o for o in root.findall(q('Object'))}
seen = set()
for ref in si.findall(q('Reference')):
    uri = ref.get('URI') or ''
    if not uri.startswith('#'):
        continue
    oid = uri[1:]
    got = hashlib.sha256(c14n(objs[oid])).digest()
    want = base64.b64decode(ref.findtext(q('DigestValue')))
    assert got == want, 'object digest mismatch for ' + oid
    seen.add(oid)
    print('OBJDIGEST_OK', oid)

assert 'idOfficeObject' in seen, 'Office signature object not referenced/covered'
print('OFFICE_OBJECT_OK')
`

// findSignatureVerifierPython returns a python interpreter that can import lxml
// and cryptography, or skips the test. It honors $SPINE_PYTHON.
func findSignatureVerifierPython(t *testing.T) string {
	t.Helper()
	candidates := []string{os.Getenv("SPINE_PYTHON"), "python3", "python"}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		p, err := exec.LookPath(c)
		if err != nil {
			continue
		}
		probe := exec.Command(p, "-c", "import lxml.etree, cryptography")
		if probe.Run() == nil {
			return p
		}
	}
	t.Skip("no python with lxml+cryptography found; set SPINE_PYTHON to cross-validate signatures")
	return ""
}

// TestSignatureCrossValidateWithLxml signs a package and confirms an independent
// lxml/cryptography toolchain verifies the SignedInfo signature and every Object
// digest, including the Office-specific object.
func TestSignatureCrossValidateWithLxml(t *testing.T) {
	py := findSignatureVerifierPython(t)

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
	f := r.GetFile(signaturePartName)
	if f == nil {
		t.Fatalf("signature part %s missing", signaturePartName)
	}
	sigXML, err := f.ReadAll()
	if err != nil {
		t.Fatalf("reading signature part: %v", err)
	}

	dir := t.TempDir()
	sigPath := filepath.Join(dir, "sig1.xml")
	scriptPath := filepath.Join(dir, "verify.py")
	if err := os.WriteFile(sigPath, sigXML, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scriptPath, []byte(pyVerifierScript), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command(py, scriptPath, sigPath).CombinedOutput()
	if err != nil {
		t.Fatalf("external signature verification failed: %v\n%s", err, out)
	}
	got := string(out)
	for _, want := range []string{"SIGNEDINFO_OK", "OBJDIGEST_OK idPackageObject", "OBJDIGEST_OK idOfficeObject", "OFFICE_OBJECT_OK"} {
		if !strings.Contains(got, want) {
			t.Fatalf("external verifier did not report %q; output:\n%s", want, got)
		}
	}
}
