package ccharvest

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestEncodeDNSQuery(t *testing.T) {
	got, err := EncodeDNSQuery("example.com")
	if err != nil {
		t.Fatal(err)
	}
	want := append(
		[]byte{0x00, 0x00, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		append([]byte("\x07example\x03com\x00"), 0x00, 0x01, 0x00, 0x01)...)
	if !bytes.Equal(got, want) {
		t.Errorf("query = % x, want % x", got, want)
	}

	for _, bad := range []string{"", ".", "a..b"} {
		if _, err := EncodeDNSQuery(bad); err == nil {
			t.Errorf("EncodeDNSQuery(%q): expected error", bad)
		}
	}
}

// buildDNSResponse crafts a response with the given rcode and A-record
// addresses, using a compression pointer for the answer names (as real
// resolvers do).
func buildDNSResponse(t *testing.T, rcode int, addrs ...netip.Addr) []byte {
	t.Helper()
	var b bytes.Buffer
	hdr := make([]byte, 12)
	hdr[2] = 0x81 // QR + RD
	hdr[3] = 0x80 | byte(rcode&0x0F)
	binary.BigEndian.PutUint16(hdr[4:6], 1) // QDCOUNT
	binary.BigEndian.PutUint16(hdr[6:8], uint16(len(addrs)))
	b.Write(hdr)
	b.WriteString("\x07example\x03com\x00") // question name at offset 12
	b.Write([]byte{0x00, 0x01, 0x00, 0x01}) // QTYPE, QCLASS
	for _, a := range addrs {
		b.Write([]byte{0xC0, 0x0C}) // pointer to offset 12
		if a.Is4() {
			b.Write([]byte{0x00, 0x01, 0x00, 0x01}) // TYPE A, CLASS IN
		} else {
			b.Write([]byte{0x00, 0x1C, 0x00, 0x01}) // TYPE AAAA, CLASS IN
		}
		b.Write([]byte{0x00, 0x00, 0x00, 0x3C}) // TTL
		raw := a.AsSlice()
		rd := make([]byte, 2)
		binary.BigEndian.PutUint16(rd, uint16(len(raw)))
		b.Write(rd)
		b.Write(raw)
	}
	return b.Bytes()
}

func TestParseDNSResponse(t *testing.T) {
	addr := netip.MustParseAddr("93.184.216.34")
	msg := buildDNSResponse(t, 0, addr)
	rcode, addrs, err := ParseDNSResponse(msg)
	if err != nil {
		t.Fatal(err)
	}
	if rcode != 0 || len(addrs) != 1 || addrs[0] != addr {
		t.Errorf("got rcode=%d addrs=%v", rcode, addrs)
	}

	// NXDOMAIN with no answers.
	msg = buildDNSResponse(t, 3)
	rcode, addrs, err = ParseDNSResponse(msg)
	if err != nil {
		t.Fatal(err)
	}
	if rcode != 3 || len(addrs) != 0 {
		t.Errorf("got rcode=%d addrs=%v", rcode, addrs)
	}

	// Truncated packets must error, not panic.
	for cut := 1; cut < len(msg); cut += 3 {
		if _, _, err := ParseDNSResponse(msg[:cut]); err == nil && cut < 12 {
			t.Errorf("ParseDNSResponse on %d-byte prefix: expected error", cut)
		}
	}
}

func TestClassifyAnswer(t *testing.T) {
	tests := []struct {
		name  string
		rcode int
		addrs []netip.Addr
		want  Verdict
	}{
		{"allow", 0, []netip.Addr{netip.MustParseAddr("93.184.216.34")}, VerdictAllow},
		{"blocked v4", 0, []netip.Addr{netip.MustParseAddr("0.0.0.0")}, VerdictBlocked},
		{"blocked v6", 0, []netip.Addr{netip.MustParseAddr("::")}, VerdictBlocked},
		{"blocked among allowed", 0, []netip.Addr{netip.MustParseAddr("1.2.3.4"), netip.MustParseAddr("0.0.0.0")}, VerdictBlocked},
		{"nxdomain", 3, nil, VerdictDead},
		{"servfail", 2, nil, VerdictDead},
		{"no answers", 0, nil, VerdictDead},
	}
	for _, tt := range tests {
		if got := ClassifyAnswer(tt.rcode, tt.addrs); got != tt.want {
			t.Errorf("%s: ClassifyAnswer = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestHostGateCachesVerdicts(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if ct := r.Header.Get("Content-Type"); ct != "application/dns-message" {
			t.Errorf("Content-Type = %q", ct)
		}
		body, _ := io.ReadAll(r.Body)
		if len(body) < 12 {
			t.Errorf("query too short: %d bytes", len(body))
		}
		w.Header().Set("Content-Type", "application/dns-message")
		_, _ = w.Write(buildDNSResponse(t, 0, netip.MustParseAddr("0.0.0.0")))
	}))
	defer srv.Close()

	g := NewHostGate(srv.Client(), srv.URL)
	for i := 0; i < 3; i++ {
		v, err := g.Check(context.Background(), "Blocked.example.")
		if err != nil {
			t.Fatal(err)
		}
		if v != VerdictBlocked {
			t.Fatalf("verdict = %v, want blocked", v)
		}
	}
	if calls != 1 {
		t.Errorf("resolver called %d times, want 1 (cache miss only)", calls)
	}
}
