package ccharvest

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"strings"
)

var zipMagic = []byte{'P', 'K', 0x03, 0x04}

// ErrNotOOXML marks a payload that is not a usable OPC/OOXML package: it does
// not start with the zip local-file-header magic, does not open as a zip, or
// carries no [Content_Types].xml part. A dead origin that answers a live
// refetch with an HTML error page or a login redirect (HTTP 200, non-file
// body) fails this check, so the harvester can reject it at the fetch stage
// instead of feeding garbage to the library Open.
var ErrNotOOXML = errors.New("payload is not a valid OOXML package")

// openOOXMLPackage validates the OPC envelope of payload — zip magic, a
// readable central directory, and the presence of [Content_Types].xml — and
// returns the opened zip reader for further inspection. Every failure wraps
// ErrNotOOXML so callers can classify a non-package body uniformly.
func openOOXMLPackage(payload []byte) (*zip.Reader, error) {
	if !bytes.HasPrefix(payload, zipMagic) {
		return nil, fmt.Errorf("%w: no zip local file header", ErrNotOOXML)
	}
	zr, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return nil, fmt.Errorf("%w: zip: %v", ErrNotOOXML, err)
	}
	for _, f := range zr.File {
		if strings.TrimPrefix(f.Name, "/") == "[Content_Types].xml" {
			return zr, nil
		}
	}
	return nil, fmt.Errorf("%w: no [Content_Types].xml", ErrNotOOXML)
}

// ValidateOOXMLPackage reports whether payload is a syntactically valid OPC
// package: it starts with the zip local-file-header magic, opens as a zip, and
// contains the [Content_Types].xml part. It deliberately does NOT require a
// recognizable main part, so it accepts any OPC package while still rejecting
// the non-zip bodies (HTML error pages, login redirects) a dead origin returns.
// A non-nil result wraps ErrNotOOXML.
func ValidateOOXMLPackage(payload []byte) error {
	_, err := openOOXMLPackage(payload)
	return err
}

// ClassifyOOXML validates that payload is an OPC package and reports its real
// document type ("pptx", "xlsx" or "docx") from the parts it contains, so a
// mislabeled server MIME cannot file a document under the wrong corpus
// directory. It returns an error when the payload is not a usable OOXML
// package (wrong magic, unreadable zip, no [Content_Types].xml, or a missing
// or ambiguous main part).
func ClassifyOOXML(payload []byte) (string, error) {
	zr, err := openOOXMLPackage(payload)
	if err != nil {
		return "", err
	}
	kinds := make(map[string]bool)
	for _, f := range zr.File {
		name := strings.TrimPrefix(f.Name, "/")
		switch {
		case strings.HasPrefix(name, "word/document") && strings.HasSuffix(name, ".xml"):
			kinds["docx"] = true
		case strings.HasPrefix(name, "xl/workbook") && strings.HasSuffix(name, ".xml"):
			kinds["xlsx"] = true
		case name == "ppt/presentation.xml":
			kinds["pptx"] = true
		}
	}
	if len(kinds) == 0 {
		return "", errors.New("zip has no recognizable OOXML main part")
	}
	if len(kinds) > 1 {
		return "", fmt.Errorf("zip has ambiguous main parts: %v", SortedKinds(kinds))
	}
	for k := range kinds {
		return k, nil
	}
	return "", errors.New("unreachable")
}

// SortedKinds returns the recognized document kinds present in the set, in a
// stable order.
func SortedKinds(kinds map[string]bool) []string {
	out := make([]string, 0, len(kinds))
	for _, k := range []string{"docx", "pptx", "xlsx"} {
		if kinds[k] {
			out = append(out, k)
		}
	}
	return out
}
