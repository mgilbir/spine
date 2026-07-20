package oxml

import (
	"regexp"
	"strings"
)

// oleProgIDAttr matches a ProgID/progId attribute (case-insensitive, either
// spelling used by VML's o:OLEObject and PML's p:oleObj) and captures its
// value.
var oleProgIDAttr = regexp.MustCompile(`(?i)prog[Ii][Dd]="([^"]*)"`)

// activeXClassIDAttr and activeXPersistenceAttr match the classid and
// persistence attributes on an ActiveX ocx part's root element (they carry the
// ax: prefix, e.g. ax:classid); the prefix is ignored so the scan is
// prefix-agnostic.
var (
	activeXClassIDAttr     = regexp.MustCompile(`(?i)classid="([^"]*)"`)
	activeXPersistenceAttr = regexp.MustCompile(`(?i)persistence="([^"]*)"`)
)

// ExtractActiveXControlInfo returns the COM class id and persistence mode
// declared on an ActiveX control part (ax:ocx). The class id (e.g.
// "{8BD21D40-EC42-11CE-9E0D-00AA006002F3}") identifies the control server; the
// persistence value (e.g. "persistPropertyBag") names how the control's state
// is stored. Both are read best-effort from the part's root element rather than
// schema-parsed, since spine does not author the ActiveX persistence stream.
func ExtractActiveXControlInfo(rawXML []byte) (classID, persistence string) {
	if m := activeXClassIDAttr.FindSubmatch(rawXML); m != nil {
		classID = string(m[1])
	}
	if m := activeXPersistenceAttr.FindSubmatch(rawXML); m != nil {
		persistence = string(m[1])
	}
	return classID, persistence
}

// ExtractOLEProgID returns the ProgID declared for the OLE relationship relID
// in the raw XML of the part that references it, or "" when none is found. The
// ProgID names the embedded object's server (e.g. "Excel.Sheet.12",
// "Word.Document.12") and lives on the referencing element, not in the
// relationship or the object part itself.
//
// It is deliberately a best-effort scan over raw bytes rather than a schema
// parse: OLE references appear in several unrelated grammars (VML o:OLEObject,
// WordprocessingML w:object, PresentationML p:oleObj/p:embed) whose ProgID sits
// either on the same element as the relationship id or on its parent. The scan
// finds the relationship id, then takes the nearest ProgID within a small
// window around it — the last one at or before the id, else the first after it.
func ExtractOLEProgID(rawXML []byte, relID string) string {
	if relID == "" || len(rawXML) == 0 {
		return ""
	}
	xml := string(rawXML)

	// Locate the relationship id as an attribute value (r:id / r:embed / r:link
	// all end in ="relID").
	idx := strings.Index(xml, `"`+relID+`"`)
	if idx < 0 {
		return ""
	}

	const window = 512
	start := idx - window
	if start < 0 {
		start = 0
	}
	end := idx + len(relID) + 2 + window
	if end > len(xml) {
		end = len(xml)
	}
	segment := xml[start:end]
	rel := idx - start

	matches := oleProgIDAttr.FindAllStringSubmatchIndex(segment, -1)
	if len(matches) == 0 {
		return ""
	}
	chosen := matches[0]
	for _, m := range matches {
		if m[0] <= rel {
			chosen = m
		}
	}
	return segment[chosen[2]:chosen[3]]
}
