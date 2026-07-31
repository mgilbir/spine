package xml

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Canonicalization checked against an independent implementation.
//
// The output of this file decides what a signature covers: the verifier
// canonicalizes a part, digests the result, and compares that to what the
// signature says. A canonical form that omits something is a signature that
// does not cover it — an attacker may change what was omitted and the digest
// still matches — and one that renders something differently from everyone else
// is a signature no other implementation can verify. Neither failure shows up
// in a round trip through this library, where both sides share the mistake.
//
// So the oracle is libxml2, through xmllint or the same engine behind Python's
// lxml. It found two defects when it was first run: processing instructions
// were dropped from the node-set, and attribute values were not normalized per
// XML 1.0 §3.3.3.
//
// Unlike the schema suites, this needs no copyrighted data — only the tool — so
// CI installs it and SPINE_REQUIRE_C14N makes a missing one a failure.

// referenceC14N runs the reference canonicalizer, or reports that none is
// installed.
func referenceC14N(t *testing.T, doc string) (string, bool) {
	t.Helper()
	if path, err := exec.LookPath("xmllint"); err == nil {
		cmd := exec.Command(path, "--nonet", "--c14n", "-")
		cmd.Stdin = strings.NewReader(doc)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("xmllint --c14n failed on %q: %v", doc, err)
		}
		return string(out), true
	}
	if path, err := exec.LookPath("python3"); err == nil {
		const script = `
import sys
from lxml import etree
doc = etree.parse(sys.stdin.buffer)
sys.stdout.buffer.write(etree.tostring(doc, method="c14n", exclusive=False, with_comments=False))
`
		cmd := exec.Command(path, "-c", script)
		cmd.Stdin = strings.NewReader(doc)
		if out, err := cmd.Output(); err == nil {
			return string(out), true
		}
	}
	return "", false
}

func TestCanonicalizeMatchesLibxml2(t *testing.T) {
	// Each case is a shape where implementations are known to diverge, so a
	// passing run means agreement on the parts that are easy to get wrong.
	docs := map[string]string{
		"attribute order":            `<r z="1" a="2" xmlns:b="urn:b" b:m="3"><c/></r>`,
		"attributes sort by uri":     `<r xmlns:p="urn:p" xmlns:q="urn:a" q:z="1" p:a="2" m="3"/>`,
		"default namespace":          `<r xmlns="urn:d"><c xmlns="urn:d"><g/></c></r>`,
		"default namespace undone":   `<r xmlns="urn:d"><c xmlns=""><g/></c></r>`,
		"redundant declaration":      `<r xmlns:p="urn:p"><p:c xmlns:p="urn:p"/></r>`,
		"unused declaration dropped": `<r xmlns:unused="urn:u"><c/></r>`,
		"xml:lang inheritance":       `<r xml:lang="en"><c><d/></c></r>`,
		"xml:space":                  `<r xml:space="preserve"><c>  spaced  </c></r>`,
		"entities in text and attrs": `<r a="&lt;&amp;&gt;&quot;">t &lt; &amp; &gt;</r>`,
		"CRLF in text":               "<r>line1\r\nline2</r>",
		"empty elements":             `<r><e/><e2></e2></r>`,
		"processing instruction":     `<r><?pi data?><x/></r>`,
		"pi with no data":            `<r><?bare?><x/></r>`,
		"pi in the prolog":           `<?first go?><r><x/></r>`,
		"comment dropped":            `<r><!--gone--><x/></r>`,
		"literal tab in attribute":   "<r a=\"x\ty\"/>",
		"literal newline in attr":    "<r a=\"x\ny\"/>",
		"escaped tab in attribute":   `<r a="x&#x9;y"/>`,
		"mixed literal and escaped":  "<r a=\"&#x9;x\ty\"/>",
		"nested same prefix rebound": `<r xmlns:p="urn:1"><p:a><p:b xmlns:p="urn:2"><p:c/></p:b></p:a></r>`,
		"text with CR reference":     `<r>a&#xD;b</r>`,
		"attribute with newline ref": `<r a="x&#xA;y"/>`,
	}

	checked := 0
	for name, doc := range docs {
		want, ok := referenceC14N(t, doc)
		if !ok {
			if os.Getenv("SPINE_REQUIRE_C14N") != "" {
				t.Fatal("SPINE_REQUIRE_C14N is set but no reference canonicalizer is installed " +
					"(expected xmllint from libxml2-utils, or python3-lxml)")
			}
			t.Skip("no reference canonicalizer installed: `apt-get install libxml2-utils` (xmllint), " +
				"or python3-lxml. CI installs xmllint and sets SPINE_REQUIRE_C14N.")
		}
		got, err := Canonicalize([]byte(doc))
		if err != nil {
			t.Errorf("%s: Canonicalize(%q): %v", name, doc, err)
			continue
		}
		checked++
		if string(got) != want {
			t.Errorf("%s: canonical form differs from libxml2\n\tinput: %q\n\tours:  %q\n\tref:   %q",
				name, doc, string(got), want)
		}
	}
	if checked == 0 {
		t.Error("no case was compared, so this proves nothing")
	}
}

// The navigation accessors a signature builder reaches for. They were never
// executed by a test, which for a two-line accessor is not alarming on its own
// — but they are the API through which a caller decides *which* element it is
// about to digest, and a wrong answer there is a signature over the wrong node.
func TestC14NNodeAccessors(t *testing.T) {
	const doc = `<sig:Signature xmlns:sig="urn:sig" xmlns="urn:plain">` +
		`<sig:SignedInfo><Reference URI="#x"/></sig:SignedInfo></sig:Signature>`

	root, err := ParseC14N([]byte(doc))
	if err != nil {
		t.Fatalf("ParseC14N: %v", err)
	}
	if got := root.LocalName(); got != "Signature" {
		t.Errorf("LocalName() = %q, want Signature", got)
	}
	if got := root.NamespaceURI(); got != "urn:sig" {
		t.Errorf("NamespaceURI() = %q, want urn:sig", got)
	}

	signed := root.FindChild("SignedInfo")
	if signed == nil {
		t.Fatal("FindChild(SignedInfo) found nothing")
	}
	if got := signed.NamespaceURI(); got != "urn:sig" {
		t.Errorf("SignedInfo NamespaceURI() = %q, want urn:sig", got)
	}

	// A child in the default namespace must report that namespace, not its
	// parent's: the two differ here precisely so a confusion between them is
	// visible.
	ref := signed.FindChild("Reference")
	if ref == nil {
		t.Fatal("FindChild(Reference) found nothing")
	}
	if got := ref.NamespaceURI(); got != "urn:plain" {
		t.Errorf("Reference NamespaceURI() = %q, want urn:plain", got)
	}
	if got := ref.LocalName(); got != "Reference" {
		t.Errorf("Reference LocalName() = %q, want Reference", got)
	}
	if missing := root.FindChild("NoSuchChild"); missing != nil {
		t.Errorf("FindChild(NoSuchChild) = %v, want nil", missing)
	}
}
