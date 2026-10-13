package testutil

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"unicode/utf16"
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
// entries, [Content_Types].xml included), or only the named entries when any
// are given, re-encoded as Windows-1252, its declaration naming that encoding
// (one is added where the part has none), keeping entry order. It fails when a
// part holds a character the code page cannot represent. It returns the names
// of the parts holding a character outside ASCII, whose bytes the re-encoding
// changed.
func WithWindows1252(data []byte, only ...string) ([]byte, []string, error) {
	var nonASCII []string
	out, err := reencode(data, only, func(name string, b []byte) ([]byte, error) {
		b, err := toWindows1252(b)
		if err != nil {
			return nil, err
		}
		for _, c := range b {
			if c >= 0x80 {
				nonASCII = append(nonASCII, name)
				break
			}
		}
		return b, nil
	})
	return out, nonASCII, err
}

// WithUTF16 returns the package with every XML part, or only the named
// entries when any are given, re-encoded as UTF-16 in the given byte order
// with a byte order mark, its declaration naming UTF-16 (one is added where
// the part has none), keeping entry order.
func WithUTF16(data []byte, bigEndian bool, only ...string) ([]byte, error) {
	return reencode(data, only, func(_ string, b []byte) ([]byte, error) {
		b, err := redeclare(b, "UTF-16")
		if err != nil {
			return nil, err
		}
		out := make([]byte, 0, 2*len(b)+2)
		put := func(u uint16) {
			if bigEndian {
				out = append(out, byte(u>>8), byte(u))
			} else {
				out = append(out, byte(u), byte(u>>8))
			}
		}
		put(0xFEFF)
		for _, u := range utf16.Encode([]rune(string(b))) {
			put(u)
		}
		return out, nil
	})
}

// reencode rewrites the XML parts of a package, or the named entries, through
// encode, keeping entry order.
func reencode(data []byte, only []string, encode func(name string, b []byte) ([]byte, error)) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		lower := strings.ToLower(f.Name)
		if len(only) > 0 && slices.Contains(only, f.Name) || len(only) == 0 && (strings.HasSuffix(lower, ".xml") || strings.HasSuffix(lower, ".rels")) {
			if b, err = encode(f.Name, b); err != nil {
				return nil, fmt.Errorf("%s: %w", f.Name, err)
			}
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: f.Name, Method: f.Method, Modified: f.Modified})
		if err != nil {
			return nil, err
		}
		if _, err = w.Write(b); err != nil {
			return nil, err
		}
	}
	if err = zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// redeclare returns a UTF-8 part, its byte order mark dropped, with its
// declaration naming encoding; one is added where the part has none.
func redeclare(b []byte, encoding string) ([]byte, error) {
	b = bytes.TrimPrefix(b, []byte(UTF8BOM))
	if !utf8.Valid(b) {
		return nil, fmt.Errorf("not UTF-8")
	}
	decl := `<?xml version="1.0" encoding="` + encoding + `"`
	if m := encodingDecl.Find(b); m != nil {
		return append([]byte(decl), b[len(m):]...), nil
	}
	if bytes.HasPrefix(b, []byte("<?xml")) {
		return nil, fmt.Errorf("declaration without an encoding")
	}
	return append([]byte(decl+"?>"), b...), nil
}

func toWindows1252(b []byte) ([]byte, error) {
	b, err := redeclare(b, "windows-1252")
	if err != nil {
		return nil, err
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
