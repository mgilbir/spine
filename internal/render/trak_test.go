package render

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/forme/style"
)

// trackedFace is a face whose AAT tracking table leaves text at 12px as it
// is and sets it 100 units per em tighter at 24px, with the STAT table
// HarfBuzz requires before it applies one.
func trackedFace(t *testing.T) *shape.Face {
	t.Helper()
	be := binary.BigEndian
	var trak []byte
	trak = be.AppendUint32(trak, 0x00010000) // version 1.0
	trak = be.AppendUint16(trak, 0)          // format
	trak = be.AppendUint16(trak, 12)         // horizontal track data
	trak = be.AppendUint16(trak, 0)          // no vertical
	trak = be.AppendUint16(trak, 0)          // reserved
	trak = be.AppendUint16(trak, 1)          // one track
	trak = be.AppendUint16(trak, 2)          // two sizes
	trak = be.AppendUint32(trak, 28)         // size table
	trak = be.AppendUint32(trak, 0)          // the normal track, 0.0
	trak = be.AppendUint16(trak, 256)        // its name
	trak = be.AppendUint16(trak, 36)         // its values
	trak = be.AppendUint32(trak, 12<<16)     // 12
	trak = be.AppendUint32(trak, 24<<16)     // 24
	trak = be.AppendUint16(trak, 0)
	trak = be.AppendUint16(trak, uint16(0xFFFF-99)) // -100
	var stat []byte
	stat = be.AppendUint16(stat, 1)
	stat = be.AppendUint16(stat, 1)
	stat = be.AppendUint16(stat, 8)
	stat = be.AppendUint16(stat, 0)
	stat = be.AppendUint32(stat, 0)
	stat = be.AppendUint16(stat, 0)
	stat = be.AppendUint32(stat, 0)
	stat = be.AppendUint16(stat, 2)
	f, err := shape.Load(fonttest.SFNT(fonttest.SFNTOptions{Glyphs: []fonttest.Glyph{{Rune: 'A', Advance: 1000, HasShape: true, Ink: [4]int{0, 0, 1000, 1000}}, {Rune: ' ', Advance: 250}}, Extra: map[string][]byte{"trak": trak, "STAT": stat}}))
	if err != nil {
		t.Fatal(err)
	}
	if f.FeaturesAt(shape.Features{}, 24).PointSize != 24 {
		t.Fatal("the fixture face is not tracked")
	}
	return f
}

func TestTextTrackedAtItsSize(t *testing.T) {
	f := trackedFace(t)
	// Four 1em letters: at 12px untracked, 48px; at 24px tracked by
	// -100/1000 em each, 4 × 0.9 × 24 = 86.4px rather than 96.
	// "AA AA" is four letters and a quarter-em space: 51px at 12px, and
	// (4 × 900 + 150)/1000 × 24 = 90px at 24px, where measuring it untracked
	// would take 102px and break it.
	for _, tc := range []struct {
		size, width, fit float64
	}{{12, 48, 51}, {24, 86.4, 90}} {
		plain, _ := NewTextLayout(Limits{})
		lines, err := plain.Lines(context.Background(), f, "AAAA", unit(tc.size), unit(500), shape.Features{}, RepertoireASCII)
		if err != nil {
			t.Fatal(err)
		}
		if got := lines[0].Width.Px(); got < tc.width-0.05 || got > tc.width+0.05 {
			t.Errorf("plain at %vpx: %v, want %v", tc.size, got, tc.width)
		}
		rich, _ := NewTextLayout(Limits{})
		rl, err := rich.RichLines(context.Background(), []Span{{Face: f, Size: unit(tc.size), Text: "AAAA"}}, unit(500), RepertoireASCII)
		if err != nil {
			t.Fatal(err)
		}
		if got := rl[0].Width.Px(); got < tc.width-0.05 || got > tc.width+0.05 {
			t.Errorf("rich at %vpx: %v, want %v", tc.size, got, tc.width)
		}
		// Measured as drawn: a width that fits the tracked line holds it
		// on one line.
		fit, _ := NewTextLayout(Limits{})
		if rl, err := fit.RichLines(context.Background(), []Span{{Face: f, Size: unit(tc.size), Text: "AA AA"}}, unit(tc.fit+0.1), RepertoireASCII); err != nil || len(rl) != 1 {
			t.Errorf("fit at %vpx: %d lines, %v", tc.size, len(rl), err)
		}
	}
	// A text operation shapes at its size too: the fourth A starts at
	// 3 × 0.9 × 24 = 64.8px.
	op := layout.DrawText{At: layout.Point{X: unit(0), Y: unit(30)}, Text: "AAAA", Face: f, Size: unit(24), Color: style.RGBA{A: 1}}
	p, err := Prepare(context.Background(), 96*9525*2, 96*9525, []layout.Op{op}, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.draws) != 4 || p.draws[3].rect.x0 < 64.7 || p.draws[3].rect.x0 > 64.9 {
		t.Fatalf("draws: %d, last at %v", len(p.draws), p.draws[len(p.draws)-1].rect.x0)
	}
}
