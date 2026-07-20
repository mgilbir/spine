package pptx

import (
	"bytes"
	"strings"
	"testing"

	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
)

const testHandoutMasterXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:handoutMaster xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld><p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/></p:handoutMaster>`

// addHandoutMaster injects a handout master (its part, a theme, and the theme
// relationship) into a created deck, so the merge tests have a source deck that
// carries one — Create() does not add a handout master.
func addHandoutMaster(p *Presentation) {
	const hmName = "/ppt/handoutMasters/handoutMaster1.xml"
	const themeName = "/ppt/theme/theme2.xml"
	p.otherParts[hmName] = &coxml.RawPart{
		ContentType: opc.ContentTypeHandoutMaster,
		Data:        []byte(testHandoutMasterXML),
	}
	p.themeData[themeName] = defaultThemeXML()
	p.relationships[hmName] = []*opc.Relationship{{
		ID:         "rId1",
		Type:       opc.RelTypeTheme,
		Target:     "../theme/theme2.xml",
		TargetMode: opc.TargetModeInternal,
	}}
}

// TestExtractSlidesCarriesHandoutMaster confirms ExtractSlides carries the
// source deck's handout master — part, theme, presentation relationship, and
// handoutMasterIdLst entry — and that the saved deck reopens with it intact and
// validates clean.
func TestExtractSlidesCarriesHandoutMaster(t *testing.T) {
	src := buildDeck(t, []string{"One", "Two"})
	addHandoutMaster(src)

	out, err := src.ExtractSlides([]int{0, 1})
	if err != nil {
		t.Fatalf("ExtractSlides: %v", err)
	}

	if got := out.handoutMasterPartName(); got == "" {
		t.Fatal("extracted deck has no handout master")
	}
	if out.presentation.HandoutMasterIDs == nil || len(out.presentation.HandoutMasterIDs.HandoutMasterID) != 1 {
		t.Fatalf("extracted deck handoutMasterIdLst = %+v, want one entry", out.presentation.HandoutMasterIDs)
	}

	data, err := out.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	assertZipHasPrefix(t, data, "ppt/handoutMasters/")

	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if re.handoutMasterPartName() == "" {
		t.Fatal("reopened deck lost the handout master")
	}
	// The handoutMasterIdLst must survive to the reopened presentation.xml and
	// point at a relationship that resolves.
	if re.presentation.HandoutMasterIDs == nil || len(re.presentation.HandoutMasterIDs.HandoutMasterID) != 1 {
		t.Fatalf("reopened handoutMasterIdLst = %+v, want one entry", re.presentation.HandoutMasterIDs)
	}
	rid := re.presentation.HandoutMasterIDs.HandoutMasterID[0].RID
	if !relIDSet(re.relationships[presentationPartName])[rid] {
		t.Fatalf("handoutMasterId %q has no matching presentation relationship", rid)
	}
	if rep := re.Validate(); rep.HasErrors() {
		t.Fatalf("reopened deck fails validation: %v", rep)
	}
}

// TestAppendSlidesFromCarriesHandoutMaster confirms AppendSlidesFrom carries the
// source handout master when the destination has none, and does not duplicate
// when the destination already has one.
func TestAppendSlidesFromCarriesHandoutMaster(t *testing.T) {
	src := buildDeck(t, []string{"A"})
	addHandoutMaster(src)

	dst := buildDeck(t, []string{"B"})
	if err := dst.AppendSlidesFrom(src); err != nil {
		t.Fatalf("AppendSlidesFrom: %v", err)
	}
	if dst.handoutMasterPartName() == "" {
		t.Fatal("destination did not receive the handout master")
	}
	if n := len(dst.presentation.HandoutMasterIDs.HandoutMasterID); n != 1 {
		t.Fatalf("handoutMasterIdLst has %d entries, want 1", n)
	}

	// Appending a second source that also has a handout master must not add a
	// duplicate (a deck carries at most one).
	src2 := buildDeck(t, []string{"C"})
	addHandoutMaster(src2)
	if err := dst.AppendSlidesFrom(src2); err != nil {
		t.Fatalf("AppendSlidesFrom (second): %v", err)
	}
	if n := len(dst.presentation.HandoutMasterIDs.HandoutMasterID); n != 1 {
		t.Fatalf("after second append handoutMasterIdLst has %d entries, want 1", n)
	}
	if n := countHandoutMasterParts(dst); n != 1 {
		t.Fatalf("destination carries %d handout master parts, want 1", n)
	}
}

func countHandoutMasterParts(p *Presentation) int {
	n := 0
	for name := range p.otherParts {
		if strings.HasPrefix(name, "/ppt/handoutMasters/") && strings.HasSuffix(name, ".xml") {
			n++
		}
	}
	return n
}
