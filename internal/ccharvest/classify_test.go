package ccharvest

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"syscall"
	"testing"
)

// minimalXLSX builds a tiny but structurally valid OOXML (xlsx) package: the
// zip magic, a [Content_Types].xml part, and an xl/workbook.xml main part.
func minimalXLSX(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	write := func(name, body string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	write("[Content_Types].xml", `<?xml version="1.0"?><Types/>`)
	write("xl/workbook.xml", `<?xml version="1.0"?><workbook/>`)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestValidateOOXMLPackage(t *testing.T) {
	valid := minimalXLSX(t)

	// A zip with no [Content_Types].xml is not an OPC package.
	var noCT bytes.Buffer
	zw := zip.NewWriter(&noCT)
	if w, err := zw.Create("hello.txt"); err != nil {
		t.Fatal(err)
	} else if _, err := w.Write([]byte("hi")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		payload []byte
		wantErr bool
	}{
		{"valid xlsx", valid, false},
		{"html error page", []byte("<!DOCTYPE html><html><body>404 Not Found</body></html>"), true},
		{"login redirect body", []byte("<html><head><title>Sign in</title></head></html>"), true},
		{"empty", nil, true},
		{"truncated zip magic", []byte("PK\x03"), true},
		{"zip without content types", noCT.Bytes(), true},
	}
	for _, tt := range tests {
		err := ValidateOOXMLPackage(tt.payload)
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: ValidateOOXMLPackage err=%v, wantErr=%v", tt.name, err, tt.wantErr)
		}
		if err != nil && !errors.Is(err, ErrNotOOXML) {
			t.Errorf("%s: error does not wrap ErrNotOOXML: %v", tt.name, err)
		}
	}

	// ClassifyOOXML still recognizes the real type through the shared helper.
	if kind, err := ClassifyOOXML(valid); err != nil || kind != "xlsx" {
		t.Errorf("ClassifyOOXML(valid) = (%q,%v), want (xlsx,nil)", kind, err)
	}
}

// timeoutErr is a net.Error reporting a timeout, standing in for a dial/read
// deadline hit.
type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestClassifyFetchError(t *testing.T) {
	connRefused := &net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}
	connReset := &net.OpError{Op: "read", Err: os.NewSyscallError("read", syscall.ECONNRESET)}
	deadHost := &net.OpError{Op: "dial", Err: &net.DNSError{Err: "no such host", Name: "gone.example", IsNotFound: true}}
	servfail := &net.DNSError{Err: "server misbehaving", Name: "flaky.example", IsTemporary: true}

	tests := []struct {
		name    string
		err     error
		hint    bool
		wantDsp FetchDisposition
		wantSig string
	}{
		{"http 404", &HTTPStatusError{Code: 404, Status: "404 Not Found"}, false, DispPermanent, "fetch:http-404"},
		{"http 410", &HTTPStatusError{Code: 410, Status: "410 Gone"}, false, DispPermanent, "fetch:http-410"},
		{"http 401", &HTTPStatusError{Code: 401, Status: "401 Unauthorized"}, false, DispPermanent, "fetch:http-401"},
		{"live 403", &HTTPStatusError{Code: 403, Status: "403 Forbidden"}, false, DispPermanent, "fetch:http-403"},
		{"warc 403 throttle", &HTTPStatusError{Code: 403, Status: "403 Forbidden"}, true, DispTransient, "fetch:http-403"},
		{"http 429", &HTTPStatusError{Code: 429, Status: "429 Too Many Requests"}, false, DispTransient, "fetch:http-429"},
		{"http 503", &HTTPStatusError{Code: 503, Status: "503 Service Unavailable"}, false, DispTransient, "fetch:http-503"},
		{"dead host dns", deadHost, false, DispPermanent, "fetch:dns"},
		{"dns servfail", servfail, false, DispTransient, "fetch:dns-temp"},
		{"conn refused", connRefused, false, DispPermanent, "fetch:conn-refused"},
		{"conn reset", connReset, true, DispTransient, "fetch:conn-reset"},
		{"deadline timeout", context.DeadlineExceeded, false, DispTransient, "fetch:timeout"},
		{"net timeout", timeoutErr{}, false, DispTransient, "fetch:timeout"},
		{"gate dead", ErrGateDead, false, DispPermanent, "fetch:dns"},
		{"gate blocked", ErrGateBlocked, false, DispPermanent, "fetch:blocked"},
		{"too large", ErrTooLarge, false, DispPermanent, "fetch:too-large"},
		{"not ooxml", ValidateOOXMLPackage([]byte("<html>nope</html>")), false, DispPermanent, "fetch:not-ooxml"},
		{"unknown transient hint", errors.New("warc: got 3 bytes, want 4"), true, DispTransient, "fetch:transient"},
		{"unknown permanent hint", errors.New("warc decode: rotten record"), false, DispPermanent, "fetch:terminal"},
	}
	for _, tt := range tests {
		dsp, sig := ClassifyFetchError(tt.err, tt.hint)
		if dsp != tt.wantDsp || sig != tt.wantSig {
			t.Errorf("%s: ClassifyFetchError = (%v,%q), want (%v,%q)",
				tt.name, dsp, sig, tt.wantDsp, tt.wantSig)
		}
	}
}
