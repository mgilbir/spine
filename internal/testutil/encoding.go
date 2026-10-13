package testutil

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"
)

// windows1252High maps the code points Windows-1252 places in 0x80-0x9F to
// their bytes; the rest of the code page is ISO-8859-1.
var windows1252High = map[rune]byte{
	0x20AC: 0x80, 0x201A: 0x82, 0x0192: 0x83, 0x201E: 0x84, 0x2026: 0x85, 0x2020: 0x86, 0x2021: 0x87,
	0x02C6: 0x88, 0x2030: 0x89, 0x0160: 0x8A, 0x2039: 0x8B, 0x0152: 0x8C, 0x017D: 0x8E,
	0x2018: 0x91, 0x2019: 0x92, 0x201C: 0x93, 0x201D: 0x94, 0x2022: 0x95, 0x2013: 0x96, 0x2014: 0x97,
	0x02DC: 0x98, 0x2122: 0x99, 0x0161: 0x9A, 0x203A: 0x9B, 0x0153: 0x9C, 0x017E: 0x9E, 0x0178: 0x9F,
}

var encodingDecl = regexp.MustCompile(`(?i)^<\?xml[^?]*?encoding\s*=\s*["'][^"']*["']`)

// WithWindows1252 returns the package with every XML part (".xml" and ".rels"
// entries, [Content_Types].xml included) re-encoded as Windows-1252, its
// declaration naming that encoding (one is added where the part has none),
// keeping entry order. It fails when a part holds a character the code page
// cannot represent. It returns the names of the parts holding a character
// outside ASCII, whose bytes the re-encoding changed.
func WithWindows1252(data []byte) ([]byte, []string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, nil, err
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	var nonASCII []string
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, nil, err
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, nil, err
		}
		lower := strings.ToLower(f.Name)
		if strings.HasSuffix(lower, ".xml") || strings.HasSuffix(lower, ".rels") {
			if b, err = toWindows1252(b); err != nil {
				return nil, nil, fmt.Errorf("%s: %w", f.Name, err)
			}
			for _, c := range b {
				if c >= 0x80 {
					nonASCII = append(nonASCII, f.Name)
					break
				}
			}
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: f.Name, Method: f.Method, Modified: f.Modified})
		if err != nil {
			return nil, nil, err
		}
		if _, err = w.Write(b); err != nil {
			return nil, nil, err
		}
	}
	if err = zw.Close(); err != nil {
		return nil, nil, err
	}
	return out.Bytes(), nonASCII, nil
}

func toWindows1252(b []byte) ([]byte, error) {
	b = bytes.TrimPrefix(b, []byte(UTF8BOM))
	if !utf8.Valid(b) {
		return nil, fmt.Errorf("not UTF-8")
	}
	const decl = `<?xml version="1.0" encoding="windows-1252"`
	if m := encodingDecl.Find(b); m != nil {
		b = append([]byte(decl), b[len(m):]...)
	} else if bytes.HasPrefix(b, []byte("<?xml")) {
		return nil, fmt.Errorf("declaration without an encoding")
	} else {
		b = append([]byte(decl+"?>"), b...)
	}
	out := make([]byte, 0, len(b))
	for _, r := range string(b) {
		switch c, ok := windows1252High[r]; {
		case r < 0x80 || r >= 0xA0 && r <= 0xFF:
			out = append(out, byte(r))
		case ok:
			out = append(out, c)
		default:
			return nil, fmt.Errorf("%U has no Windows-1252 byte", r)
		}
	}
	return out, nil
}
