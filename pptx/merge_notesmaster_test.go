package pptx

import (
	"bytes"
	"strings"
	"testing"

	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
)

const testNotesMasterXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:notesMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld><p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/></p:notesMaster>`

// addNotesMaster injects a notes master (its part, a theme, and the theme
// relationship) into a created deck, so the merge tests have a source deck that
// carries one — Create() does not add a notes master.
func addNotesMaster(p *Presentation) {
	const nmName = "/ppt/notesMasters/notesMaster1.xml"
	const themeName = "/ppt/theme/theme2.xml"
	p.otherParts[nmName] = &coxml.RawPart{
		ContentType: opc.ContentTypeNotesMaster,
		Data:        []byte(testNotesMasterXML),
	}
	p.themeData[themeName] = defaultThemeXML()
	p.relationships[nmName] = []*opc.Relationship{{
		ID:         "rId1",
		Type:       opc.RelTypeTheme,
		Target:     "../theme/theme2.xml",
		TargetMode: opc.TargetModeInternal,
	}}
}

func countNotesMasterParts(p *Presentation) int {
	n := 0
	for name := range p.otherParts {
		if strings.HasPrefix(name, "/ppt/notesMasters/") && strings.HasSuffix(name, ".xml") {
			n++
		}
	}
	return n
}

// TestExtractSlidesCarriesNotesMaster confirms ExtractSlides carries the source
// deck's notes master — part, theme, presentation relationship, and
// notesMasterIdLst entry — and that the saved deck reopens with it intact and
// validates clean.
func TestExtractSlidesCarriesNotesMaster(t *testing.T) {
	src := buildDeck(t, []string{"One", "Two"})
	addNotesMaster(src)

	out, err := src.ExtractSlides([]int{0, 1})
	if err != nil {
		t.Fatalf("ExtractSlides: %v", err)
	}

	if got := out.notesMasterPartName(); got == "" {
		t.Fatal("extracted deck has no notes master")
	}
	if out.presentation.NotesMasterIDs == nil || len(out.presentation.NotesMasterIDs.NotesMasterID) != 1 {
		t.Fatalf("extracted deck notesMasterIdLst = %+v, want one entry", out.presentation.NotesMasterIDs)
	}

	data, err := out.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	assertZipHasPrefix(t, data, "ppt/notesMasters/")

	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if re.notesMasterPartName() == "" {
		t.Fatal("reopened deck lost the notes master")
	}
	if re.presentation.NotesMasterIDs == nil || len(re.presentation.NotesMasterIDs.NotesMasterID) != 1 {
		t.Fatalf("reopened notesMasterIdLst = %+v, want one entry", re.presentation.NotesMasterIDs)
	}
	rid := re.presentation.NotesMasterIDs.NotesMasterID[0].RID
	if !relIDSet(re.relationships[presentationPartName])[rid] {
		t.Fatalf("notesMasterId %q has no matching presentation relationship", rid)
	}
	if rep := re.Validate(); rep.HasErrors() {
		t.Fatalf("reopened deck fails validation: %v", rep)
	}
}

// TestAppendSlidesFromCarriesNotesMaster confirms AppendSlidesFrom carries the
// source notes master when the destination has none, and does not duplicate when
// the destination already has one.
func TestAppendSlidesFromCarriesNotesMaster(t *testing.T) {
	src := buildDeck(t, []string{"A"})
	addNotesMaster(src)

	dst := buildDeck(t, []string{"B"})
	if err := dst.AppendSlidesFrom(src); err != nil {
		t.Fatalf("AppendSlidesFrom: %v", err)
	}
	if dst.notesMasterPartName() == "" {
		t.Fatal("destination did not receive the notes master")
	}
	if n := len(dst.presentation.NotesMasterIDs.NotesMasterID); n != 1 {
		t.Fatalf("notesMasterIdLst has %d entries, want 1", n)
	}

	// Appending a second source that also has a notes master must not add a
	// duplicate (a deck carries at most one).
	src2 := buildDeck(t, []string{"C"})
	addNotesMaster(src2)
	if err := dst.AppendSlidesFrom(src2); err != nil {
		t.Fatalf("AppendSlidesFrom (second): %v", err)
	}
	if n := len(dst.presentation.NotesMasterIDs.NotesMasterID); n != 1 {
		t.Fatalf("after second append notesMasterIdLst has %d entries, want 1", n)
	}
	if n := countNotesMasterParts(dst); n != 1 {
		t.Fatalf("destination carries %d notes master parts, want 1", n)
	}

	// The merged deck must save, reopen, and validate clean.
	data, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if re.notesMasterPartName() == "" {
		t.Fatal("reopened merged deck lost the notes master")
	}
	if rep := re.Validate(); rep.HasErrors() {
		t.Fatalf("reopened merged deck fails validation: %v", rep)
	}
}
