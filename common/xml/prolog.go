package xml

import "bytes"

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
	// and the '<' of the document element, including any comments (Aspose
	// writes a generator comment between the declaration and the root) and,
	// for parts without a declaration, a leading byte-order mark.
	Sep string

	// Trailer is the bytes after the final '>' of the document element
	// (e.g. "\r\n" for producers that end the part with a newline).
	Trailer string

	// RootEnd is the document element's end tag exactly as the source wrote
	// it (e.g. "</p:sld >" with a space before '>'); empty when canonical or
	// when the root is self-closing.
	RootEnd string
}

// CaptureProlog extracts the declaration, separator, and trailer from a source
// part's raw bytes. It works on the byte slice directly and copies only the
// small captured spans (never the whole part) into independent strings: a
// string(data) of the full part, or a substring aliasing it, would pin the
// entire part in memory through the tiny Prolog fields.
func CaptureProlog(data []byte) Prolog {
	p := Prolog{Captured: true}

	i := 0
	if bytes.HasPrefix(data, []byte("\xef\xbb\xbf")) {
		i = 3
	}
	if bytes.HasPrefix(data[i:], []byte("<?xml")) {
		if end := bytes.Index(data[i:], []byte("?>")); end >= 0 {
			declEnd := i + end + 2
			p.Decl = string(data[:declEnd])
			i = declEnd
		}
	} else {
		// No declaration: a leading byte-order mark belongs to Sep so it
		// still round-trips.
		i = 0
	}
	// Everything up to the document element's '<' is the separator,
	// including any comments and the whitespace around them.
	j := i
	for {
		lt := bytes.IndexByte(data[j:], '<')
		if lt < 0 {
			break
		}
		pos := j + lt
		if bytes.HasPrefix(data[pos:], []byte("<!--")) {
			end := bytes.Index(data[pos:], []byte("-->"))
			if end < 0 {
				break
			}
			j = pos + end + 3
			continue
		}
		p.Sep = string(data[i:pos])
		break
	}
	if gt := bytes.LastIndexByte(data, '>'); gt >= 0 {
		if gt+1 < len(data) {
			p.Trailer = string(data[gt+1:])
		}
		if lt := bytes.LastIndex(data[:gt], []byte("</")); lt >= 0 {
			end := data[lt : gt+1]
			// Only keep a non-canonical form (whitespace before '>'), so the
			// builder's normal end tag stays in charge otherwise.
			if bytes.ContainsAny(end[2:len(end)-1], " \t\r\n") {
				p.RootEnd = string(end)
			}
		}
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
