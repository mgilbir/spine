package ccharvest

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"strings"
)

var zipMagic = []byte{'P', 'K', 0x03, 0x04}

// ClassifyOOXML validates that payload is an OPC package and reports its real
// document type ("pptx", "xlsx" or "docx") from the parts it contains, so a
// mislabeled server MIME cannot file a document under the wrong corpus
// directory. It returns an error when the payload is not a usable OOXML
// package (wrong magic, unreadable zip, no [Content_Types].xml, or a missing
// or ambiguous main part).
func ClassifyOOXML(payload []byte) (string, error) {
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
