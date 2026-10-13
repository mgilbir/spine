package testutil

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
)

// UTF8BOM is the UTF-8 encoding of U+FEFF. Word writes it at the start of some
// parts (document.xml, theme1.xml); XML 1.0 §4.3.3 permits it there.
const UTF8BOM = "\xEF\xBB\xBF"

// WithBOM returns the package with every XML part (".xml" and ".rels"
// entries, [Content_Types].xml included) prefixed by a UTF-8 byte order mark,
// keeping entry order. Parts that already start with one are left alone. It
// returns the names of the parts it changed.
func WithBOM(data []byte) ([]byte, []string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, nil, err
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	var changed []string
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
		if (strings.HasSuffix(lower, ".xml") || strings.HasSuffix(lower, ".rels")) && !bytes.HasPrefix(b, []byte(UTF8BOM)) {
			b = append([]byte(UTF8BOM), b...)
			changed = append(changed, f.Name)
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
	return out.Bytes(), changed, nil
}
