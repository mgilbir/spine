package metafile

func (e *emfBuilder) raw(b []byte) *emfBuilder {
	e.records = append(e.records, b)
	return e
}

func plusRecord(typ uint16, flags uint16, body []byte) []byte {
	b := make([]byte, (12+len(body)+3)&^3)
	le16(b, 0, typ)
	le16(b, 2, flags)
	le32(b, 4, uint32(len(b)))
	le32(b, 8, uint32(len(body)))
	copy(b[12:], body)
	return b
}

// plusComment is an EMR_COMMENT holding EMF+ records.
func plusComment(records ...[]byte) []byte {
	body := make([]byte, 8)
	le32(body, 4, 0x2b464d45)
	for _, r := range records {
		body = append(body, r...)
	}
	le32(body, 0, uint32(len(body)-4))
	b := make([]byte, 8+len(body))
	le32(b, 0, 70)
	le32(b, 4, uint32(len(b)))
	copy(b[8:], body)
	return b
}

func plusHeader(dual bool) []byte {
	b := words(int32(-0x243fefee), 0, 96, 96)
	flags := uint16(0)
	if dual {
		flags = 1
	}
	return plusRecord(0x4001, flags, b)
}

func plusEOF() []byte { return plusRecord(0x4002, 0, nil) }

// font adds an ExtCreateFontIndirectW record for a face.
func (e *emfBuilder) font(h uint16, height, escapement, weight int32, face string) *emfBuilder {
	e.handle(h)
	b := words(int32(h), height, 0, escapement, escapement, weight)
	b = append(b, 0, 0, 0, 0, 0, 0, 0, 0) // italic, underline, strikeout, charset, precisions, quality, pitch
	name := make([]byte, 64)
	for i, c := range face {
		name[2*i] = byte(c)
	}
	return e.rec(82, append(b, name...))
}

// textW adds an ExtTextOutW record without a rectangle (ETO_NO_RECT is set).
func (e *emfBuilder) textW(x, y int32, s string, dx []int32, options int32) *emfBuilder {
	options |= 0x100
	n := int32(len(s))
	const fixed = 8 + 13*4
	str := make([]byte, 2*len(s))
	for i, c := range s {
		str[2*i] = byte(c)
	}
	if len(str)%4 != 0 {
		str = append(str, 0, 0)
	}
	offDx := int32(0)
	if len(dx) > 0 {
		offDx = int32(fixed + len(str))
	}
	body := words(0, 0, 0, 0, 1, f32(1), f32(1), x, y, n, fixed, options, offDx)
	body = append(body, str...)
	body = append(body, words(dx...)...)
	return e.rec(84, body)
}

// dib24 is a packed DIB of 24-bit pixels, rows bottom up, each pixel b, g, r.
func dib24(w, h int, rows [][][3]byte) (info, bits []byte) {
	info = make([]byte, 40)
	le32(info, 0, 40)
	le32(info, 4, uint32(w))
	le32(info, 8, uint32(h))
	le16(info, 12, 1)
	le16(info, 14, 24)
	stride := (w*3 + 3) &^ 3
	bits = make([]byte, stride*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			copy(bits[y*stride+3*x:], rows[y][x][:])
		}
	}
	return
}

// stretchDIBits adds an EMR_STRETCHDIBITS record.
func (e *emfBuilder) stretchDIBits(dx, dy, dw, dh int32, info, bits []byte, rop int32, sw, sh int32) *emfBuilder {
	const fixed = 80
	body := words(0, 0, 0, 0, dx, dy, 0, 0, sw, sh, fixed, int32(len(info)), int32(fixed+len(info)), int32(len(bits)), 0, rop, dw, dh)
	body = append(body, info...)
	body = append(body, bits...)
	return e.rec(81, body)
}

// plusFill is an EmfPlusFillRects record of one rectangle in a solid ARGB
// color.
func plusFill(x, y, w, h float32, argb uint32) []byte {
	return plusRecord(0x400a, 0x8000, words(int32(argb), 1, f32(x), f32(y), f32(w), f32(h)))
}

// plusPixels is an EmfPlusSetPageTransform to pixels, at scale 1.
func plusPixels() []byte { return plusRecord(0x4030, 2, words(f32(1))) }
