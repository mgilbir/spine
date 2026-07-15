package ccharvest

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
)

// Sentinel errors for the terminal, non-retryable outcomes of decoding a WARC
// record. Callers use them to bucket per-row statistics.
var (
	ErrNotResponse = errors.New("warc: record is not a response record")
	ErrTruncated   = errors.New("warc: record carries a WARC-Truncated header")
)

// DecodeRecord gunzips the raw WARC record bytes (one gzip member) and returns
// the archived HTTP response payload.
func DecodeRecord(raw []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("warc: record gzip: %w", err)
	}
	gz.Multistream(false)
	return ExtractPayload(gz)
}

// ExtractPayload decodes a single gunzipped WARC record and returns the HTTP
// response payload it stores. It requires a WARC-Type: response record,
// rejects records that carry any WARC-Truncated header, de-chunks the HTTP
// body when it uses Transfer-Encoding: chunked, and undoes a gzip
// Content-Encoding when the origin served one.
func ExtractPayload(record io.Reader) ([]byte, error) {
	br := bufio.NewReader(record)
	tp := textproto.NewReader(br)

	version, err := tp.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("warc: reading version line: %w", err)
	}
	if !strings.HasPrefix(version, "WARC/") {
		return nil, fmt.Errorf("warc: unexpected version line %q", version)
	}
	hdr, err := tp.ReadMIMEHeader()
	if err != nil {
		return nil, fmt.Errorf("warc: reading record headers: %w", err)
	}
	if !strings.EqualFold(hdr.Get("WARC-Type"), "response") {
		return nil, ErrNotResponse
	}
	if _, ok := hdr[textproto.CanonicalMIMEHeaderKey("WARC-Truncated")]; ok {
		return nil, ErrTruncated
	}
	blockLen, err := strconv.ParseInt(hdr.Get("Content-Length"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("warc: bad record Content-Length %q", hdr.Get("Content-Length"))
	}

	// The record block is the archived HTTP response (headers + body),
	// exactly blockLen bytes; the trailing \r\n\r\n record separator must not
	// leak into connection-close-framed bodies.
	resp, err := http.ReadResponse(bufio.NewReader(io.LimitReader(br, blockLen)), nil)
	if err != nil {
		return nil, fmt.Errorf("warc: parsing archived HTTP response: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("warc: reading archived HTTP body: %w", err)
	}

	switch strings.ToLower(resp.Header.Get("Content-Encoding")) {
	case "", "identity", "none":
	case "gzip", "x-gzip":
		gz, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("warc: gzip content-encoding: %w", err)
		}
		body, err = io.ReadAll(gz)
		if err != nil {
			return nil, fmt.Errorf("warc: gzip content-encoding: %w", err)
		}
	default:
		return nil, fmt.Errorf("warc: unsupported Content-Encoding %q",
			resp.Header.Get("Content-Encoding"))
	}
	return body, nil
}
