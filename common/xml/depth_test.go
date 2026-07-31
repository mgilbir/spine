package xml

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

const testLimit = 1000

func nest(depth int) string {
	var b strings.Builder
	b.WriteString(`<root>`)
	for i := 0; i < depth; i++ {
		b.WriteString(`<g>`)
	}
	for i := 0; i < depth; i++ {
		b.WriteString(`</g>`)
	}
	b.WriteString(`</root>`)
	return b.String()
}

func TestCheckNestingDepth(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"flat", `<a><b/><c/></a>`, false},
		{"at the limit", nest(testLimit - 1), false},
		{"past the limit", nest(testLimit + 1), true},
		{"self-closing does not descend", `<a>` + strings.Repeat(`<b/>`, testLimit*2) + `</a>`, false},
		// A '>' inside an attribute value is legal and must not be read as the
		// end of the tag; otherwise <a b=">"/> looks like an unclosed start tag
		// and every following element counts one level too deep.
		{"gt inside an attribute", strings.Repeat(`<a b=">"/>`, 50), false},
		{"gt inside a single-quoted attribute", strings.Repeat(`<a b='>'/>`, 50), false},
		// '<' cannot appear in text or attribute values, so only these three
		// constructs can hide one.
		{"comment", `<a><!-- <b><b><b> --></a>`, false},
		{"cdata", `<a><![CDATA[ <b><b><b> ]]></a>`, false},
		{"processing instruction", `<?xml version="1.0"?><a/>`, false},
		{"unterminated tag does not spin", `<a`, false},
		{"unterminated comment does not spin", `<a><!-- x`, false},
		{"empty", ``, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckNestingDepth([]byte(tc.in), testLimit)
			if tc.wantErr && err == nil {
				t.Fatalf("checkNestingDepth accepted input it should have rejected")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("checkNestingDepth rejected legitimate input: %v", err)
			}
			if tc.wantErr && !errors.Is(err, ErrNestingTooDeep) {
				t.Errorf("error %v does not wrap ErrNestingTooDeep", err)
			}
		})
	}
}

// TestMaxObservedNestingDepthMatchesTheScanner keeps the calibration helper
// honest: it is what the limit was chosen from (170,913 parts across 3,600 real
// documents, deepest 95), so a helper that disagreed with the enforcing scan
// would have justified the wrong number.
func TestMaxObservedNestingDepthMatchesTheScanner(t *testing.T) {
	for _, depth := range []int{0, 1, 5, 95, 300} {
		in := []byte(nest(depth))
		// nest() wraps in <root>, so the observed depth is one more.
		if got, want := MaxObservedNestingDepth(in), depth+1; got != want {
			t.Errorf("MaxObservedNestingDepth(nest(%d)) = %d, want %d", depth, got, want)
		}
		if err := CheckNestingDepth(in, testLimit); err != nil {
			t.Errorf("depth %d rejected: %v", depth, err)
		}
	}
}

// TestZeroLimitDisablesTheCheck pins the convention the opc knobs use: a
// non-positive limit means unbounded, so a caller can turn it off.
func TestZeroLimitDisablesTheCheck(t *testing.T) {
	deep := []byte(nest(testLimit * 3))
	for _, limit := range []int{0, -1} {
		if err := CheckNestingDepth(deep, limit); err != nil {
			t.Errorf("limit %d should disable the check, got %v", limit, err)
		}
	}
}

// realisticPart approximates a worksheet: wide and shallow, which is the shape
// the scan runs over on every part of every document.
func realisticPart() []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="x"><sheetData>`)
	for r := 1; r <= 2000; r++ {
		fmt.Fprintf(&b, `<row r="%d">`, r)
		for c := 0; c < 20; c++ {
			fmt.Fprintf(&b, `<c r="A%d" t="inlineStr"><is><t>cell %d</t></is></c>`, r, c)
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData></worksheet>`)
	return []byte(b.String())
}

// BenchmarkCheckNestingDepth measures what the guard adds to every parse. It is
// a byte scan rather than a token pass precisely so this stays small next to
// the decode it precedes; compare against BenchmarkUnmarshalRealisticPart.
func BenchmarkCheckNestingDepth(b *testing.B) {
	data := realisticPart()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := CheckNestingDepth(data, testLimit); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalRealisticPart(b *testing.B) {
	data := realisticPart()
	type doc struct{}
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var v doc
		if err := Unmarshal(data, &v); err != nil {
			b.Fatal(err)
		}
	}
}
