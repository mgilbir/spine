package opc

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// ValidatePartName checks if a part name conforms to the OPC part-name
// grammar (ECMA-376 Part 2 §9.1.1). Part names must:
//   - Start with a forward slash and not end with one
//   - Not contain empty, ".", or ".." segments
//   - Not have a segment ending with a dot
//   - Contain only pchar characters: unreserved (ALPHA / DIGIT / "-" / "." /
//     "_" / "~"), sub-delims ("!" / "$" / "&" / "'" / "(" / ")" / "*" / "+" /
//     "," / ";" / "="), ":" / "@", and percent-encoded octets
//   - Not percent-encode a slash or backslash (%2F / %5C)
//
// In particular a space, an unencoded '%', a backslash, control characters,
// and non-ASCII bytes are rejected — such names must be percent-encoded.
// This validation applies to parts created through CreatePart/WritePart;
// parts carried over verbatim from an existing package
// (WritePreservedPart) are only checked against the lenient structural
// rules, since wild packages contain entries (e.g. "/[trash]/0000.dat")
// whose names violate the grammar and round-trip fidelity must preserve
// them.
//
// Two rules of §9.1.1 are deliberately not enforced here, because both are
// properties of a package rather than of a name in isolation:
//
//   - The part-name/directory collision rule: a part name must not be a
//     prefix of another part name followed by a segment separator. A package
//     may therefore legally declare both "/word/media" and
//     "/word/media/image1.png" — a combination no filesystem-backed extractor
//     can materialize, since one path cannot be both a file and a directory.
//     Callers that unpack to a filesystem must check for it themselves.
//   - Any length bound. ECMA-376 sets none, but zip consumers and file
//     systems do (a 255-byte path segment, a 260-character Windows MAX_PATH),
//     so a name this function accepts can still be unwritable in practice.
func ValidatePartName(name string) error {
	if err := validatePartNameShape(name); err != nil {
		return err
	}

	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c == '/':
		case c == '%':
			if i+2 >= len(name) || !isHexDigit(name[i+1]) || !isHexDigit(name[i+2]) {
				return fmt.Errorf("%w: %q contains an unencoded '%%'", ErrInvalidPartName, name)
			}
			esc := name[i+1 : i+3]
			if strings.EqualFold(esc, "2F") || strings.EqualFold(esc, "5C") {
				return fmt.Errorf("%w: %q percent-encodes a path separator (%%%s)", ErrInvalidPartName, name, esc)
			}
			i += 2
		case isPartNameChar(c):
		default:
			return fmt.Errorf("%w: %q contains illegal character %q", ErrInvalidPartName, name, rune(c))
		}
	}

	for _, seg := range strings.Split(name[1:], "/") {
		if strings.HasSuffix(seg, ".") {
			return fmt.Errorf("%w: %q has a segment ending with a dot", ErrInvalidPartName, name)
		}
	}

	return nil
}

// validatePartNameShape enforces the lenient structural rules every part name
// must satisfy, including names of parts preserved verbatim from wild source
// packages: leading slash, no trailing slash, no empty or "."/".." segments.
func validatePartNameShape(name string) error {
	if name == "" {
		return ErrInvalidPartName
	}

	// Must start with forward slash
	if !strings.HasPrefix(name, "/") {
		return ErrInvalidPartName
	}

	// Must not end with forward slash
	if strings.HasSuffix(name, "/") {
		return ErrInvalidPartName
	}

	// Check for empty segments
	segments := strings.Split(name, "/")
	for i, seg := range segments {
		// First segment will be empty due to leading slash
		if i == 0 {
			continue
		}
		if seg == "" {
			return ErrInvalidPartName
		}
		// Check for reserved segment names
		if seg == "." || seg == ".." {
			return ErrInvalidPartName
		}
	}

	return nil
}

// isHexDigit reports whether c is an ASCII hexadecimal digit.
func isHexDigit(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// isPartNameChar reports whether c may appear literally (not percent-encoded)
// in a part-name segment: unreserved, sub-delims, ':' or '@' (RFC 3986 pchar
// minus pct-encoded, which the caller handles).
func isPartNameChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	case c == '-' || c == '.' || c == '_' || c == '~':
		return true
	case c == '!' || c == '$' || c == '&' || c == '\'' || c == '(' || c == ')' ||
		c == '*' || c == '+' || c == ',' || c == ';' || c == '=':
		return true
	case c == ':' || c == '@':
		return true
	}
	return false
}

// NormalizePartName converts a part name to its normalized form.
// This involves cleaning the path and ensuring proper formatting.
func NormalizePartName(name string) string {
	// Clean the path
	cleaned := path.Clean(name)

	// Ensure leading slash
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}

	return cleaned
}

// ResolvePartName resolves a relative URI against a base part name.
// Relationship targets are URIs, not plain paths: a fragment ("part.xml#id")
// addresses content inside the part and is stripped, and percent-escapes
// ("Some%20Image.png") are decoded so the result matches the literal part
// name stored in the package. An undecodable escape sequence leaves the
// target as-is rather than failing resolution outright.
func ResolvePartName(base, relative string) string {
	if i := strings.IndexByte(relative, '#'); i >= 0 {
		relative = relative[:i]
	}
	if decoded, err := url.PathUnescape(relative); err == nil {
		relative = decoded
	}

	if strings.HasPrefix(relative, "/") {
		return NormalizePartName(relative)
	}

	// Get the directory of the base part
	baseDir := path.Dir(base)

	// Join and normalize
	return NormalizePartName(path.Join(baseDir, relative))
}
