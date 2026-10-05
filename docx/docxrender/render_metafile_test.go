package docxrender

import (
	"encoding/binary"
	"errors"
	"image/color"
	"testing"

	"github.com/mgilbir/spine/render"
)

// wordTestEMF is an EMF of a 40 by 40 pixel picture filled by one rectangle in
// a brush of the given style: 0 is solid red, 2 is hatched.
func wordTestEMF(brushStyle uint32) []byte {
	var recs [][]byte
	rec := func(typ uint32, vals ...uint32) {
		b := make([]byte, 8+4*len(vals))
		binary.LittleEndian.PutUint32(b, typ)
		binary.LittleEndian.PutUint32(b[4:], uint32(len(b)))
		for i, v := range vals {
			binary.LittleEndian.PutUint32(b[8+4*i:], v)
		}
		recs = append(recs, b)
	}
	rec(39, 1, brushStyle, 0x0000ff, 0)
	rec(37, 1)
	rec(37, 0x80000008)
	rec(43, 0, 0, 40, 40)
	hdr := make([]byte, 88)
	put := func(off int, v uint32) { binary.LittleEndian.PutUint32(hdr[off:], v) }
	put(0, 1)
	put(4, 88)
	put(16, 39)
	put(20, 39)
	put(32, 1000)
	put(36, 1000)
	put(40, 0x464d4520)
	put(44, 0x10000)
	put(52, uint32(len(recs)+2))
	binary.LittleEndian.PutUint16(hdr[56:], 2)
	put(72, 1000)
	put(76, 1000)
	put(80, 250)
	put(84, 250)
	out := hdr
	for _, r := range recs {
		out = append(out, r...)
	}
	eof := make([]byte, 20)
	binary.LittleEndian.PutUint32(eof, 14)
	binary.LittleEndian.PutUint32(eof[4:], 20)
	binary.LittleEndian.PutUint32(eof[16:], 20)
	out = append(out, eof...)
	binary.LittleEndian.PutUint32(out[48:], uint32(len(out)))
	return out
}

func TestMetafilePictureIsDrawn(t *testing.T) {
	pic := wordTestPic{w: 40, h: 30}
	body := wordTestBody(wordTestPara("", pic.inline()))
	p, _ := wordTestRender(t, body, wordTestMedia(wordTestEMF(0)))
	if n := len(p.pictures(1)); n != 1 {
		t.Fatalf("%d pictures", n)
	}
	if got := nrgba(p.pixels(t, 1), wordTestLeft+20, wordTestTop+15); got != wordTestRed {
		t.Errorf("inside the picture %+v", got)
	}
}

func TestMetafileWhatCannotBeDrawnExactly(t *testing.T) {
	pic := wordTestPic{w: 40, h: 30}
	body := wordTestBody(wordTestPara("", pic.inline()))
	strictErr, pages, o, err := prepareBoth(t, body, wordTestMedia(wordTestEMF(2)), nil)
	if !errors.Is(strictErr, render.ErrUnsupported) {
		t.Errorf("strict: %v", strictErr)
	}
	if err != nil || pages == nil {
		t.Fatal(err)
	}
	if !warned(o, "metafile", render.ErrApproximated) && !warned(o, "metafile", render.ErrUnsupported) {
		t.Errorf("warnings %v", o.warnings)
	}
	if got := nrgba(pages.pixels(t, 1), wordTestLeft+20, wordTestTop+15); got == (color.NRGBA{}) {
		t.Errorf("the hatched fill is left out, over white: %+v", got)
	}
}
