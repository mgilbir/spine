package opc

import (
	"net/url"
	"path"
	"strings"
)

// ValidatePartName checks if a part name conforms to OPC naming rules.
// Part names must:
// - Start with a forward slash
// - Not end with a forward slash
// - Not contain empty segments
// - Not contain certain reserved characters
// - Be case-insensitive unique
func ValidatePartName(name string) error {
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
