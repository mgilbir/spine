package pptx_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/testutil"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx"
)

// unreadableFixture is a one-slide deck with its masters, layouts, theme,
// presentation/view properties, table styles and document properties — every
// entry of which the sweep below makes unreadable in turn.
func unreadableFixture(t *testing.T) []byte {
	t.Helper()
	p := pptx.Create()
	tb := p.AddSlide().AddTextBox()
	tb.SetText("slide text that must survive")
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pptx.OpenReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("intact fixture does not open: %v", err)
	}
	return data
}

func zipEntryNames(t *testing.T, pkg []byte) []string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	return names
}

// TestOpen_UnreadablePartFailsOpen: every part of the package, made present
// but unreadable, must fail the open with the read error in the chain. Before,
// an unreadable theme, presProps, viewProps, tableStyles, docProps part or
// slide-layout relationships part opened cleanly and the next save dropped it.
func TestOpen_UnreadablePartFailsOpen(t *testing.T) {
	pkg := unreadableFixture(t)
	names := zipEntryNames(t, pkg)
	for _, want := range []string{
		"ppt/theme/theme1.xml", "ppt/presProps.xml", "docProps/core.xml",
		"docProps/app.xml", "ppt/slideLayouts/_rels/slideLayout1.xml.rels",
	} {
		found := false
		for _, n := range names {
			found = found || n == want
		}
		if !found {
			t.Fatalf("fixture lacks %s; the cases below would not cover it (entries: %v)", want, names)
		}
	}
	for _, name := range names {
		t.Run(name+"/oversized", func(t *testing.T) {
			bad := testutil.WithOversizedEntry(t, pkg, name)
			_, err := pptx.OpenReader(bytes.NewReader(bad), int64(len(bad)))
			if err == nil {
				t.Fatal("open succeeded with an unreadable part")
			}
			if !errors.Is(err, opc.ErrDecompressionLimit) {
				t.Errorf("error does not wrap opc.ErrDecompressionLimit: %v", err)
			}
			if !strings.Contains(err.Error(), name) {
				t.Errorf("error does not name the unreadable part: %v", err)
			}
		})
		t.Run(name+"/badcrc", func(t *testing.T) {
			bad := testutil.WithBadCRC(t, pkg, name)
			_, err := pptx.OpenReader(bytes.NewReader(bad), int64(len(bad)))
			if !errors.Is(err, zip.ErrChecksum) {
				t.Errorf("open error = %v, want one wrapping zip.ErrChecksum", err)
			}
		})
	}
}

// TestOpen_MissingCorePropertiesStillOpens keeps the leniency for a part that
// is genuinely absent: a dangling core-properties relationship opens and saves.
func TestOpen_MissingCorePropertiesStillOpens(t *testing.T) {
	without := testutil.WithoutEntry(t, unreadableFixture(t), "docProps/core.xml")
	p, err := pptx.OpenReader(bytes.NewReader(without), int64(len(without)))
	if err != nil {
		t.Fatalf("open with core.xml absent: %v", err)
	}
	if _, err := p.SaveBytes(); err != nil {
		t.Fatalf("save with core.xml absent: %v", err)
	}
}

// TestSave_SlideUnreadableAfterOpenFailsSave: slide parts are validated at
// Open and then re-read from the source on first access and at save. When
// that re-read fails (the source was truncated or replaced underneath the open
// deck) the slide has no model, and the save must fail rather than write an
// empty slide in its place — whether the slide was left alone, only read, or
// edited (an edit lands on the empty stand-in model).
func TestSave_SlideUnreadableAfterOpenFailsSave(t *testing.T) {
	pkg := unreadableFixture(t)
	cases := []struct {
		name  string
		touch func(*pptx.Slide)
	}{
		{"untouched", func(*pptx.Slide) {}},
		{"read", func(s *pptx.Slide) { _ = s.Shapes() }},
		{"edited", func(s *pptx.Slide) { s.SetName("renamed") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := testutil.NewSwitchableReaderAt(pkg)
			p, err := pptx.OpenReader(src, src.Size())
			if err != nil {
				t.Fatal(err)
			}
			src.Fail()
			tc.touch(p.Slides()[0])
			out, err := p.SaveBytes()
			if err == nil {
				t.Fatalf("save succeeded (%d bytes) after the slide became unreadable: "+
					"the slide was regenerated empty instead of preserved", len(out))
			}
			if !errors.Is(err, testutil.ErrInjectedRead) {
				t.Errorf("save error = %v, want one wrapping the read failure", err)
			}
			if !strings.Contains(err.Error(), "/ppt/slides/slide1.xml") {
				t.Errorf("save error does not name the slide part: %v", err)
			}
		})
	}
}
