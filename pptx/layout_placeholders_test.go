package pptx

import (
	"archive/zip"
	"bytes"
	"io"
	"regexp"
	"strings"
	"testing"
)

// partXML returns a part of a saved package.
func partXML(t *testing.T, data []byte, name string) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	f, err := zr.Open(strings.TrimPrefix(name, "/"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck
	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestAddSlideFromLayoutAddsTheLayoutsPlaceholders: a slide added from a
// layout used the layout's design but had no placeholder shapes, so the
// layout's title and body placeholders did not appear on it, unlike a slide
// PowerPoint adds from the same layout. It now carries one placeholder per
// layout placeholder (type, index, orientation and size copied, geometry
// inherited), except the date, footer and slide-number ones, which PowerPoint
// adds only through its header and footer settings.
func TestAddSlideFromLayoutAddsTheLayoutsPlaceholders(t *testing.T) {
	check := func(t *testing.T, p *Presentation, layout *SlideLayout) {
		t.Helper()
		type key struct {
			typ PlaceholderType
			idx uint32
		}
		want := map[key]bool{}
		for _, ph := range layout.Placeholders() {
			switch ph.PlaceholderType() {
			case PlaceholderDateTime, PlaceholderFooter, PlaceholderSlideNumber:
				continue
			}
			want[key{ph.PlaceholderType(), ph.Index()}] = true
		}
		if len(want) == 0 {
			t.Fatalf("layout %q has no content placeholders; the test needs some", layout.Name())
		}
		p.AddSlideFromLayout(layout)
		data, err := p.SaveBytes()
		if err != nil {
			t.Fatal(err)
		}
		r, err := OpenReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		slide := r.Slides()[len(r.Slides())-1]
		got := map[key]bool{}
		for _, ph := range slide.Placeholders() {
			got[key{ph.PlaceholderType(), ph.Index()}] = true
		}
		for k := range want {
			if !got[k] {
				t.Errorf("layout placeholder %q idx %d missing from the slide", k.typ, k.idx)
			}
		}
		for k := range got {
			if !want[k] {
				t.Errorf("slide has placeholder %q idx %d the layout's content placeholders do not", k.typ, k.idx)
			}
		}
		// Geometry is inherited from the layout: no xfrm on the slide.
		xml := partXML(t, data, slide.partName)
		shapes := regexp.MustCompile(`(?s)<p:sp>.*?</p:sp>`).FindAllString(xml, -1)
		if len(shapes) != len(want) {
			t.Errorf("slide has %d shapes, want %d", len(shapes), len(want))
		}
		for _, sp := range shapes {
			if strings.Contains(sp, "<a:xfrm") {
				t.Errorf("a cloned placeholder carries its own xfrm:\n%s", sp)
			}
			// Body properties are inherited too: an untouched text placeholder
			// writes an empty bodyPr, not a new frame's anchor and insets.
			if strings.Contains(sp, "<p:txBody>") && !strings.Contains(sp, "<a:bodyPr/>") {
				t.Errorf("a cloned placeholder overrides the layout's body properties:\n%s", sp)
			}
		}
	}

	t.Run("created deck, Title and Content", func(t *testing.T) {
		p := Create()
		layout, err := p.LayoutByType(LayoutTitleAndContent)
		if err != nil {
			t.Fatal(err)
		}
		check(t, p, layout)
	})
	t.Run("created deck, Title Slide", func(t *testing.T) {
		p := Create()
		layout, err := p.LayoutByType(LayoutTitle)
		if err != nil {
			t.Fatal(err)
		}
		check(t, p, layout)
	})
	t.Run("PowerPoint deck", func(t *testing.T) {
		p, err := Open("testdata/test_slides.pptx")
		if err != nil {
			t.Fatal(err)
		}
		layout, err := p.LayoutByName("Title and Content")
		if err != nil {
			t.Fatal(err)
		}
		check(t, p, layout)
		// The lookups find what was cloned, so filling a new slide works.
		r := p.Slides()[len(p.Slides())-1]
		if r.TitlePlaceholder() == nil || r.BodyPlaceholder() == nil {
			t.Error("TitlePlaceholder or BodyPlaceholder is nil on a slide added from Title and Content")
		}
	})
}
