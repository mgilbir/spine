package metafile

import (
	"encoding/binary"
	"math"
)

// The tests build their metafiles here, from the specifications, rather than
// reading files.

func le32(b []byte, off int, v uint32) { binary.LittleEndian.PutUint32(b[off:], v) }
func le16(b []byte, off int, v uint16) { binary.LittleEndian.PutUint16(b[off:], v) }

// emfBuilder writes an EMF whose reference device has four pixels to the
// millimetre, so a w by h pixel picture is w/4 by h/4 millimetres.
type emfBuilder struct {
	w, h    int
	records [][]byte
	handles uint16
}

func newEMF(w, h int) *emfBuilder { return &emfBuilder{w: w, h: h} }

func words(vals ...int32) []byte {
	b := make([]byte, 4*len(vals))
	for i, v := range vals {
		le32(b, 4*i, uint32(v))
	}
	return b
}

func (e *emfBuilder) rec(typ uint32, body []byte) *emfBuilder {
	b := make([]byte, (8+len(body)+3)&^3)
	le32(b, 0, typ)
	le32(b, 4, uint32(len(b)))
	copy(b[8:], body)
	e.records = append(e.records, b)
	return e
}

func (e *emfBuilder) r(typ uint32, vals ...int32) *emfBuilder { return e.rec(typ, words(vals...)) }

func (e *emfBuilder) handle(n uint16) { e.handles = max(e.handles, n) }

func (e *emfBuilder) brush(h uint16, style, color, hatch int32) *emfBuilder {
	e.handle(h)
	return e.r(39, int32(h), style, color, hatch)
}

func (e *emfBuilder) pen(h uint16, style, width int32, color int32) *emfBuilder {
	e.handle(h)
	return e.r(38, int32(h), style, width, 0, color)
}

func (e *emfBuilder) sel(h uint32) *emfBuilder { return e.r(37, int32(h)) }

func (e *emfBuilder) rect(typ uint32, l, t, r, b int32) *emfBuilder { return e.r(typ, l, t, r, b) }

func (e *emfBuilder) poly(typ uint32, pts ...int32) *emfBuilder {
	n := len(pts) / 2
	body := words(0, 0, int32(e.w), int32(e.h), int32(n))
	return e.rec(typ, append(body, words(pts...)...))
}

func f32(v float32) int32 { return int32(math.Float32bits(v)) }

func (e *emfBuilder) bytes() []byte {
	hdr := make([]byte, 88)
	le32(hdr, 0, 1)
	le32(hdr, 4, 88)
	// Bounds, inclusive, in device pixels.
	le32(hdr, 8, 0)
	le32(hdr, 12, 0)
	le32(hdr, 16, uint32(e.w-1))
	le32(hdr, 20, uint32(e.h-1))
	// Frame, in hundredths of a millimetre.
	le32(hdr, 24, 0)
	le32(hdr, 28, 0)
	le32(hdr, 32, uint32(e.w*25))
	le32(hdr, 36, uint32(e.h*25))
	le32(hdr, 40, 0x464d4520)
	le32(hdr, 44, 0x10000)
	le32(hdr, 52, uint32(len(e.records)+2))
	le16(hdr, 56, e.handles+1)
	le32(hdr, 72, 1000)
	le32(hdr, 76, 1000)
	le32(hdr, 80, 250)
	le32(hdr, 84, 250)
	eof := make([]byte, 20)
	le32(eof, 0, 14)
	le32(eof, 4, 20)
	le32(eof, 16, 20)
	out := hdr
	for _, r := range e.records {
		out = append(out, r...)
	}
	out = append(out, eof...)
	le32(out, 48, uint32(len(out)))
	return out
}

// wmfBuilder writes a WMF.
type wmfBuilder struct {
	records [][]byte
	objects uint16
	// placeable bounds, if set.
	bounds    [4]int16
	placeable bool
	inch      uint16
}

func newWMF() *wmfBuilder { return &wmfBuilder{} }

func (w *wmfBuilder) place(l, t, r, b int16, inch uint16) *wmfBuilder {
	w.placeable, w.bounds, w.inch = true, [4]int16{l, t, r, b}, inch
	return w
}

func (w *wmfBuilder) rec(typ uint16, args ...int16) *wmfBuilder {
	b := make([]byte, 6+2*len(args))
	le32(b, 0, uint32(len(b)/2))
	le16(b, 4, typ)
	for i, a := range args {
		le16(b, 6+2*i, uint16(a))
	}
	w.records = append(w.records, b)
	return w
}

func (w *wmfBuilder) bytes() []byte {
	var body []byte
	maxWords := uint32(3)
	for _, r := range w.records {
		body = append(body, r...)
		maxWords = max(maxWords, binary.LittleEndian.Uint32(r))
	}
	eof := []byte{3, 0, 0, 0, 0, 0}
	body = append(body, eof...)
	hdr := make([]byte, 18)
	le16(hdr, 0, 1)
	le16(hdr, 2, 9)
	le16(hdr, 4, 0x300)
	le32(hdr, 6, uint32((18+len(body))/2))
	le16(hdr, 10, w.objects)
	le32(hdr, 12, maxWords)
	out := append(hdr, body...)
	if !w.placeable {
		return out
	}
	p := make([]byte, 22)
	le32(p, 0, 0x9ac6cdd7)
	for i, v := range w.bounds {
		le16(p, 6+2*i, uint16(v))
	}
	le16(p, 14, w.inch)
	var sum uint16
	for i := 0; i < 20; i += 2 {
		sum ^= binary.LittleEndian.Uint16(p[i:])
	}
	le16(p, 20, sum)
	return append(p, out...)
}

// color is a COLORREF.
func rgb(r, g, b int) int32 { return int32(r | g<<8 | b<<16) }
