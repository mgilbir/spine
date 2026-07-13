package oxml

import "strings"

// RawPart stores a part that is preserved as raw bytes for round-trip fidelity.
type RawPart struct {
	ContentType string
	Data        []byte
}

// RelsPathToSourcePart converts a .rels path to its source part path.
// For example, "/ppt/_rels/presentation.xml.rels" becomes "/ppt/presentation.xml".
// Per OPC, the .rels part lives in a "_rels" directory next to its source part,
// so only that single directory component is removed; a folder name that merely
// ends in "_rels" (e.g. "/custom_rels/...") is left intact.
func RelsPathToSourcePart(relsPath string) string {
	if !strings.HasSuffix(relsPath, ".rels") {
		return relsPath
	}
	path := relsPath[:len(relsPath)-len(".rels")]
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return path
	}
	dir, name := path[:i], path[i+1:]
	if dir == "_rels" {
		// Package-level rels without a leading slash: "_rels/.rels".
		return name
	}
	if strings.HasSuffix(dir, "/_rels") {
		// Drop the "_rels" component, keeping its parent (including the
		// trailing slash): "/ppt/_rels" -> "/ppt/".
		return dir[:len(dir)-len("_rels")] + name
	}
	return path
}
