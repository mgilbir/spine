package main

import (
	"archive/zip"
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

// Sentinel errors for the terminal, non-retryable outcomes of decoding a
// WARC record. The caller uses them to bucket per-row statistics.
var (
	errNotResponse = errors.New("warc: record is not a response record")
	errTruncated   = errors.New("warc: record carries a WARC-Truncated header")
)

// extractPayload decodes a single gunzipped WARC record and returns the HTTP
// response payload it stores. It requires a WARC-Type: response record,
// rejects records that carry any WARC-Truncated header, de-chunks the HTTP
// body when it uses Transfer-Encoding: chunked, and undoes a gzip
// Content-Encoding when the origin served one.
func extractPayload(record io.Reader) ([]byte, error) {
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
		return nil, errNotResponse
	}
	if _, ok := hdr[textproto.CanonicalMIMEHeaderKey("WARC-Truncated")]; ok {
		return nil, errTruncated
	}
	blockLen, err := strconv.ParseInt(hdr.Get("Content-Length"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("warc: bad record Content-Length %q", hdr.Get("Content-Length"))
	}

	// The record block is the archived HTTP response (headers + body),
	// exactly blockLen bytes; the trailing \r\n\r\n record separator must
	// not leak into connection-close-framed bodies.
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

var zipMagic = []byte{'P', 'K', 0x03, 0x04}

// classifyOOXML validates that payload is an OPC package and reports its real
// document type ("pptx", "xlsx" or "docx") from the parts it contains, so a
// mislabeled server MIME cannot file a document under the wrong corpus
// directory. It returns an error when the payload is not a usable OOXML
// package (wrong magic, unreadable zip, no [Content_Types].xml, or a missing
// or ambiguous main part).
func classifyOOXML(payload []byte) (string, error) {
	if !bytes.HasPrefix(payload, zipMagic) {
		return "", errors.New("payload does not start with a zip local file header")
	}
	zr, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return "", fmt.Errorf("zip: %w", err)
	}
	hasContentTypes := false
	kinds := make(map[string]bool)
	for _, f := range zr.File {
		name := strings.TrimPrefix(f.Name, "/")
		switch {
		case name == "[Content_Types].xml":
			hasContentTypes = true
		case strings.HasPrefix(name, "word/document") && strings.HasSuffix(name, ".xml"):
			kinds["docx"] = true
		case strings.HasPrefix(name, "xl/workbook") && strings.HasSuffix(name, ".xml"):
			kinds["xlsx"] = true
		case name == "ppt/presentation.xml":
			kinds["pptx"] = true
		}
	}
	if !hasContentTypes {
		return "", errors.New("zip has no [Content_Types].xml")
	}
	if len(kinds) == 0 {
		return "", errors.New("zip has no recognizable OOXML main part")
	}
	if len(kinds) > 1 {
		return "", fmt.Errorf("zip has ambiguous main parts: %v", sortedKinds(kinds))
	}
	for k := range kinds {
		return k, nil
	}
	return "", errors.New("unreachable")
}

func sortedKinds(kinds map[string]bool) []string {
	out := make([]string, 0, len(kinds))
	for _, k := range []string{"docx", "pptx", "xlsx"} {
		if kinds[k] {
			out = append(out, k)
		}
	}
	return out
}
