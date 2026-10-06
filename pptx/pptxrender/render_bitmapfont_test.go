package pptxrender

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/render"
)

// renderBitmapFace is a font of one glyph, the letter A, that is an 8 by 8
// monochrome bitmap of a strike of 10 pixels to the em and has no outlines.
func renderBitmapFace(t *testing.T) *shape.Face {
	t.Helper()
	be16 := func(v int) []byte { return []byte{byte(v >> 8), byte(v)} }
	be32 := func(v int) []byte { return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)} }
	cat := func(parts ...[]byte) []byte {
		var out []byte
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}
	ebdt := cat(be16(2), be16(0), []byte{8, 8, 0, 8, 8}, []byte{0xF0, 0xF0, 0xF0, 0xF0, 0xF0, 0xF0, 0xF0, 0xF0})
	array := cat(be16(1), be16(1), be32(8))
	sub := cat(be16(1), be16(1), be32(4), be32(0), be32(len(ebdt)-4))
	line := cat([]byte{10, 0, 0, 0}, make([]byte, 8))
	eblc := cat(be16(2), be16(0), be32(1), be32(56), be32(len(array)+len(sub)), be32(1), be32(0), line, line,
		be16(1), be16(1), []byte{10, 10, 1, 1}, array, sub)
	font := fonttest.SFNT(fonttest.SFNTOptions{Glyphs: []fonttest.Glyph{{Rune: 'A', Advance: 1000, HasShape: true}}, Extra: map[string][]byte{"EBLC": eblc, "EBDT": ebdt}})
	// Drop glyf and loca.
	n := int(font[4])<<8 | int(font[5])
	tags := make([]string, 0, n)
	bodies := map[string][]byte{}
	for i := 0; i < n; i++ {
		rec := font[12+16*i:]
		tag := string(rec[:4])
		off := int(rec[8])<<24 | int(rec[9])<<16 | int(rec[10])<<8 | int(rec[11])
		length := int(rec[12])<<24 | int(rec[13])<<16 | int(rec[14])<<8 | int(rec[15])
		if tag != "glyf" && tag != "loca" {
			tags = append(tags, tag)
			bodies[tag] = font[off : off+length]
		}
	}
	out := cat(font[:4], be16(len(tags)), make([]byte, 6), make([]byte, 16*len(tags)))
	for i, tag := range tags {
		for len(out)%4 != 0 {
			out = append(out, 0)
		}
		copy(out[12+16*i:], tag)
		copy(out[12+16*i+8:], be32(len(out)))
		copy(out[12+16*i+12:], be32(len(bodies[tag])))
		out = append(out, bodies[tag]...)
	}
	f, err := shape.Load(out)
	if err != nil {
		t.Fatal(err)
	}
	if !f.BitmapOnly() {
		t.Fatal("not a bitmap font")
	}
	return f
}

func TestRenderTurnedBitmapFontText(t *testing.T) {
	data, opts := renderInheritedText(t)
	face := renderBitmapFace(t)
	opts.Fonts = func(context.Context, render.FontRequest) (*shape.Face, error) { return face, nil }
	rewrites := renderBody(`<a:lstStyle/>`, `<a:p><a:r><a:rPr lang="en-US"/><a:t>A</a:t></a:r></a:p>`)
	turned := func(s string) string {
		s = rewrites(s)
		i := strings.Index(s, "<p:sp>")
		return s[:i] + strings.Replace(s[i:], `<a:xfrm>`, `<a:xfrm rot="5400000">`, 1)
	}
	files := map[string]func(string) string{"ppt/slides/slide1.xml": turned}
	// Strict refuses text it cannot draw.
	if _, err := renderRewrittenPNG(t, data, opts, files); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict: %v", err)
	}
	// Best effort leaves the glyphs out, says so, and draws the slide.
	var warnings []string
	lenient := opts
	lenient.Warn = func(err error) {
		if errors.Is(err, render.ErrApproximated) && strings.Contains(err.Error(), "bitmap font") {
			warnings = append(warnings, err.Error())
		}
	}
	renderSlidePNG(t, data, lenient, files)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "turned glyphs of a bitmap font left out") {
		t.Fatalf("warnings: %q", warnings)
	}
}
