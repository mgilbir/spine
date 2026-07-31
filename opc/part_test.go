package opc

import (
	"bytes"
	"errors"
	"strings"
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

// Part-type tests removed along with the dead Part abstraction (C121).

// TestResolvePartName_URITargets verifies that relationship targets are
// treated as URIs: percent-escapes are decoded and fragments stripped before
// resolution (C208).
func TestResolvePartName_URITargets(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		relative string
		want     string
	}{
		{
			name:     "percent-encoded space",
			base:     "/ppt/slides/slide1.xml",
			relative: "../media/Some%20Image.png",
			want:     "/ppt/media/Some Image.png",
		},
		{
			name:     "fragment stripped",
			base:     "/word/document.xml",
			relative: "styles.xml#heading1",
			want:     "/word/styles.xml",
		},
		{
			name:     "percent-encoded and fragment",
			base:     "/word/document.xml",
			relative: "My%20File.xml#frag",
			want:     "/word/My File.xml",
		},
		{
			name:     "absolute target with escape",
			base:     "/",
			relative: "/docProps/My%20Props.xml",
			want:     "/docProps/My Props.xml",
		},
		{
			name:     "invalid escape left as-is",
			base:     "/",
			relative: "a%2zb.xml",
			want:     "/a%2zb.xml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolvePartName(tt.base, tt.relative); got != tt.want {
				t.Errorf("ResolvePartName(%q, %q) = %q, want %q", tt.base, tt.relative, got, tt.want)
			}
		})
	}
}

// TestValidatePartName_StrictGrammar covers the OPC part-name grammar rules
// enforced for writer-side validation (C119): backslash, space, unencoded
// '%', control characters, non-ASCII bytes, encoded path separators, and
// trailing-dot segments are rejected for new parts.
func TestValidatePartName_StrictGrammar(t *testing.T) {
	valid := []string{
		"/media/image%20one.png", // encoded space
		"/a/b!$&'()*+,;=.xml",    // sub-delims
		"/a/b:@c.xml",            // ':' and '@'
		"/a/%C3%A9.xml",          // encoded non-ASCII
	}
	for _, name := range valid {
		if err := ValidatePartName(name); err != nil {
			t.Errorf("ValidatePartName(%q) = %v, want nil", name, err)
		}
	}

	invalid := []string{
		"/a b.xml",             // unencoded space
		`/a\b.xml`,             // backslash
		"/a%2.xml",             // truncated escape
		"/a%zz.xml",            // non-hex escape
		"/a%2Fb.xml",           // encoded slash
		"/a%5cb.xml",           // encoded backslash
		"/a\x01b.xml",          // control character
		"/caf\xc3\xa9.xml",     // raw non-ASCII (must be percent-encoded)
		"/dir./file.xml",       // segment ending with dot
		"/file.xml.",           // final segment ending with dot
		"/[trash]/0000.dat",    // '[' outside the grammar (preserved-only name)
		"/a<b.xml", "/a>b:xml", // reserved-ish characters outside pchar
	}
	for _, name := range invalid {
		if err := ValidatePartName(name); !errors.Is(err, ErrInvalidPartName) {
			t.Errorf("ValidatePartName(%q) = %v, want ErrInvalidPartName", name, err)
		}
	}
}

// TestWritePreservedPart_LenientNames verifies that parts carried over from a
// source package bypass the strict grammar (wild packages contain names like
// /[trash]/0000.dat that must keep round-tripping), while CreatePart rejects
// the same names for new parts.
func TestWritePreservedPart_LenientNames(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.WritePreservedPart("/[trash]/0000.dat", "", []byte{0x00}); err != nil {
		t.Fatalf("WritePreservedPart([trash] name) error = %v", err)
	}
	if err := w.WritePreservedPart("/word/media/wild name.png", "image/png", []byte("x")); err != nil {
		t.Fatalf("WritePreservedPart(space name) error = %v", err)
	}
	if _, err := w.CreatePart("/word/media/wild name2.png", "image/png", CompressionDeflate); !errors.Is(err, ErrInvalidPartName) {
		t.Errorf("CreatePart(space name) = %v, want ErrInvalidPartName", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

// The two normalizations have to agree, and each has to be idempotent.
//
// reader.go states the agreement as a fact — canonicalZipEntryName "agrees with
// NormalizePartName, which is what GetFile runs its query through" — and it was
// not true for the two spellings the normalization exists for. A backslash was
// a separator on the indexing side and an ordinary character on the query side,
// so a part stored by a producer as "word\document.xml" was indexed as
// /word/document.xml and could not be asked for under the name it was written
// with. A leading ".." was resolved on one side and left in place on the other.
//
// Stated as a property rather than as examples because the failure was in the
// spellings nobody thought to write down.
func TestPartNameNormalizationsAgree(t *testing.T) {
	inputs := []string{
		"/a/b.xml", "a/b.xml", "./a/b.xml", "a//b.xml", "a/../b.xml",
		"/a/../b.xml", "/a/b/../c.xml", "/a/../../b.xml", "../b.xml",
		"../../../etc/passwd", "/../x.xml", `word\document.xml`,
		`\word\document.xml`, `a\b/c.xml`, "/ppt/./slides/slide1.xml",
		"/", "", ".", "..", "//server/share/x.xml", "/a b/c.xml",
		"/A/B.XML", "/word/media/", "media/", "/a/b/",
	}
	for _, in := range inputs {
		norm := NormalizePartName(in)
		canon := canonicalZipEntryName(in)
		if norm != canon {
			t.Errorf("normalizations disagree on %q: NormalizePartName=%q canonicalZipEntryName=%q",
				in, norm, canon)
		}
		if again := NormalizePartName(norm); again != norm {
			t.Errorf("NormalizePartName is not idempotent on %q: %q then %q", in, norm, again)
		}
		if !strings.HasPrefix(norm, "/") {
			t.Errorf("NormalizePartName(%q) = %q, which is not rooted", in, norm)
		}
		// A name that still carries a parent segment addresses something
		// outside the package, and two such names can collide on a third.
		for _, seg := range strings.Split(norm, "/") {
			if seg == ".." {
				t.Errorf("NormalizePartName(%q) = %q, which escapes the package root", in, norm)
			}
		}
	}
}

// A part stored under a producer's backslash spelling is reachable by that
// spelling, which is the behavior the agreement above buys.
func TestGetFileAcceptsABackslashSpelling(t *testing.T) {
	pkg := createTestPackage(t,
		map[string][]byte{"/word/document.xml": []byte("<document/>")},
		map[string]string{"/word/document.xml": "application/xml"})

	r, err := NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	for _, spelling := range []string{
		"/word/document.xml", `word\document.xml`, `/word\document.xml`,
		"word/document.xml", "/word/./document.xml", "/word/../word/document.xml",
	} {
		if f := r.GetFile(spelling); f == nil {
			t.Errorf("GetFile(%q) found nothing; the same part answers to its other spellings", spelling)
		}
	}
}
