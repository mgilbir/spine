package oxml

import (
	"encoding/xml"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

func marshalAC(ac *AlternateContent) string {
	b := xmlb.NewPresentationMLBuilder()
	ac.MarshalToBuilder(b, xmlb.NSMarkupCompatibility, "AlternateContent")
	return b.String()
}

// C41: multiple mc:Choice branches are preserved (previously collapsed to the last).
func TestAlternateContent_MultipleChoices(t *testing.T) {
	src := `<AlternateContent xmlns="http://schemas.openxmlformats.org/markup-compatibility/2006">` +
		`<Choice Requires="p14"><p14:one/></Choice>` +
		`<Choice Requires="p15"><p15:two/></Choice>` +
		`<Fallback/>` +
		`</AlternateContent>`

	var ac AlternateContent
	if err := xml.Unmarshal([]byte(src), &ac); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(ac.Choices) != 2 {
		t.Fatalf("got %d choices, want 2", len(ac.Choices))
	}
	if !ac.HasFallback {
		t.Error("empty Fallback was dropped (HasFallback false)")
	}

	out := marshalAC(&ac)
	if !strings.Contains(out, `Requires="p14"`) || !strings.Contains(out, `Requires="p15"`) {
		t.Errorf("a Choice was lost on marshal:\n%s", out)
	}
	if !strings.Contains(out, "Fallback") {
		t.Errorf("empty Fallback not re-emitted:\n%s", out)
	}
}

// C41: a Requires listing several prefixes declares each namespace.
func TestAlternateContent_MultiPrefixRequires(t *testing.T) {
	src := `<AlternateContent xmlns="http://schemas.openxmlformats.org/markup-compatibility/2006">` +
		`<Choice Requires="p14 p15"><p14:x/></Choice>` +
		`</AlternateContent>`

	var ac AlternateContent
	if err := xml.Unmarshal([]byte(src), &ac); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out := marshalAC(&ac)
	if !strings.Contains(out, "xmlns:p14=") || !strings.Contains(out, "xmlns:p15=") {
		t.Errorf("multi-prefix Requires left a prefix undeclared:\n%s", out)
	}
}
