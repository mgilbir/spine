package xlsx

import (
	"bytes"
	"encoding/xml"
	"strconv"
	"strings"
)

// isSVG reports whether data looks like an SVG document (an <svg> root,
// possibly preceded by an XML declaration, BOM, or doctype/comments).
func isSVG(data []byte) bool {
	s := data
	// Skip a UTF-8 BOM.
	s = bytes.TrimPrefix(s, []byte{0xEF, 0xBB, 0xBF})
	s = bytes.TrimSpace(s)
	if len(s) == 0 || s[0] != '<' {
		return false
	}
	// Scan a bounded prefix for the <svg tag, tolerating <?xml ...?>, <!-- -->,
	// and <!DOCTYPE ...> before it.
	limit := 512
	if len(s) < limit {
		limit = len(s)
	}
	head := bytes.ToLower(s[:limit])
	return bytes.Contains(head, []byte("<svg"))
}

// svgIntrinsicSize returns the intrinsic pixel size of an SVG from its root
// width/height, falling back to the viewBox, then to the CSS default 300x150.
func svgIntrinsicSize(data []byte) (w, h int) {
	w, h = 0, 0
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
				w = parseSVGLength(a.Value)
			case "height":
				h = parseSVGLength(a.Value)
			case "viewbox":
				vbW, vbH = parseViewBox(a.Value)
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

// parseSVGLength parses an SVG length, accepting a leading number with an
// optional unit suffix ("px", "pt", ...). Non-pixel units are treated as
// pixels (a coarse intrinsic-size estimate; callers can set an explicit size).
func parseSVGLength(v string) int {
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

// parseViewBox parses "min-x min-y width height" and returns width, height.
func parseViewBox(v string) (w, h int) {
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
