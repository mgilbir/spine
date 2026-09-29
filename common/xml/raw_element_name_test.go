package xml

import (
	"errors"
	"testing"
)

// WriteRawElement holds a captured element's own name to the QName rule, so a
// replayed child rebuilt as "<ns1::/>" fails the save instead of shipping a
// part nothing can read back.
func TestWriteRawElementRefusesANonQName(t *testing.T) {
	refuse := []string{`<ns1::/>`, `<:/>`, `<p:x: a="1"/>`, `<:x>t</:x>`, "<x:\n/>"}
	for _, raw := range refuse {
		b := NewBuilder()
		b.StartElement("", "r")
		b.WriteRawElement([]byte(raw))
		b.EndElement("", "r")
		if err := b.Finish(); !errors.Is(err, ErrUnwritableName) {
			t.Errorf("%q: Finish = %v, want ErrUnwritableName", raw, err)
		}
	}
	accept := []string{`<p:x a="1"/>`, `<x/>`, `<x>t</x>`, "<x\n/>", ``}
	for _, raw := range accept {
		b := NewBuilder()
		b.StartElement("", "r")
		b.WriteRawElement([]byte(raw))
		b.EndElement("", "r")
		if err := b.Finish(); err != nil {
			t.Errorf("%q: Finish = %v, want nil", raw, err)
		}
	}
}
