package render

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestSourcePreflightBoundsAndSyntax(t *testing.T) {
	allow := func(XMLNode) error { return nil }
	for _, tt := range []struct {
		s     string
		bytes int64
		nodes int
		want  error
	}{
		{"<r/>", 3, 100, ErrLimit}, {"<r><c/></r>", 100, 1, ErrLimit},
		{"<r/><r/>", 100, 10, ErrInvalid}, {"<r a='1' a='2'/>", 100, 10, ErrInvalid},
		{"<!DOCTYPE r><r/>", 100, 10, ErrUnsupported}, {"<r><?evil x?></r>", 100, 10, ErrUnsupported},
	} {
		b, e := NewSourceBudget(tt.bytes, tt.nodes)
		if e != nil {
			t.Fatal(e)
		}
		e = b.CheckReader(context.Background(), bytes.NewBufferString(tt.s), allow)
		if !errors.Is(e, tt.want) {
			t.Fatalf("%s: %v", tt.s, e)
		}
	}
	b, _ := NewSourceBudget(100, 100)
	var got []XMLNode
	if e := b.CheckXML(context.Background(), []byte("<r><c/><c/></r>"), func(n XMLNode) error {
		got = append(got, XMLNode{StartElement: n.StartElement, Occurrence: n.Occurrence})
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	if len(got) != 3 || got[2].Occurrence != 2 {
		t.Fatal(got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := b.CheckXML(ctx, []byte("<r/>"), allow); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

// TestSourcePreflightPrologue pins the XML 1.0 rules for what may precede and
// follow the root: a byte order mark only as the very first bytes, the XML
// declaration only as the first token, and only XML white space (not Unicode
// white space) outside the root.
func TestSourcePreflightPrologue(t *testing.T) {
	const bom, decl = "\xEF\xBB\xBF", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`
	for _, tt := range []struct {
		name, s string
		want    error // nil: accepted
	}{
		{"bom", bom + "<r/>", nil},
		{"bom+decl", bom + decl + "\r\n<r/>", nil},
		{"decl", decl + "\n<r/>\n", nil},
		{"trailing comment", "<r/><!-- c -->", nil},
		{"bom after decl", decl + bom + "<r/>", ErrInvalid},
		{"two boms", bom + bom + "<r/>", ErrInvalid},
		{"bom after root", "<r/>" + bom, ErrInvalid},
		{"nbsp before root", " <r/>", ErrInvalid},
		{"nbsp after root", "<r/> ", ErrInvalid},
		{"space before decl", " " + decl + "<r/>", ErrInvalid},
		{"comment before decl", "<!-- c -->" + decl + "<r/>", ErrInvalid},
		{"decl in root", "<r>" + decl + "</r>", ErrInvalid},
		{"decl after root", "<r/>" + decl, ErrInvalid},
	} {
		b, err := NewSourceBudget(0, 0)
		if err != nil {
			t.Fatal(err)
		}
		err = b.CheckXML(context.Background(), []byte(tt.s), func(XMLNode) error { return nil })
		if tt.want == nil && err != nil || tt.want != nil && !errors.Is(err, tt.want) {
			t.Errorf("%s: got %v, want %v", tt.name, err, tt.want)
		}
	}
	// The budget is charged for the bytes as stored, BOM included.
	b, err := NewSourceBudget(int64(len(bom+"<r/>"))-1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = b.CheckXML(context.Background(), []byte(bom+"<r/>"), func(XMLNode) error { return nil }); !errors.Is(err, ErrLimit) {
		t.Fatalf("BOM not charged: %v", err)
	}
}

// TestSourcePreflightTextIsXMLSpace pins that only XML white space is exempt
// from the text check: a no-break space is content the profile must see, or
// it would vanish through a projection that drops text there.
func TestSourcePreflightTextIsXMLSpace(t *testing.T) {
	for _, tt := range []struct {
		s    string
		text bool
	}{
		{"<r> \t\r\n</r>", false},
		{"<r><![CDATA[ ]]></r>", false},
		{"<r> </r>", true},
		{"<r>\ufeff</r>", true},
		{"<r>\u0085</r>", true},
		{"<r>　</r>", true},
		{"<r>x</r>", true},
	} {
		b, err := NewSourceBudget(0, 0)
		if err != nil {
			t.Fatal(err)
		}
		text := false
		if err = b.CheckXML(context.Background(), []byte(tt.s), func(n XMLNode) error {
			text = text || n.Text
			return nil
		}); err != nil {
			t.Fatalf("%q: %v", tt.s, err)
		}
		if text != tt.text {
			t.Errorf("%q: text reported %v, want %v", tt.s, text, tt.text)
		}
	}
}
