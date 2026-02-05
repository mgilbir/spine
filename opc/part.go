package opc

import (
	"path"
	"strings"
)

// Part represents a part within an OPC package.
// Parts are the fundamental units of content within a package.
type Part struct {
	// Name is the URI path of the part (e.g., "/ppt/presentation.xml").
	Name string

	// ContentType is the MIME type of the part content.
	ContentType string

	// Relationships contains the relationships defined for this part.
	Relationships []*Relationship

	// Data holds the raw content of the part.
	Data []byte
}

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
func ResolvePartName(base, relative string) string {
	if strings.HasPrefix(relative, "/") {
		return NormalizePartName(relative)
	}

	// Get the directory of the base part
	baseDir := path.Dir(base)

	// Join and normalize
	return NormalizePartName(path.Join(baseDir, relative))
}

// Extension returns the file extension of the part name.
func (p *Part) Extension() string {
	return strings.ToLower(path.Ext(p.Name))
}

// GetRelationshipsByType returns all relationships with the specified type.
func (p *Part) GetRelationshipsByType(relType string) []*Relationship {
	var result []*Relationship
	for _, rel := range p.Relationships {
		if rel.Type == relType {
			result = append(result, rel)
		}
	}
	return result
}

// GetRelationshipByID returns the relationship with the specified ID, or nil.
func (p *Part) GetRelationshipByID(id string) *Relationship {
	for _, rel := range p.Relationships {
		if rel.ID == id {
			return rel
		}
	}
	return nil
}
