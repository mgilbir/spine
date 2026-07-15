package ccharvest

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// buildWARCRecord assembles a WARC response record around an HTTP block.
func buildWARCRecord(warcType, extraWARCHeaders, httpBlock string) []byte {
	var b bytes.Buffer
	b.WriteString("WARC/1.0\r\n")
	b.WriteString("WARC-Type: " + warcType + "\r\n")
	b.WriteString("WARC-Record-ID: <urn:uuid:00000000-0000-0000-0000-000000000000>\r\n")
	b.WriteString(extraWARCHeaders)
	fmt.Fprintf(&b, "Content-Length: %d\r\n", len(httpBlock))
	b.WriteString("\r\n")
	b.WriteString(httpBlock)
	b.WriteString("\r\n\r\n") // record separator
	return b.Bytes()
}

func httpBlockWithContentLength(payload string) string {
	return fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\nContent-Length: %d\r\n\r\n%s",
		len(payload), payload)
}

func TestExtractPayloadContentLength(t *testing.T) {
	payload := "PK\x03\x04 pretend zip bytes"
	rec := buildWARCRecord("response", "", httpBlockWithContentLength(payload))
	got, err := ExtractPayload(bytes.NewReader(rec))
	if err != nil {
		t.Fatalf("ExtractPayload: %v", err)
	}
	if string(got) != payload {
		t.Errorf("payload = %q, want %q", got, payload)
	}
}

func TestExtractPayloadChunked(t *testing.T) {
	payload := "hello chunked world"
	chunked := fmt.Sprintf("%x\r\n%s\r\n%x\r\n%s\r\n0\r\n\r\n",
		5, payload[:5], len(payload)-5, payload[5:])
	block := "HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n" + chunked
	rec := buildWARCRecord("response", "", block)
	got, err := ExtractPayload(bytes.NewReader(rec))
	if err != nil {
		t.Fatalf("ExtractPayload: %v", err)
	}
	if string(got) != payload {
		t.Errorf("payload = %q, want %q", got, payload)
	}
}

func TestExtractPayloadConnectionCloseFraming(t *testing.T) {
	// No Content-Length and no Transfer-Encoding on the HTTP response: the
	// body runs to the end of the record block. The WARC record separator
	// (\r\n\r\n) must not leak into the payload.
	payload := "body until end of record"
	block := "HTTP/1.1 200 OK\r\n\r\n" + payload
	rec := buildWARCRecord("response", "", block)
	got, err := ExtractPayload(bytes.NewReader(rec))
	if err != nil {
		t.Fatalf("ExtractPayload: %v", err)
	}
	if string(got) != payload {
		t.Errorf("payload = %q, want %q", got, payload)
	}
}

func TestExtractPayloadGzipContentEncoding(t *testing.T) {
	payload := "compressed payload bytes"
	var gzBuf bytes.Buffer
	gw := gzip.NewWriter(&gzBuf)
	if _, err := gw.Write([]byte(payload)); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	block := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Encoding: gzip\r\nContent-Length: %d\r\n\r\n%s",
		gzBuf.Len(), gzBuf.String())
	rec := buildWARCRecord("response", "", block)
	got, err := ExtractPayload(bytes.NewReader(rec))
	if err != nil {
		t.Fatalf("ExtractPayload: %v", err)
	}
	if string(got) != payload {
		t.Errorf("payload = %q, want %q", got, payload)
	}
}

func TestExtractPayloadTruncated(t *testing.T) {
	rec := buildWARCRecord("response", "WARC-Truncated: length\r\n",
		httpBlockWithContentLength("partial"))
	if _, err := ExtractPayload(bytes.NewReader(rec)); !errors.Is(err, ErrTruncated) {
		t.Errorf("err = %v, want ErrTruncated", err)
	}
	// An empty-valued WARC-Truncated header must also be treated as truncated.
	rec = buildWARCRecord("response", "WARC-Truncated: \r\n",
		httpBlockWithContentLength("partial"))
	if _, err := ExtractPayload(bytes.NewReader(rec)); !errors.Is(err, ErrTruncated) {
		t.Errorf("err = %v, want ErrTruncated for empty-valued header", err)
	}
}

func TestExtractPayloadNotResponse(t *testing.T) {
	rec := buildWARCRecord("request", "", "GET / HTTP/1.1\r\n\r\n")
	if _, err := ExtractPayload(bytes.NewReader(rec)); !errors.Is(err, ErrNotResponse) {
		t.Errorf("err = %v, want ErrNotResponse", err)
	}
}

func TestExtractPayloadNotWARC(t *testing.T) {
	if _, err := ExtractPayload(strings.NewReader("HTTP/1.1 200 OK\r\n\r\n")); err == nil {
		t.Error("expected error for non-WARC input")
	}
}

func TestDecodeRecordGzipMember(t *testing.T) {
	payload := "record payload"
	rec := buildWARCRecord("response", "", httpBlockWithContentLength(payload))
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(rec); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := DecodeRecord(buf.Bytes())
	if err != nil {
		t.Fatalf("DecodeRecord: %v", err)
	}
	if string(got) != payload {
		t.Errorf("payload = %q, want %q", got, payload)
	}
}

// buildZip creates an in-memory zip holding the named (empty) parts.
func buildZip(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, n := range names {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("<x/>")); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestClassifyOOXML(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    string
		wantErr bool
	}{
		{"docx", buildZip(t, "[Content_Types].xml", "word/document.xml"), "docx", false},
		{"docx alt main part", buildZip(t, "[Content_Types].xml", "word/document2.xml"), "docx", false},
		{"xlsx", buildZip(t, "[Content_Types].xml", "xl/workbook.xml"), "xlsx", false},
		{"pptx", buildZip(t, "[Content_Types].xml", "ppt/presentation.xml"), "pptx", false},
		{"leading slash part names", buildZip(t, "/[Content_Types].xml", "/xl/workbook.xml"), "xlsx", false},
		{"no content types", buildZip(t, "word/document.xml"), "", true},
		{"no main part", buildZip(t, "[Content_Types].xml", "docProps/core.xml"), "", true},
		{"ambiguous", buildZip(t, "[Content_Types].xml", "word/document.xml", "xl/workbook.xml"), "", true},
		{"not a zip", []byte("<!DOCTYPE html><html></html>"), "", true},
		{"ole encrypted container", append([]byte{0xD0, 0xCF, 0x11, 0xE0}, make([]byte, 64)...), "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ClassifyOOXML(tt.payload)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ClassifyOOXML = %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ClassifyOOXML: %v", err)
			}
			if got != tt.want {
				t.Errorf("ClassifyOOXML = %q, want %q", got, tt.want)
			}
		})
	}
}
