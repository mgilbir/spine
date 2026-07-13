package xml

import "strings"

// Prolog captures a source part's XML declaration and the whitespace around
// the document element, so a regenerated part reproduces the producer's
// formatting byte-for-byte. Producers differ in every respect the type
// records: LibreOffice omits standalone="yes" and separates with a bare \n,
// some generators use a bare \r or no declaration at all, and several write a
// trailing newline after the document element.
type Prolog struct {
	// Captured reports whether the values below were captured from a source
	// part. When false, writers fall back to the default declaration.
	Captured bool

	// Decl is the XML declaration exactly as the source wrote it, including
	// any leading byte-order mark and the closing "?>". Empty when the source
	// part had no declaration.
	Decl string

	// Sep is the bytes between the declaration (or the start of the part)
	// and the '<' of the document element.
	Sep string

	// Trailer is the bytes after the final '>' of the document element
	// (e.g. "\r\n" for producers that end the part with a newline).
	Trailer string
}

// CaptureProlog extracts the declaration, separator, and trailer from a source
// part's raw bytes.
func CaptureProlog(data []byte) Prolog {
	p := Prolog{Captured: true}
	s := string(data)

	i := 0
	if strings.HasPrefix(s, "\xef\xbb\xbf") {
		i = 3
	}
	if strings.HasPrefix(s[i:], "<?xml") {
		if end := strings.Index(s[i:], "?>"); end >= 0 {
			declEnd := i + end + 2
			p.Decl = s[:declEnd]
			i = declEnd
		}
	}
	if lt := strings.IndexByte(s[i:], '<'); lt >= 0 {
		p.Sep = s[i : i+lt]
	}
	if gt := strings.LastIndexByte(s, '>'); gt >= 0 && gt+1 < len(s) {
		p.Trailer = s[gt+1:]
	}
	return p
}

// WriteProlog writes the captured declaration and separator, or the default
// header (WriteHeader) when p was never captured from a source part.
func (b *Builder) WriteProlog(p Prolog) {
	if !p.Captured {
		b.WriteHeader()
		return
	}
	if p.Decl != "" {
		b.WriteRaw([]byte(p.Decl))
	}
	if p.Sep != "" {
		b.WriteRaw([]byte(p.Sep))
	}
}

// WriteTrailer writes the captured bytes that followed the document element.
// Call it after the root element has been closed.
func (b *Builder) WriteTrailer(p Prolog) {
	if p.Trailer != "" {
		b.WriteRaw([]byte(p.Trailer))
	}
}
