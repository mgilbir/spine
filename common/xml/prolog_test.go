package xml

import "testing"

func TestCaptureProlog(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		decl    string
		sep     string
		trailer string
	}{
		{
			name: "canonical CRLF",
			data: "<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\r\n<root/>",
			decl: "<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>",
			sep:  "\r\n",
		},
		{
			name: "LibreOffice LF no standalone",
			data: "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<root/>",
			decl: "<?xml version=\"1.0\" encoding=\"UTF-8\"?>",
			sep:  "\n",
		},
		{
			name: "bare CR",
			data: "<?xml version=\"1.0\"?>\r<root/>",
			decl: "<?xml version=\"1.0\"?>",
			sep:  "\r",
		},
		{
			name: "no declaration",
			data: "<root/>",
		},
		{
			name:    "trailing newline",
			data:    "<?xml version=\"1.0\"?>\r\n<root></root>\r\n",
			decl:    "<?xml version=\"1.0\"?>",
			sep:     "\r\n",
			trailer: "\r\n",
		},
		{
			name: "BOM included in declaration",
			data: "\xef\xbb\xbf<?xml version=\"1.0\"?>\r\n<root/>",
			decl: "\xef\xbb\xbf<?xml version=\"1.0\"?>",
			sep:  "\r\n",
		},
		{
			name: "no separator",
			data: "<?xml version=\"1.0\"?><root/>",
			decl: "<?xml version=\"1.0\"?>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := CaptureProlog([]byte(tt.data))
			if !p.Captured {
				t.Fatal("Captured = false")
			}
			if p.Decl != tt.decl {
				t.Errorf("Decl = %q, want %q", p.Decl, tt.decl)
			}
			if p.Sep != tt.sep {
				t.Errorf("Sep = %q, want %q", p.Sep, tt.sep)
			}
			if p.Trailer != tt.trailer {
				t.Errorf("Trailer = %q, want %q", p.Trailer, tt.trailer)
			}
		})
	}
}

func TestWritePrologDefault(t *testing.T) {
	b := NewBuilder()
	b.WriteProlog(Prolog{})
	want := "<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\r\n"
	if got := b.String(); got != want {
		t.Errorf("uncaptured prolog = %q, want default header %q", got, want)
	}
}

func TestWritePrologRoundTrip(t *testing.T) {
	src := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<root/>\n"
	p := CaptureProlog([]byte(src))
	b := NewBuilder()
	b.RegisterNamespace("", "")
	b.WriteProlog(p)
	b.EmptyElement("", "root")
	b.WriteTrailer(p)
	if got := b.String(); got != src {
		t.Errorf("round trip = %q, want %q", got, src)
	}
}
