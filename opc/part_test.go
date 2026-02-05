package opc

import (
	"testing"
)

func TestValidatePartName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		// Valid part names
		{name: "simple path", input: "/document.xml", wantErr: nil},
		{name: "nested path", input: "/ppt/slides/slide1.xml", wantErr: nil},
		{name: "deep nesting", input: "/a/b/c/d/e/f.xml", wantErr: nil},
		{name: "with extension", input: "/content.txt", wantErr: nil},
		{name: "multiple extensions", input: "/file.tar.gz", wantErr: nil},
		{name: "underscore in name", input: "/_rels/.rels", wantErr: nil},
		{name: "hyphen in name", input: "/my-file.xml", wantErr: nil},
		{name: "numbers in name", input: "/slide123.xml", wantErr: nil},

		// Invalid part names
		{name: "empty string", input: "", wantErr: ErrInvalidPartName},
		{name: "no leading slash", input: "document.xml", wantErr: ErrInvalidPartName},
		{name: "trailing slash", input: "/folder/", wantErr: ErrInvalidPartName},
		{name: "double slash", input: "/folder//file.xml", wantErr: ErrInvalidPartName},
		{name: "dot segment", input: "/folder/./file.xml", wantErr: ErrInvalidPartName},
		{name: "dotdot segment", input: "/folder/../file.xml", wantErr: ErrInvalidPartName},
		{name: "only slash", input: "/", wantErr: ErrInvalidPartName},
		{name: "empty segment", input: "/a//b", wantErr: ErrInvalidPartName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePartName(tt.input)
			if err != tt.wantErr {
				t.Errorf("ValidatePartName(%q) = %v, want %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestNormalizePartName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "already normalized", input: "/document.xml", want: "/document.xml"},
		{name: "no leading slash", input: "document.xml", want: "/document.xml"},
		{name: "double slashes", input: "//document.xml", want: "/document.xml"},
		{name: "dot in path", input: "/folder/./file.xml", want: "/folder/file.xml"},
		{name: "dotdot in path", input: "/folder/sub/../file.xml", want: "/folder/file.xml"},
		{name: "complex path", input: "a/b/../c/./d.xml", want: "/a/c/d.xml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizePartName(tt.input)
			if got != tt.want {
				t.Errorf("NormalizePartName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolvePartName(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		relative string
		want     string
	}{
		{
			name:     "absolute target",
			base:     "/ppt/presentation.xml",
			relative: "/ppt/slides/slide1.xml",
			want:     "/ppt/slides/slide1.xml",
		},
		{
			name:     "relative target same dir",
			base:     "/ppt/presentation.xml",
			relative: "presProps.xml",
			want:     "/ppt/presProps.xml",
		},
		{
			name:     "relative target subdir",
			base:     "/ppt/presentation.xml",
			relative: "slides/slide1.xml",
			want:     "/ppt/slides/slide1.xml",
		},
		{
			name:     "relative target parent dir",
			base:     "/ppt/slides/slide1.xml",
			relative: "../presentation.xml",
			want:     "/ppt/presentation.xml",
		},
		{
			name:     "root base",
			base:     "/",
			relative: "docProps/core.xml",
			want:     "/docProps/core.xml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolvePartName(tt.base, tt.relative)
			if got != tt.want {
				t.Errorf("ResolvePartName(%q, %q) = %q, want %q", tt.base, tt.relative, got, tt.want)
			}
		})
	}
}

func TestPart_Extension(t *testing.T) {
	tests := []struct {
		name     string
		partName string
		want     string
	}{
		{name: "xml extension", partName: "/document.xml", want: ".xml"},
		{name: "rels extension", partName: "/_rels/.rels", want: ".rels"},
		{name: "png extension", partName: "/media/image1.PNG", want: ".png"},
		{name: "no extension", partName: "/noext", want: ""},
		{name: "multiple dots", partName: "/file.tar.gz", want: ".gz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Part{Name: tt.partName}
			got := p.Extension()
			if got != tt.want {
				t.Errorf("Part{Name: %q}.Extension() = %q, want %q", tt.partName, got, tt.want)
			}
		})
	}
}

func TestPart_GetRelationshipsByType(t *testing.T) {
	part := &Part{
		Name: "/ppt/presentation.xml",
		Relationships: []*Relationship{
			{ID: "rId1", Type: "http://type/slide", Target: "slides/slide1.xml"},
			{ID: "rId2", Type: "http://type/slide", Target: "slides/slide2.xml"},
			{ID: "rId3", Type: "http://type/theme", Target: "theme/theme1.xml"},
		},
	}

	slides := part.GetRelationshipsByType("http://type/slide")
	if len(slides) != 2 {
		t.Errorf("GetRelationshipsByType returned %d relationships, want 2", len(slides))
	}

	themes := part.GetRelationshipsByType("http://type/theme")
	if len(themes) != 1 {
		t.Errorf("GetRelationshipsByType returned %d relationships, want 1", len(themes))
	}

	none := part.GetRelationshipsByType("http://type/nonexistent")
	if len(none) != 0 {
		t.Errorf("GetRelationshipsByType returned %d relationships, want 0", len(none))
	}
}

func TestPart_GetRelationshipByID(t *testing.T) {
	part := &Part{
		Name: "/ppt/presentation.xml",
		Relationships: []*Relationship{
			{ID: "rId1", Type: "http://type/slide", Target: "slides/slide1.xml"},
			{ID: "rId2", Type: "http://type/slide", Target: "slides/slide2.xml"},
		},
	}

	rel := part.GetRelationshipByID("rId1")
	if rel == nil {
		t.Fatal("GetRelationshipByID returned nil for existing ID")
	}
	if rel.Target != "slides/slide1.xml" {
		t.Errorf("GetRelationshipByID returned wrong relationship, got target %q", rel.Target)
	}

	rel = part.GetRelationshipByID("rId999")
	if rel != nil {
		t.Error("GetRelationshipByID returned non-nil for non-existing ID")
	}
}
