package docsguard_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The godoc-first drift channel (C579).
//
// Every behavior caveat of the last wave landed in godoc and none of them
// reached the guides in docs/: AppendSlidesFrom mutating its source deck,
// out-of-range slide jumps being dropped at save time, Image.SVGData versus
// Data, DeleteSheet's cascade, Cell.Value returning time.Time and typed cached
// formula results. The previous docs audit verified that every guide snippet
// executes — which a caveat drift passes straight through, because the snippet
// still runs, it just no longer tells the whole truth.
//
// This is the cheap mechanical half of the fix: for each API whose behavior
// carries a caveat a user can be burned by, pin (a) that the guide mentions the
// API at all and (b) that the sentence-level cue for the caveat is still there.
// It cannot judge whether the prose is *good*; it does catch the guide silently
// losing the caveat, and it makes "add a row here" the obvious move when the
// next caveat is written into godoc.

// caveat is one API whose documented behavior must survive in a guide.
type caveat struct {
	// api is the symbol as the guide spells it.
	api string
	// guide is the file under docs/ that must cover it.
	guide string
	// cues are phrases that must all appear in the guide; together they are
	// the caveat, not just the feature.
	cues []string
	// why records the behavior being pinned, for whoever hits a failure.
	why string
}

var caveats = []caveat{
	{
		api:   "AppendSlidesFrom",
		guide: "pptx.md",
		cues:  []string{"may modify the source", "flushes"},
		why: "marshaling the source's slides to snapshot them flushes its pending edits, " +
			"embeds its pending media and allocates its shape ids — the source handle is " +
			"not left as the caller found it",
	},
	{
		api:   "SetHyperlinkToSlide",
		guide: "pptx.md",
		cues:  []string{"out of range", "no hyperlink is emitted"},
		why: "the slide index is resolved at save time; an index that is still out of range " +
			"then produces no hyperlink at all, with no error from the setter",
	},
	{
		api:   "SVGData",
		guide: "xlsx.md",
		cues:  []string{"raster", "nil"},
		why:   "Data() returns the raster fallback for an SVG image; SVGData() returns the SVG, or nil",
	},
	{
		api:   "DeleteSheet",
		guide: "xlsx.md",
		cues:  []string{"cascade", "defined names", "pivot"},
		why: "deletion cascades to the parts only that sheet owned, re-points sheet-scoped " +
			"defined names, and deliberately spares pivot caches",
	},
	{
		api:   "Cell.Value",
		guide: "xlsx.md",
		cues:  []string{"time.Time", "cached"},
		why: "Value() returns time.Time for date cells and the typed cached result for a " +
			"formula cell, not just strings/numbers/booleans",
	},
}

// TestGuidesCarryGodocCaveats asserts each pinned caveat is still present in
// its guide.
func TestGuidesCarryGodocCaveats(t *testing.T) {
	root := repoRoot(t)
	loaded := map[string]string{}

	for _, c := range caveats {
		text, ok := loaded[c.guide]
		if !ok {
			data, err := os.ReadFile(filepath.Join(root, "docs", c.guide))
			if err != nil {
				t.Fatalf("reading docs/%s: %v", c.guide, err)
			}
			text = string(data)
			loaded[c.guide] = text
		}
		if !strings.Contains(text, c.api) {
			t.Errorf("docs/%s never mentions %s. It is a user-facing API with a behavior "+
				"caveat (%s); the guide is where a reader looks before the godoc.",
				c.guide, c.api, c.why)
			continue
		}
		var missing []string
		for _, cue := range c.cues {
			if !strings.Contains(text, cue) {
				missing = append(missing, cue)
			}
		}
		if len(missing) > 0 {
			t.Errorf("docs/%s documents %s but no longer says %q. The caveat: %s.\n"+
				"Restore it, or update this guard deliberately if the behavior itself changed.",
				c.guide, c.api, strings.Join(missing, ", "), c.why)
		}
	}
}
