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
