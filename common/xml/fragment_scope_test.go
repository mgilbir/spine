package xml

import "testing"

// A fragment's prefixes may be bound by declarations that stayed on an ancestor,
// so the in-scope set names them. What it must not do is stop checking: a prefix
// that is neither declared in the fragment nor in scope is still unbound, and
// that is the case the whole rule exists for.
func TestFragmentScopeStillRejectsAnUnboundPrefix(t *testing.T) {
	const frag = `<pivotCaches><pivotCache cacheId="1" r:id="rId5"/></pivotCaches>`

	if err := CheckNamespacePrefixes([]byte(frag)); err == nil {
		t.Fatal("the strict check should reject a fragment whose r: is undeclared")
	}
	if err := CheckNamespacePrefixesInScope([]byte(frag), []string{"r"}); err != nil {
		t.Errorf("r is in scope, so the fragment should pass: %v", err)
	}
	// The point of the test: a different prefix in scope does not excuse r.
	if err := CheckNamespacePrefixesInScope([]byte(frag), []string{"x"}); err == nil {
		t.Error("an unrelated in-scope prefix must not excuse an unbound r:")
	}
	// Nor does an empty set.
	if err := CheckNamespacePrefixesInScope([]byte(frag), nil); err == nil {
		t.Error("an empty in-scope set must behave exactly like the strict check")
	}
}

// A fragment that declares its own prefix needs no help, and the in-scope set
// must not be required for it.
func TestFragmentDeclaringItsOwnPrefixPasses(t *testing.T) {
	const frag = `<a:x xmlns:a="urn:example"><a:y a:k="v"/></a:x>`
	if err := CheckNamespacePrefixesInScope([]byte(frag), nil); err != nil {
		t.Errorf("a self-contained fragment should pass with no in-scope set: %v", err)
	}
}

// UnmarshalFragment keeps the other half of the part-level contract: content
// after the fragment's root element is still refused.
func TestUnmarshalFragmentStillRequiresACleanEnd(t *testing.T) {
	var v struct{}
	err := UnmarshalFragment([]byte(`<a:x xmlns:a="urn:example"/><trailing/>`), nil, &v)
	if err == nil {
		t.Error("content after the fragment's root element should be refused")
	}
}
