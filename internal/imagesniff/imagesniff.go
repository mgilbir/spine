// Package imagesniff identifies image bytes by their magic prefix.
//
// It exists because the three format packages each need the same answer to the
// same question — "are these bytes actually an image, and of what kind?" — at
// the moment a caller hands them over. Before C441 only xlsx asked: docx and
// pptx accepted arbitrary bytes under an "image/png" content type and wrote a
// corrupt media part, and pptx did so while its own godoc promised a save-time
// failure. One sniffer keeps the three answers identical, which is the point:
// a user who learns one format's rejection rule has learned all three.
//
// Detection is prefix-only. It never decodes pixel data, so it is O(1) in the
// image size and cannot be driven into heavy work by hostile input.
package imagesniff

import (
	"bytes"
	"encoding/xml"
	"strconv"
	"strings"
)

// Kind is a recognized image format.
type Kind int

const (
	// Unknown means the bytes match no format this package recognizes.
	Unknown Kind = iota
	PNG
	JPEG
	GIF
	BMP
	TIFF
	EMF
	WMF
	SVG
)

// ContentType returns the OPC content type for a kind, or "" for Unknown.
func (k Kind) ContentType() string {
	switch k {
	case PNG:
		return "image/png"
	case JPEG:
		return "image/jpeg"
	case GIF:
		return "image/gif"
	case BMP:
		return "image/bmp"
	case TIFF:
		return "image/tiff"
	case EMF:
		return "image/x-emf"
	case WMF:
		return "image/x-wmf"
	case SVG:
		return "image/svg+xml"
	}
	return ""
}

// Ext returns the conventional file extension for a kind, without the dot, or
// "" for Unknown.
func (k Kind) Ext() string {
	switch k {
	case PNG:
		return "png"
	case JPEG:
		return "jpeg"
	case GIF:
		return "gif"
	case BMP:
		return "bmp"
	case TIFF:
		return "tiff"
	case EMF:
		return "emf"
	case WMF:
		return "wmf"
	case SVG:
		return "svg"
	}
	return ""
}

// String renders the kind's name as it appears in error messages.
func (k Kind) String() string {
	switch k {
	case PNG:
		return "PNG"
	case JPEG:
		return "JPEG"
	case GIF:
		return "GIF"
	case BMP:
		return "BMP"
	case TIFF:
		return "TIFF"
	case EMF:
		return "EMF"
	case WMF:
		return "WMF"
	case SVG:
		return "SVG"
	}
	return "unknown"
}

// Detect reports which image format data is in, or Unknown when the bytes match
// none. Empty input is Unknown.
func Detect(data []byte) Kind {
	switch {
	case len(data) >= 8 && bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return PNG
	case len(data) >= 3 && bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		return JPEG
	case len(data) >= 6 && (bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a"))):
		return GIF
	case len(data) >= 2 && bytes.HasPrefix(data, []byte("BM")):
		return BMP
	// TIFF: "II*\0" (little-endian) or "MM\0*" (big-endian). Checked before EMF
	// and WMF, whose signatures are longer and cannot collide with these.
	case len(data) >= 4 && (bytes.HasPrefix(data, []byte{'I', 'I', 0x2A, 0x00}) || bytes.HasPrefix(data, []byte{'M', 'M', 0x00, 0x2A})):
		return TIFF
	// EMF: an ENHMETAHEADER whose iType is 1 and whose dSignature (at offset 40)
	// is " EMF". The signature is what distinguishes it from any other file
	// starting with 0x01000000.
	case len(data) >= 44 && bytes.HasPrefix(data, []byte{0x01, 0x00, 0x00, 0x00}) && bytes.Equal(data[40:44], []byte{0x20, 0x45, 0x4D, 0x46}):
		return EMF
	// WMF: the placeable header magic (0x9AC6CDD7), or a plain METAHEADER whose
	// mtType is 1 or 2 and whose mtHeaderSize is 9 words.
	case len(data) >= 4 && bytes.HasPrefix(data, []byte{0xD7, 0xCD, 0xC6, 0x9A}):
		return WMF
	case len(data) >= 4 && (bytes.HasPrefix(data, []byte{0x01, 0x00, 0x09, 0x00}) || bytes.HasPrefix(data, []byte{0x02, 0x00, 0x09, 0x00})):
		return WMF
	case IsSVG(data):
		return SVG
	}
	return Unknown
}

// IsSVG reports whether data looks like an SVG document: an <svg> root,
// possibly preceded by a UTF-8 BOM, whitespace, an XML declaration, comments or
// a doctype. Only a bounded prefix is scanned, so a large non-SVG XML document
// costs the same as a small one.
func IsSVG(data []byte) bool {
	s := bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	s = bytes.TrimSpace(s)
	if len(s) == 0 || s[0] != '<' {
		return false
	}
	limit := 512
	if len(s) < limit {
		limit = len(s)
	}
	return bytes.Contains(bytes.ToLower(s[:limit]), []byte("<svg"))
}

// SVGSizePx returns an SVG's intrinsic pixel size, taken from the root
// width/height attributes, falling back to the viewBox and then to the CSS
// default of 300x150. Only the root element is decoded.
//
// It lives here rather than in one format package because xlsx and pptx both
// need an SVG's size to give a freshly added picture a sensible default frame,
// and a 1x1 transparent PNG — the raster fallback both write — is not it.
func SVGSizePx(data []byte) (w, h int) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok || strings.ToLower(se.Name.Local) != "svg" {
			continue
		}
		var vbW, vbH int
		for _, a := range se.Attr {
			switch strings.ToLower(a.Name.Local) {
			case "width":
				w = svgLength(a.Value)
			case "height":
				h = svgLength(a.Value)
			case "viewbox":
				vbW, vbH = svgViewBox(a.Value)
			}
		}
		if w <= 0 {
			w = vbW
		}
		if h <= 0 {
			h = vbH
		}
		break
	}
	if w <= 0 {
		w = 300
	}
	if h <= 0 {
		h = 150
	}
	return w, h
}

// svgLength parses an SVG length: a leading number with an optional unit
// suffix ("px", "pt", ...). Non-pixel units are treated as pixels — a coarse
// intrinsic-size estimate that callers can override with an explicit size.
func svgLength(v string) int {
	v = strings.TrimSpace(v)
	end := 0
	for end < len(v) && (v[end] == '.' || v[end] == '-' || v[end] == '+' || (v[end] >= '0' && v[end] <= '9')) {
		end++
	}
	if end == 0 {
		return 0
	}
	f, err := strconv.ParseFloat(v[:end], 64)
	if err != nil || f <= 0 {
		return 0
	}
	return int(f + 0.5)
}

// svgViewBox parses "min-x min-y width height" and returns width, height.
func svgViewBox(v string) (w, h int) {
	fields := strings.FieldsFunc(v, func(r rune) bool { return r == ' ' || r == ',' || r == '\t' })
	if len(fields) != 4 {
		return 0, 0
	}
	fw, err1 := strconv.ParseFloat(fields[2], 64)
	fh, err2 := strconv.ParseFloat(fields[3], 64)
	if err1 != nil || err2 != nil {
		return 0, 0
	}
	return int(fw + 0.5), int(fh + 0.5)
}

// In reports whether k is one of the allowed kinds. Each format package has its
// own accepted set (docx and xlsx embed raster images plus SVG; pptx also
// accepts the metafile formats PowerPoint renders), so the shared sniffer
// answers "what is it" and the caller answers "do I take it".
func (k Kind) In(allowed ...Kind) bool {
	if k == Unknown {
		return false
	}
	for _, a := range allowed {
		if k == a {
			return true
		}
	}
	return false
}
