package imagesniff

import "testing"

func TestDetect(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want Kind
	}{
		{"png", []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0, 0}, PNG},
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0}, JPEG},
		{"gif87a", []byte("GIF87a....."), GIF},
		{"gif89a", []byte("GIF89a....."), GIF},
		{"bmp", []byte("BM\x00\x00"), BMP},
		{"tiff little-endian", []byte{'I', 'I', 0x2A, 0x00, 0}, TIFF},
		{"tiff big-endian", []byte{'M', 'M', 0x00, 0x2A, 0}, TIFF},
		{"wmf placeable", []byte{0xD7, 0xCD, 0xC6, 0x9A, 0}, WMF},
		{"wmf metaheader", []byte{0x01, 0x00, 0x09, 0x00, 0}, WMF},
		{"svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), SVG},
		{"svg with declaration and doctype", []byte(
			"\xEF\xBB\xBF<?xml version=\"1.0\"?>\n<!DOCTYPE svg><!-- c --><SVG/>"), SVG},

		{"empty", nil, Unknown},
		{"plain text", []byte("this is definitely not an image"), Unknown},
		{"truncated png signature", []byte{0x89, 'P', 'N'}, Unknown},
		{"xml that is not svg", []byte(`<?xml version="1.0"?><root/>`), Unknown},
		// A leading 0x01000000 alone must not read as EMF: the " EMF"
		// signature at offset 40 is what identifies one.
		{"not EMF despite iType", make([]byte, 64), Unknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Detect(tc.data); got != tc.want {
				t.Errorf("Detect = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDetectEMF(t *testing.T) {
	emf := make([]byte, 44)
	emf[0] = 0x01 // iType = EMR_HEADER
	copy(emf[40:44], []byte{0x20, 0x45, 0x4D, 0x46})
	if got := Detect(emf); got != EMF {
		t.Errorf("Detect(EMF header) = %v, want EMF", got)
	}
	// Same header without the signature is not an EMF.
	emf[41] = 'X'
	if got := Detect(emf); got == EMF {
		t.Error("Detect accepted an EMR_HEADER with the wrong dSignature")
	}
}

func TestKindMetadata(t *testing.T) {
	for _, k := range []Kind{PNG, JPEG, GIF, BMP, TIFF, EMF, WMF, SVG} {
		if k.ContentType() == "" {
			t.Errorf("%v has no content type", k)
		}
		if k.Ext() == "" {
			t.Errorf("%v has no extension", k)
		}
		if k.String() == "unknown" {
			t.Errorf("%v renders as \"unknown\"", int(k))
		}
	}
	if Unknown.ContentType() != "" || Unknown.Ext() != "" || Unknown.String() != "unknown" {
		t.Error("Unknown should carry no content type or extension")
	}
}

func TestIn(t *testing.T) {
	if !PNG.In(PNG, JPEG, GIF) {
		t.Error("PNG.In(PNG, ...) = false")
	}
	if BMP.In(PNG, JPEG, GIF) {
		t.Error("BMP.In(PNG, JPEG, GIF) = true")
	}
	// Unknown is never in any set, even one that (nonsensically) lists it: a
	// caller passing the zero value must not accidentally admit garbage.
	if Unknown.In(Unknown, PNG) {
		t.Error("Unknown.In(...) = true")
	}
}

func TestSVGSizePx(t *testing.T) {
	cases := []struct {
		name string
		data string
		w, h int
	}{
		{"width and height", `<svg width="64" height="32"/>`, 64, 32},
		{"units stripped", `<svg width="64px" height="32pt"/>`, 64, 32},
		{"viewBox fallback", `<svg viewBox="0 0 100 50"/>`, 100, 50},
		{"width wins over viewBox", `<svg width="10" viewBox="0 0 100 50"/>`, 10, 50},
		{"css default", `<svg/>`, 300, 150},
		{"malformed viewBox", `<svg viewBox="nope"/>`, 300, 150},
		{"zero rejected", `<svg width="0" height="0"/>`, 300, 150},
		{"not xml", `not xml at all`, 300, 150},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, h := SVGSizePx([]byte(tc.data))
			if w != tc.w || h != tc.h {
				t.Errorf("SVGSizePx = %dx%d, want %dx%d", w, h, tc.w, tc.h)
			}
		})
	}
}
