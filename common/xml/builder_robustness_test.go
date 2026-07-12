package xml

import (
	"strings"
	"testing"
)

// C105: a mandatory (non-omitempty) zero-valued child must be written, not
// dropped by the parent self-closing.
func TestMarshal_MandatoryZeroChildNotDropped(t *testing.T) {
	type inner struct {
		V int64 `xml:"http://schemas.openxmlformats.org/drawingml/2006/main v"`
	}
	type outer struct {
		Inner inner `xml:"http://schemas.openxmlformats.org/drawingml/2006/main inner"`
	}

	b := NewPresentationMLBuilder()
	b.MarshalElement(NSDrawingML, "outer", &outer{Inner: inner{V: 0}})
	out := b.String()

	if strings.Contains(out, "<a:outer/>") || strings.Contains(out, "<a:outer />") {
		t.Fatalf("parent self-closed, dropping mandatory child: %s", out)
	}
	if !strings.Contains(out, "<a:v>0</a:v>") {
		t.Errorf("mandatory zero-valued child <a:v>0</a:v> missing: %s", out)
	}
}

// C104: the Builder reports unbalanced and mismatched elements.
func TestBuilder_FinishBalanced(t *testing.T) {
	b := NewBuilder()
	b.StartElement("ns", "root")
	b.EndElement("ns", "root")
	if err := b.Finish(); err != nil {
		t.Errorf("balanced builder reported error: %v", err)
	}
}

func TestBuilder_FinishUnclosed(t *testing.T) {
	b := NewBuilder()
	b.StartElement("ns", "root")
	b.StartElement("ns", "child")
	b.EndElement("ns", "child")
	if err := b.Finish(); err == nil {
		t.Error("expected an unclosed-element error, got nil")
	}
}

func TestBuilder_MismatchedClose(t *testing.T) {
	b := NewBuilder()
	b.StartElement("ns", "a")
	b.EndElement("ns", "b") // wrong name
	if b.Err() == nil {
		t.Error("expected a mismatched-close error, got nil")
	}
}

// C214: the balance check must compare the full prefixed name, not just the
// local name — <p:sp></a:sp> is mismatched even though both are "sp".
func TestBuilder_MismatchedPrefix(t *testing.T) {
	b := NewPresentationMLBuilder()
	b.StartElement(NSPresentationML, "sp") // <p:sp>
	b.EndElement(NSDrawingML, "sp")        // </a:sp>
	if b.Err() == nil {
		t.Errorf("closing </a:sp> against open <p:sp> not detected: %q", b.String())
	}
}

func TestBuilder_MatchingPrefixBalanced(t *testing.T) {
	b := NewPresentationMLBuilder()
	b.StartElement(NSPresentationML, "sp")
	b.EndElement(NSPresentationML, "sp")
	if err := b.Finish(); err != nil {
		t.Errorf("balanced prefixed element reported error: %v", err)
	}
}

// C214: EndElementInlineNS takes the prefix directly and must be checked too.
func TestBuilder_MismatchedInlineNSPrefix(t *testing.T) {
	b := NewBuilder()
	b.StartElementInlineNS("http://example.com/p14", "p14", "ext")
	b.EndElementInlineNS("p15", "ext") // wrong prefix
	if b.Err() == nil {
		t.Errorf("closing </p15:ext> against open <p14:ext> not detected: %q", b.String())
	}
}

func TestBuilder_MatchingInlineNSBalanced(t *testing.T) {
	b := NewBuilder()
	b.StartElementInlineNS("http://example.com/p14", "p14", "ext")
	b.EndElementInlineNS("p14", "ext")
	if err := b.Finish(); err != nil {
		t.Errorf("balanced inline-NS element reported error: %v", err)
	}
}
