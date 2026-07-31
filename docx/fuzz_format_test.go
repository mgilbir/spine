package docx

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/fuzzseed"
)

// assertPartsAreWellFormed decodes every XML part of a saved package. Every
// value the formatting setters take lands in an *attribute* (w:ascii, w:val,
// w:styleId, w:initials), which is the one place a stray quote or an invalid
// character produces a package no reader can open — and the only signal is a
// parse failure, since the writer itself is happy either way.
// assertPartsAreWellFormed checks the syntax of every XML part in a saved
// package. Syntax is only half of it: Go's decoder resolves an undeclared
// prefix to the literal prefix instead of failing, so a part naming elements in
// a namespace nothing declares tokenizes cleanly here while Office reports it
// as damaged. assertEmittedNamespacesResolve is the other half, and takes the
// package the save was built from so a part preserved verbatim from a source
// that was already unresolvable is not blamed on the writer.
func assertPartsAreWellFormed(t *testing.T, data []byte) {
	t.Helper()
	zr, err := zipReader(data)
	if err != nil {
		t.Fatalf("the saved package is not a readable zip: %v", err)
	}
	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, ".xml") && !strings.HasSuffix(f.Name, ".rels") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("opening %s: %v", f.Name, err)
		}
		body, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("reading %s: %v", f.Name, err)
		}
		dec := xml.NewDecoder(bytes.NewReader(body))
		for {
			_, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("%s is not well-formed XML: %v\n%s", f.Name, err, body)
			}
		}
	}
}

// formatSnapshot is everything the formatting API can be asked to report back.
type formatSnapshot struct {
	font, color, styleName, initials string
	styleType                        StyleType
	size                             float64
	strike, bold                     bool
}

func snapshot(t *testing.T, doc *Document) formatSnapshot {
	t.Helper()
	var s formatSnapshot
	if paras := doc.Paragraphs(); len(paras) > 0 {
		if runs := paras[0].Runs(); len(runs) > 0 {
			r := runs[0]
			s.font, s.color, s.size = r.Font(), r.Color(), r.FontSize()
			s.strike, s.bold = r.Strike(), r.Bold()
		}
	}
	if st := doc.Styles().Style("FuzzStyle"); st != nil {
		s.styleName, s.styleType = st.Name(), st.Type()
	}
	if cs := doc.Comments(); len(cs) > 0 {
		s.initials = cs[0].Initials()
	}
	return s
}

// FuzzDocxRunAndStyleFormatting drives the run-format setters, the Style
// builder and Comment.SetInitials with arbitrary strings, then saves and
// reopens twice.
//
// Two invariants are checked, and neither is "it did not panic":
//
//  1. Every XML part of the saved package parses. These setters write their
//     argument straight into an attribute value, so a caller-supplied quote,
//     angle bracket or control character is the package's problem, not the
//     caller's.
//  2. The read-back is a fixed point after one round trip. Arbitrary input may
//     legitimately be sanitized on the way out (XML 1.0 forbids most control
//     characters outright), so the first save is allowed to change a value —
//     but the *second* must not. A value that keeps changing is a setter and a
//     getter that disagree about the encoding, which silently corrupts a
//     document every time it is opened and saved.
func FuzzDocxRunAndStyleFormatting(f *testing.F) {
	f.Add("Cambria Math", "C00FFE", 13.5, "Emphasis Two", "GBH", true)
	f.Add("", "", 0.0, "", "", false)
	f.Add(`Times "New" Roman`, `#GG<x>`, -3.5, `A & B`, `<i>`, true)
	f.Add("font\x00with\x01controls", "col\x0bor", 1e9, "name￾bad", "in￿it", false)
	f.Add("한글 폰트", "AABBCC", 0.5, "スタイル", "日本", true)
	f.Add(strings.Repeat("F", 4096), "112233", 1e-9, strings.Repeat("N", 4096), "\t\n\r", false)

	f.Fuzz(func(t *testing.T, font, colorHex string, size float64, styleName, initials string, strike bool) {
		doc := Create()
		p := doc.AddParagraph()
		r := p.AddRun()
		r.SetText("fuzzed")
		r.SetFont(font)
		r.SetColor(colorHex)
		r.SetFontSize(size)
		r.SetStrike(strike)
		r.SetBold(!strike)

		doc.Styles().AddCharacterStyle("FuzzStyle", styleName).
			SetName(styleName).
			SetLink(styleName).
			SetUIPriority(7).
			SetAlignment(AlignmentCenter).
			SetLineSpacing(size).
			SetIndentHanging(size)

		p.AddComment("Fuzz Author", "fuzz comment").SetInitials(initials)

		first, err := doc.SaveBytes()
		if err != nil {
			// Refusing to write is a legitimate outcome for hostile input.
			return
		}
		assertPartsAreWellFormed(t, first)
		assertEmittedNamespacesResolve(t, nil, first)

		once, err := OpenReader(bytes.NewReader(first), int64(len(first)))
		if err != nil {
			t.Fatalf("a package this library just wrote does not reopen: %v", err)
		}
		defer func() { _ = once.Close() }()
		afterOne := snapshot(t, once)

		second, err := once.SaveBytes()
		if err != nil {
			t.Fatalf("re-saving a reopened package failed: %v", err)
		}
		assertPartsAreWellFormed(t, second)
		assertEmittedNamespacesResolve(t, nil, second)

		twice, err := OpenReader(bytes.NewReader(second), int64(len(second)))
		if err != nil {
			t.Fatalf("the twice-saved package does not reopen: %v", err)
		}
		defer func() { _ = twice.Close() }()
		afterTwo := snapshot(t, twice)

		if afterOne != afterTwo {
			t.Fatalf("the formatting is not a fixed point after one round trip:\n first reopen:  %+v\n second reopen: %+v",
				afterOne, afterTwo)
		}
	})
}

// assertEmittedNamespacesResolve holds a saved package to the rule that every
// element it names resolves to a declared namespace. See
// fuzzseed.AssertEmittedNamespacesResolve for why the source package is needed
// to blame it correctly, and for the two ordinary public-API calls that broke
// the rule before the Builder declared what it writes.
func assertEmittedNamespacesResolve(t *testing.T, source, saved []byte) {
	t.Helper()
	fuzzseed.AssertEmittedNamespacesResolve(t, source, saved)
}
