package opc

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// nestedPackage builds a package whose one XML part nests depth levels deep.
func nestedPackage(t *testing.T, depth int) []byte {
	t.Helper()
	var body strings.Builder
	body.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><root>`)
	for i := 0; i < depth; i++ {
		body.WriteString(`<g>`)
	}
	for i := 0; i < depth; i++ {
		body.WriteString(`</g>`)
	}
	body.WriteString(`</root>`)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	ct, err := zw.Create("[Content_Types].xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ct.Write([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="xml" ContentType="application/xml"/></Types>`)); err != nil {
		t.Fatal(err)
	}
	w, err := zw.Create("deep/part.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(body.String())); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// readDeepPart opens pkg with opts and reads the nested part, which is where the
// bound is enforced — reading a part is the one chokepoint every part's bytes
// pass through.
func readDeepPart(t *testing.T, pkg []byte, opts ReaderOptions) error {
	t.Helper()
	r, err := NewReaderWithOptions(bytes.NewReader(pkg), int64(len(pkg)), opts)
	if err != nil {
		return err
	}
	f := r.GetFile("/deep/part.xml")
	if f == nil {
		t.Fatal("fixture is missing /deep/part.xml, so this test would pass without checking anything")
	}
	_, err = f.ReadAll()
	return err
}

// Nesting is a resource dimension the byte-oriented limits cannot see: a part
// under a megabyte can nest deeply enough to exhaust memory, because every level
// costs a decoder frame and a model frame however few bytes express it. 244 KB
// of nested p:grpSp cost 627 MB resident before this bound existed.
func TestNestingDepthBound(t *testing.T) {
	shallow := nestedPackage(t, 50)
	deep := nestedPackage(t, MaxNestingDepth+50)

	t.Run("real depth is admitted", func(t *testing.T) {
		if err := readDeepPart(t, shallow, ReaderOptions{}); err != nil {
			t.Errorf("a 50-level part was rejected: %v", err)
		}
	})

	t.Run("past the default is rejected", func(t *testing.T) {
		err := readDeepPart(t, deep, ReaderOptions{})
		if !errors.Is(err, xmlb.ErrNestingTooDeep) {
			t.Errorf("error = %v, want one wrapping ErrNestingTooDeep", err)
		}
		// The message has to say which knob to reach for; a bound that rejects a
		// file without naming its own override is a dead end for the caller.
		if err == nil || !strings.Contains(err.Error(), "MaxNestingDepth") {
			t.Errorf("error %v does not name MaxNestingDepth", err)
		}
	})

	t.Run("the option raises it", func(t *testing.T) {
		opts := ReaderOptions{MaxNestingDepth: MaxNestingDepth + 100}
		if err := readDeepPart(t, deep, opts); err != nil {
			t.Errorf("raising MaxNestingDepth did not admit the part: %v", err)
		}
	})

	t.Run("a negative option disables it", func(t *testing.T) {
		// Matches MaxDecompressedPartSize and friends: zero means "use the
		// package default", negative means unbounded.
		opts := ReaderOptions{MaxNestingDepth: -1}
		if err := readDeepPart(t, deep, opts); err != nil {
			t.Errorf("a negative MaxNestingDepth did not disable the bound: %v", err)
		}
	})

	t.Run("zero uses the package default", func(t *testing.T) {
		if err := readDeepPart(t, deep, ReaderOptions{MaxNestingDepth: 0}); !errors.Is(err, xmlb.ErrNestingTooDeep) {
			t.Errorf("zero should fall back to the package default, got %v", err)
		}
	})
}

// TestNestingBoundIsCapturedPerReader pins the concurrency contract the sibling
// limits document: the value is snapshotted when the Reader is built, so a later
// change to the package variable cannot alter a Reader already open.
func TestNestingBoundIsCapturedPerReader(t *testing.T) {
	saved := MaxNestingDepth
	defer func() { MaxNestingDepth = saved }()

	deep := nestedPackage(t, saved+50)
	r, err := NewReader(bytes.NewReader(deep), int64(len(deep)))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	// Raising the global after construction must not admit this Reader's part.
	MaxNestingDepth = saved + 1000
	f := r.GetFile("/deep/part.xml")
	if f == nil {
		t.Fatal("fixture is missing /deep/part.xml")
	}
	if _, err := f.ReadAll(); !errors.Is(err, xmlb.ErrNestingTooDeep) {
		t.Errorf("the Reader honoured a global raised after it was built: %v", err)
	}
}

// TestNonMarkupPartsAreNotScanned documents the one carve-out: media parts have
// no element structure to nest, and scanning them would put the cost on the
// bytes that dominate a package for no benefit.
func TestNonMarkupPartsAreNotScanned(t *testing.T) {
	for _, name := range []string{"word/document.xml", "_rels/.rels", "xl/drawings/d.vml"} {
		if !isMarkupPart(name) {
			t.Errorf("isMarkupPart(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"word/media/image1.png", "ppt/media/v.mp4", "docProps/thumbnail.jpeg"} {
		if isMarkupPart(name) {
			t.Errorf("isMarkupPart(%q) = true, want false", name)
		}
	}
}
