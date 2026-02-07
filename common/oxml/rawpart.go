package oxml

import "strings"

// RawPart stores a part that is preserved as raw bytes for round-trip fidelity.
type RawPart struct {
	ContentType string
	Data        []byte
}

// RelsPathToSourcePart converts a .rels path to its source part path.
// For example, "/ppt/_rels/presentation.xml.rels" becomes "/ppt/presentation.xml".
func RelsPathToSourcePart(relsPath string) string {
	if !strings.HasSuffix(relsPath, ".rels") {
		return relsPath
	}
	path := relsPath[:len(relsPath)-5]
	path = strings.Replace(path, "/_rels/", "/", 1)
	path = strings.Replace(path, "_rels/", "", 1)
	return path
}
