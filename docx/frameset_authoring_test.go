package docx

import (
	"bytes"
	"testing"
)

// TestSetFramesetNewPart authors a nested frameset on a created document (no
// web-settings part) and reads it back after a round-trip.
func TestSetFramesetNewPart(t *testing.T) {
	doc := Create()
	err := doc.SetFrameset(FramesetDef{
		Layout: "cols",
		Framesets: []FramesetDef{{
			Layout: "rows",
			Frames: []FrameDef{{Name: "top", Size: "*", SourceTarget: "top.html"}},
		}},
		Frames: []FrameDef{{
			Name:         "side",
			Size:         "240",
			Scrollbar:    "off",
			SourceTarget: "side.html",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	re, _ := reopen(t, doc)
	defer re.Close() //nolint:errcheck

	fs := re.Frameset()
	if fs == nil {
		t.Fatal("Frameset = nil after SetFrameset")
	}
	if fs.Layout() != "cols" {
		t.Errorf("top layout = %q, want cols", fs.Layout())
	}
	if len(fs.Framesets()) != 1 || len(fs.Frames()) != 1 {
		t.Fatalf("top-level: %d framesets, %d frames", len(fs.Framesets()), len(fs.Frames()))
	}
	nested := fs.Framesets()[0]
	if nested.Layout() != "rows" || len(nested.Frames()) != 1 {
		t.Fatalf("nested: layout %q, %d frames", nested.Layout(), len(nested.Frames()))
	}
	top := nested.Frames()[0]
	if top.Name() != "top" || top.Size() != "*" || top.SourceTarget() != "top.html" {
		t.Errorf("top frame = name %q size %q target %q", top.Name(), top.Size(), top.SourceTarget())
	}
	side := fs.Frames()[0]
	if side.Name() != "side" || side.Size() != "240" || side.Scrollbar() != "off" || side.SourceTarget() != "side.html" {
		t.Errorf("side frame = name %q size %q scrollbar %q target %q", side.Name(), side.Size(), side.Scrollbar(), side.SourceTarget())
	}
	if rep := re.Validate(); rep.HasErrors() {
		t.Errorf("Validate reported errors: %v", rep)
	}
}

// TestSetFramesetReplacesExisting replaces the frameset of an existing frameset
// document.
func TestSetFramesetReplacesExisting(t *testing.T) {
	fixture := framesetFixture(t)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.SetFrameset(FramesetDef{
		Layout: "rows",
		Frames: []FrameDef{
			{Name: "only", Size: "*", SourceTarget: "only.html"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	re, _ := reopen(t, doc)
	defer re.Close() //nolint:errcheck
	fs := re.Frameset()
	if fs == nil {
		t.Fatal("Frameset = nil after replacement")
	}
	if fs.Layout() != "rows" {
		t.Errorf("layout = %q, want rows", fs.Layout())
	}
	if len(fs.Framesets()) != 0 {
		t.Errorf("nested framesets = %d, want 0 after replacement", len(fs.Framesets()))
	}
	if len(fs.Frames()) != 1 || fs.Frames()[0].SourceTarget() != "only.html" {
		t.Fatalf("frames = %d, first target = %q", len(fs.Frames()), func() string {
			if len(fs.Frames()) > 0 {
				return fs.Frames()[0].SourceTarget()
			}
			return ""
		}())
	}
	if rep := re.Validate(); rep.HasErrors() {
		t.Errorf("Validate reported errors: %v", rep)
	}
}

// TestSetFramesetPreservesOtherSettings adds a frameset to a web-settings part
// that carries other settings and no frameset, and confirms the other settings
// survive verbatim while the frameset is inserted.
func TestSetFramesetPreservesOtherSettings(t *testing.T) {
	const ct = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
		`<Override PartName="/word/webSettings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.webSettings+xml"/>` +
		`</Types>`
	const docRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/webSettings" Target="webSettings.xml"/>` +
		`</Relationships>`
	// Note: no xmlns:r on the root — the injector must add it for frame r:ids.
	const webSettings = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:webSettings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:optimizeForBrowser/>` +
		`</w:webSettings>`
	fixture := buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml":          ct,
		"_rels/.rels":                  fixtureRootRels,
		"word/document.xml":            `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + `<w:document ` + fixtureWNS + `><w:body><w:p/></w:body></w:document>`,
		"word/_rels/document.xml.rels": docRels,
		"word/webSettings.xml":         webSettings,
	})

	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.SetFrameset(FramesetDef{
		Layout: "cols",
		Frames: []FrameDef{{Name: "main", SourceTarget: "main.html"}},
	}); err != nil {
		t.Fatal(err)
	}
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	ws, ok := zipEntry(t, saved, "word/webSettings.xml")
	if !ok {
		t.Fatal("webSettings part missing after save")
	}
	if !bytes.Contains(ws, []byte("<w:optimizeForBrowser/>")) {
		t.Errorf("optimizeForBrowser setting not preserved: %s", ws)
	}

	re, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatal(err)
	}
	defer re.Close() //nolint:errcheck
	fs := re.Frameset()
	if fs == nil || fs.Layout() != "cols" {
		t.Fatalf("Frameset = %v", fs)
	}
	if len(fs.Frames()) != 1 || fs.Frames()[0].SourceTarget() != "main.html" {
		t.Fatalf("frame source not resolved: %s", ws)
	}
	if rep := re.Validate(); rep.HasErrors() {
		t.Errorf("Validate reported errors: %v", rep)
	}
}

// TestSetFramesetInvalidLayout rejects an out-of-range layout.
func TestSetFramesetInvalidLayout(t *testing.T) {
	doc := Create()
	if err := doc.SetFrameset(FramesetDef{Layout: "diagonal"}); err == nil {
		t.Fatal("SetFrameset with invalid layout = nil error, want error")
	}
}

// TestSetFramesetInvalidScrollbar rejects an out-of-range scrollbar value.
func TestSetFramesetInvalidScrollbar(t *testing.T) {
	doc := Create()
	err := doc.SetFrameset(FramesetDef{
		Frames: []FrameDef{{Name: "f", Scrollbar: "sometimes"}},
	})
	if err == nil {
		t.Fatal("SetFrameset with invalid scrollbar = nil error, want error")
	}
}

// TestWebSettingsUnmodifiedRoundTripsByteIdentical guards that opening a
// frameset document and NOT authoring anything leaves the part byte-identical.
func TestWebSettingsUnmodifiedRoundTripsByteIdentical(t *testing.T) {
	fixture := framesetFixture(t)
	orig, _ := zipEntry(t, fixture, "word/webSettings.xml")
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	got, ok := zipEntry(t, saved, "word/webSettings.xml")
	if !ok || !bytes.Equal(orig, got) {
		t.Fatal("unmodified webSettings part not byte-identical after round-trip")
	}
}
