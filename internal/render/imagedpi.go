package render

import (
	"bytes"
	"encoding/binary"
)

// ImageDPI reads the resolution an image file records, in dots per inch: a
// PNG's pHYs chunk in pixels per metre, or a JPEG's JFIF density in dots per
// inch or centimetre. ok is false when the file records none, or records
// only an aspect ratio; formats then size the image at 96 DPI. Only the
// headers before the image data are read.
func ImageDPI(data []byte) (x, y float64, ok bool) {
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		for at := 8; at+12 <= len(data); {
			n := int(binary.BigEndian.Uint32(data[at:]))
			kind := string(data[at+4 : at+8])
			if n < 0 || n > len(data)-at-12 || kind == "IDAT" || kind == "IEND" {
				return 0, 0, false
			}
			if kind == "pHYs" {
				if n != 9 || data[at+16] != 1 {
					return 0, 0, false
				}
				return pixelsPerInch(binary.BigEndian.Uint32(data[at+8:]), 0.0254), pixelsPerInch(binary.BigEndian.Uint32(data[at+12:]), 0.0254), validDPI(data[at+8:at+16])
			}
			at += n + 12
		}
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8}):
		for at := 2; at+4 <= len(data); {
			if data[at] != 0xFF {
				return 0, 0, false
			}
			marker := data[at+1]
			if marker == 0xD8 || marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
				at += 2
				continue
			}
			if marker == 0xDA || marker == 0xD9 {
				return 0, 0, false
			}
			n := int(binary.BigEndian.Uint16(data[at+2:]))
			if n < 2 || n > len(data)-at-2 {
				return 0, 0, false
			}
			segment := data[at+4 : at+2+n]
			if marker == 0xE0 && len(segment) >= 12 && bytes.HasPrefix(segment, []byte("JFIF\x00")) {
				dx, dy := float64(binary.BigEndian.Uint16(segment[8:])), float64(binary.BigEndian.Uint16(segment[10:]))
				switch segment[7] {
				case 1:
					return dx, dy, dx > 0 && dy > 0
				case 2:
					return dx * 2.54, dy * 2.54, dx > 0 && dy > 0
				}
				return 0, 0, false
			}
			at += 2 + n
		}
	}
	return 0, 0, false
}

func pixelsPerInch(perUnit uint32, unitInches float64) float64 {
	return float64(perUnit) * unitInches
}

// validDPI checks that both pixels-per-metre values are positive.
func validDPI(b []byte) bool {
	return binary.BigEndian.Uint32(b) > 0 && binary.BigEndian.Uint32(b[4:]) > 0
}
