package xml

import "testing"

// The expected outputs for whole-document cases were produced by lxml's
// Canonical XML 1.0 serializer (etree.tostring(method="c14n")), the de-facto
// reference implementation. The subset cases are hand-derived from the C14N
// spec: lxml 6.x mis-serializes a document subset when a default namespace is
// in scope at depth ≥ 2 (it emits a spurious xmlns="" on the grandchild), so
// its subset output is not a trustworthy oracle there.
func TestCanonicalizeWholeDocument(t *testing.T) {
	cases := []struct {
		name string
		in   string
		exp  string
	}{
		{"simple", `<a><b>text</b></a>`, `<a><b>text</b></a>`},
		{"attrs_sorted", `<a z="1" a="2" xmlns:x="urn:x" x:m="3"><b/></a>`, `<a xmlns:x="urn:x" a="2" z="1" x:m="3"><b></b></a>`},
		{"empty_elem", `<a><b/></a>`, `<a><b></b></a>`},
		{"ws_mixed", "<a>\n  <b>t</b>\n</a>", "<a>\n  <b>t</b>\n</a>"},
		{"entities_text", `<a>&lt;&amp;&gt;"'</a>`, `<a>&lt;&amp;&gt;"'</a>`},
		{"attr_esc", `<a v="&lt;&amp;&quot;&#9;&#10;&#13;"/>`, `<a v="&lt;&amp;&quot;&#x9;&#xA;&#xD;"></a>`},
		{"ns_redecl", `<a xmlns="urn:1"><b xmlns="urn:1"><c/></b></a>`, `<a xmlns="urn:1"><b><c></c></b></a>`},
		{"text_gt", `<a>1 &gt; 0 &amp; 2 &lt; 3</a>`, `<a>1 &gt; 0 &amp; 2 &lt; 3</a>`},
		{"nested_ns", `<r xmlns="urn:d" xmlns:m="urn:m"><m:x a="1"><y/></m:x></r>`, `<r xmlns="urn:d" xmlns:m="urn:m"><m:x a="1"><y></y></m:x></r>`},
		{"multi_attr_ns", `<a xmlns:p="urn:p" xmlns:q="urn:q" p:b="1" q:a="2" c="3"/>`, `<a xmlns:p="urn:p" xmlns:q="urn:q" c="3" p:b="1" q:a="2"></a>`},
		{"default_undecl", `<a xmlns="urn:1"><b xmlns=""><c/></b></a>`, `<a xmlns="urn:1"><b xmlns=""><c></c></b></a>`},
		{"cdata", `<a><![CDATA[ <hi> & ]]></a>`, `<a> &lt;hi&gt; &amp; </a>`},
		{"comment_stripped", `<a><!-- c --><b/></a>`, `<a><b></b></a>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Canonicalize([]byte(tc.in))
			if err != nil {
				t.Fatalf("Canonicalize: %v", err)
			}
			if string(got) != tc.exp {
				t.Errorf("Canonicalize(%q):\n got %q\nwant %q", tc.in, got, tc.exp)
			}
		})
	}
}

func TestCanonicalizeSubset(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		child string
		exp   string
	}{
		{
			"signedinfo",
			`<Signature xmlns="urn:d" xmlns:m="urn:m"><SignedInfo Id="x"><Ref m:t="1"/></SignedInfo><Val/></Signature>`,
			"SignedInfo",
			`<SignedInfo xmlns="urn:d" xmlns:m="urn:m" Id="x"><Ref m:t="1"></Ref></SignedInfo>`,
		},
		{
			// Depth-2 default namespace: Reference stays in urn:d with no
			// redeclaration (lxml's subset serializer gets this wrong).
			"object",
			`<Signature xmlns="urn:d"><Object Id="o"><Manifest><Reference URI="/a">d</Reference></Manifest></Object></Signature>`,
			"Object",
			`<Object xmlns="urn:d" Id="o"><Manifest><Reference URI="/a">d</Reference></Manifest></Object>`,
		},
		{
			// Unused ancestor namespace is rendered on the apex (inclusive C14N).
			"unused_anc",
			`<r xmlns:un="urn:un" xmlns="urn:d"><c><d/></c></r>`,
			"c",
			`<c xmlns="urn:d" xmlns:un="urn:un"><d></d></c>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, err := ParseC14N([]byte(tc.in))
			if err != nil {
				t.Fatalf("ParseC14N: %v", err)
			}
			node := root.FindChild(tc.child)
			if node == nil {
				t.Fatalf("child %q not found", tc.child)
			}
			if got := string(node.Canonical()); got != tc.exp {
				t.Errorf("subset Canonical:\n got %q\nwant %q", got, tc.exp)
			}
		})
	}
}

func TestFindByID(t *testing.T) {
	in := `<Signature xmlns="urn:d"><Object Id="a"><X/></Object><Object Id="b"><Y/></Object></Signature>`
	root, err := ParseC14N([]byte(in))
	if err != nil {
		t.Fatalf("ParseC14N: %v", err)
	}
	n := root.FindByID("b")
	if n == nil {
		t.Fatal("FindByID(b) returned nil")
	}
	if got := string(n.Canonical()); got != `<Object xmlns="urn:d" Id="b"><Y></Y></Object>` {
		t.Errorf("unexpected canonical: %q", got)
	}
	if root.FindByID("missing") != nil {
		t.Error("FindByID(missing) should be nil")
	}
}

// TestCanonicalizeMultipleRoots confirms a document with more than one
// top-level element is rejected with an error rather than panicking. Go's
// tokenizer tolerates trailing sibling roots, so a crafted signature part such
// as "<Signature>…</Signature><x/>" reaches the tree builder with an empty
// element stack; without the guard it indexes stack[-1] and panics.
func TestCanonicalizeMultipleRoots(t *testing.T) {
	for _, in := range []string{
		`<a/><b/>`,
		`<a></a><b></b>`,
		`<Signature xmlns="urn:d"><SignedInfo/></Signature><x/>`,
		`<a/><b/><c/>`,
	} {
		if _, err := Canonicalize([]byte(in)); err == nil {
			t.Errorf("Canonicalize(%q): expected error, got nil", in)
		}
		if _, err := ParseC14N([]byte(in)); err == nil {
			t.Errorf("ParseC14N(%q): expected error, got nil", in)
		}
	}

	// A well-formed single-root document still canonicalizes unchanged.
	if got, err := Canonicalize([]byte(`<a><b>t</b></a>`)); err != nil {
		t.Fatalf("single-root Canonicalize: %v", err)
	} else if string(got) != `<a><b>t</b></a>` {
		t.Errorf("single-root canonical = %q", got)
	}
}
